import { api } from '../api/client';
import { runCommand, parseCommandData } from './commandRunner';

/** Chunk size aligned with agent/server limits for reliable CKX1 frames. */
const CHUNK_BYTES = 96 * 1024;
const CONCURRENCY = 4;
const LARGE_FILE_BYTES = 256 * 1024;

function decodeBase64ToUint8Array(base64) {
    const bin = atob(String(base64).replace(/\s/g, ''));
    const out = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i += 1) out[i] = bin.charCodeAt(i);
    return out;
}

async function readFileChunk(deviceId, path, offset, size, signal) {
    if (signal?.aborted) {
        throw new DOMException('Aborted', 'AbortError');
    }
    const status = await runCommand(deviceId, 'file_read_chunk', { path, offset, size }, 120);
    if (status.status !== 'success') {
        throw new Error(status.error || status.message || 'Chunk read failed');
    }
    const data = parseCommandData(status.data);
    if (!data || typeof data !== 'object') {
        throw new Error('Invalid chunk response');
    }
    const bytesRead = data.bytes_read ?? 0;
    const content = data.content ? decodeBase64ToUint8Array(data.content) : new Uint8Array(0);
    return {
        bytes: content,
        bytesRead,
        fileSize: data.file_size != null ? Number(data.file_size) : null,
        offset: data.offset != null ? Number(data.offset) : offset,
    };
}

function pendingBytes(pending) {
    let n = 0;
    for (const bytes of pending.values()) n += bytes?.length || 0;
    return n;
}

/**
 * Parallel windowed chunk download with ordered Blob assembly.
 * Progress is monotonic and updates as soon as each chunk arrives (plus contiguous assembly).
 */
async function downloadViaParallelChunks(deviceId, remotePath, { offset = 0, fileSize = null, signal, onProgress } = {}) {
    onProgress?.(offset, fileSize > 0 ? fileSize : null);

    const first = await readFileChunk(deviceId, remotePath, offset, CHUNK_BYTES, signal);
    let total = first.fileSize != null && first.fileSize > 0 ? first.fileSize : fileSize;
    if (first.bytesRead <= 0 || first.bytes.length === 0) {
        onProgress?.(offset, total ?? 0);
        return new Blob([], { type: 'application/octet-stream' });
    }

    const parts = [first.bytes];
    let assembled = offset + first.bytes.length;
    const report = (pendingMap = null) => {
        const extra = pendingMap ? pendingBytes(pendingMap) : 0;
        const received = Math.min(
            total != null && total > 0 ? total : assembled + extra,
            assembled + extra
        );
        onProgress?.(received, total ?? received);
    };
    report();

    if (total != null && assembled >= total) {
        return new Blob(parts, { type: 'application/octet-stream' });
    }
    if ((total == null || total <= 0) && first.bytesRead < CHUNK_BYTES) {
        return new Blob(parts, { type: 'application/octet-stream' });
    }

    // Unknown size: sequential reads with progress each chunk.
    if (total == null || total <= 0) {
        let pos = assembled;
        while (true) {
            if (signal?.aborted) throw new DOMException('Aborted', 'AbortError');
            const chunk = await readFileChunk(deviceId, remotePath, pos, CHUNK_BYTES, signal);
            if (chunk.fileSize != null && chunk.fileSize > 0) total = chunk.fileSize;
            if (chunk.bytesRead <= 0 || chunk.bytes.length === 0) break;
            parts.push(chunk.bytes);
            pos += chunk.bytes.length;
            assembled = pos;
            report();
            if (total != null && assembled >= total) break;
            if (chunk.bytesRead < CHUNK_BYTES) break;
        }
        return new Blob(parts, { type: 'application/octet-stream' });
    }

    // Known size: sliding window — keep CONCURRENCY fetches in flight, assemble in order.
    const pending = new Map();
    let nextFetch = assembled;
    let inFlight = 0;
    let rejectWait = null;
    let resolveWait = null;
    let notified = false;

    const wake = () => {
        notified = true;
        if (resolveWait) {
            const r = resolveWait;
            resolveWait = null;
            rejectWait = null;
            notified = false;
            r();
        }
    };

    const waitForProgress = () => {
        if (notified) {
            notified = false;
            return Promise.resolve();
        }
        return new Promise((resolve, reject) => {
            resolveWait = resolve;
            rejectWait = reject;
        });
    };
    const launch = () => {
        while (inFlight < CONCURRENCY && nextFetch < total) {
            if (signal?.aborted) {
                rejectWait?.(new DOMException('Aborted', 'AbortError'));
                return;
            }
            const off = nextFetch;
            const size = Math.min(CHUNK_BYTES, total - off);
            nextFetch += size;
            inFlight += 1;
            readFileChunk(deviceId, remotePath, off, size, signal)
                .then((chunk) => {
                    inFlight -= 1;
                    if (chunk.fileSize != null && chunk.fileSize > 0) {
                        total = chunk.fileSize;
                    }
                    if (!chunk.bytes?.length) {
                        rejectWait?.(new Error(`Empty chunk at offset ${off}`));
                        wake();
                        return;
                    }
                    pending.set(off, chunk.bytes);
                    report(pending);
                    wake();
                })
                .catch((err) => {
                    inFlight -= 1;
                    rejectWait?.(err);
                    wake();
                });
        }
    };

    launch();

    while (assembled < total) {
        if (signal?.aborted) {
            throw new DOMException('Aborted', 'AbortError');
        }

        let drained = false;
        while (pending.has(assembled)) {
            const bytes = pending.get(assembled);
            pending.delete(assembled);
            parts.push(bytes);
            assembled += bytes.length;
            drained = true;
            report(pending);
            if (assembled >= total) {
                return new Blob(parts, { type: 'application/octet-stream' });
            }
        }

        launch();

        if (assembled >= total) break;
        if (!drained && inFlight === 0 && !pending.has(assembled)) {
            throw new Error(`Download stalled at offset ${assembled}`);
        }
        if (!pending.has(assembled)) {
            await waitForProgress();
        }
    }

    return new Blob(parts, { type: 'application/octet-stream' });
}

