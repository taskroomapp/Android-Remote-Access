import { ApiError } from './errors.js';

export function payloadToBlob(data) {
    if (data == null) return new Blob([]);
    if (data instanceof Blob) return data;
    if (typeof data === 'object' && data.content) {
        return base64ToBlob(data.content, data.mime_type);
    }
    if (typeof data === 'string') {
        if (/^[A-Za-z0-9+/=\s]+$/.test(data.slice(0, 64)) && data.length > 64) {
            try {
                return base64ToBlob(data);
            } catch {
                /* fall through */
            }
        }
        return new Blob([data]);
    }
    return new Blob([JSON.stringify(data)]);
}

export function base64ToBlob(b64, mime = 'application/octet-stream') {
    const cleaned = b64.replace(/\s/g, '');
    const binary = atob(cleaned);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
    return new Blob([bytes], { type: mime });
}

export function attachFilesApi(proto, { API_BASE_URL }) {
    proto.normalizePath = function normalizePath(path) {
        return (path || '/')
            .replace('/storage/emulated/legacy', '/storage/emulated/0')
            .replace(/^\/storage\/emulated$/, '/storage/emulated/0');
    };

    proto.storageRoots = async function storageRoots(deviceId) {
        return this.fileList(deviceId, '/storage/emulated/0');
    };

    proto.fileList = async function fileList(deviceId, path) {
        return this.executeCommand(deviceId, 'file_list', { path: this.normalizePath(path) });
    };

    proto.fileRead = async function fileRead(deviceId, path) {
        return this.executeCommand(deviceId, 'file_read', { path: this.normalizePath(path) });
    };

    proto.fileStat = async function fileStat(deviceId, path) {
        return this.fileList(deviceId, path);
    };

    proto.listFiles = async function listFiles(deviceId, path = '/') {
        return this.request(`/files/list?device_id=${deviceId}&path=${encodeURIComponent(path)}`);
    };

    proto.readFile = async function readFile(deviceId, path) {
        return this.request(`/files/read?device_id=${deviceId}&path=${encodeURIComponent(path)}`);
    };

    proto.deleteFile = async function deleteFile(deviceId, path) {
        return this.request(`/files/delete?device_id=${deviceId}&path=${encodeURIComponent(path)}`, {
            method: 'DELETE',
        });
    };

    proto.downloadFileStream = async function downloadFileStream(deviceId, remotePath, { offset = 0, fileSize = null, signal, onProgress } = {}) {
        const normPath = this.normalizePath(remotePath);
        const q = new URLSearchParams({
            device_id: deviceId,
            path: normPath,
        });
        const url = `${API_BASE_URL}/files/stream?${q.toString()}`;
        await this.ensureCkx1();
        const headers = {
            Authorization: `Bearer ${this.accessToken}`,
            'X-CKX1-Session': this.ckx1.protocolSessionId,
            Accept: 'application/octet-stream',
        };
        if (offset > 0) {
            headers.Range = `bytes=${offset}-`;
        }

        // Announce known size immediately so the UI is not stuck at "0 B / —".
        if (fileSize != null && fileSize > 0) {
            onProgress?.(offset, fileSize);
        }

        let response;
        try {
            response = await fetch(url, { headers, signal, cache: 'no-store' });
        } catch (err) {
            if (err?.name === 'AbortError') throw err;
            return this.downloadFileStreamLegacy(deviceId, normPath, { offset, signal, onProgress });
        }

        if (response.status === 404 || response.status === 502 || response.status === 503) {
            return this.downloadFileStreamLegacy(deviceId, normPath, { offset, signal, onProgress });
        }

        if (!response.ok) {
            const error = await response.json().catch(() => ({}));
            throw new ApiError(error.message || 'Download failed', response.status, error.code);
        }

        const totalHeader = response.headers.get('Content-Length');
        const contentRange = response.headers.get('Content-Range');
        let total = fileSize > 0 ? fileSize : null;
        if (contentRange) {
            const m = /\/(\d+)\s*$/.exec(contentRange);
            if (m) total = parseInt(m[1], 10);
        } else if (totalHeader) {
            total = offset + parseInt(totalHeader, 10);
        }
        onProgress?.(offset, total ?? fileSize ?? null);

        const reader = response.body?.getReader();
        if (!reader) {
            return this.downloadFileStreamLegacy(deviceId, normPath, { offset, signal, onProgress });
        }

        const parts = [];
        let received = offset;
        let lastEmit = 0;
        while (true) {
            const { done, value } = await reader.read();
            if (done) break;
            if (value?.length) {
                parts.push(value);
                received += value.length;
                // Emit often enough for a smooth progress bar (every ~32 KiB or always near end).
                if (received - lastEmit >= 32 * 1024 || (total != null && received >= total)) {
                    lastEmit = received;
                    onProgress?.(received, total ?? received);
                }
            }
            if (signal?.aborted) {
                reader.cancel();
                throw new DOMException('Aborted', 'AbortError');
            }
        }
        onProgress?.(received, total ?? received);
        return new Blob(parts, { type: 'application/octet-stream' });
    };

    proto.downloadFileStreamLegacy = async function downloadFileStreamLegacy(deviceId, remotePath, { offset = 0, signal, onProgress } = {}) {
        const json = await this.request(
            `/files/read?device_id=${encodeURIComponent(deviceId)}&path=${encodeURIComponent(remotePath)}`,
            { signal },
        );
        if (json.status && json.status !== 'success') {
            throw new ApiError(json.error || 'File read failed', 400, 'FILE_READ_FAILED');
        }
        const blob = payloadToBlob(json.data);
        if (offset > 0 && blob.size > offset) {
            const sliced = blob.slice(offset);
            onProgress?.(blob.size, blob.size);
            return sliced;
        }
        onProgress?.(blob.size, blob.size);
        return blob;
    };
}