/**
 * Download a device file as Blob with accurate incremental progress.
 * Large known-size files use parallel chunk pulls so the UI updates continuously
 * (HTTP /files/stream is often buffered by proxies and sits at 0% until done).
 */
export async function downloadDeviceFileAsBlob(
    deviceId,
    remotePath,
    { fileSize = null, offset = 0, signal, onProgress } = {}
) {
    let knownSize = fileSize != null && fileSize > 0 ? Number(fileSize) : null;
    if (knownSize == null) {
        knownSize = await statRemoteFileSize(deviceId, remotePath);
    }
    onProgress?.(offset, knownSize);

    const preferChunks =
        knownSize != null &&
        knownSize >= LARGE_FILE_BYTES;

    if (preferChunks) {
        try {
            return await downloadViaParallelChunks(deviceId, remotePath, {
                offset,
                fileSize: knownSize,
                signal,
                onProgress,
            });
        } catch (err) {
            if (err?.name === 'AbortError') throw err;
            /* fall through to stream */
        }
    }

    try {
        return await api.downloadFileStream(deviceId, remotePath, {
            offset,
            fileSize: knownSize,
            signal,
            onProgress,
        });
    } catch (err) {
        if (err?.name === 'AbortError') throw err;
    }

    return downloadViaParallelChunks(deviceId, remotePath, {
        offset,
        fileSize: knownSize,
        signal,
        onProgress,
    });
}

export async function statRemoteFileSize(deviceId, remotePath) {
    try {
        const chunk = await readFileChunk(deviceId, remotePath, 0, 1);
        return chunk.fileSize != null && chunk.fileSize > 0 ? chunk.fileSize : null;
    } catch {
        return null;
    }
}
