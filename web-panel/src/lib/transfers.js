import { api } from '../api/client';
import { downloadDeviceFileAsBlob, statRemoteFileSize } from './largeFileDownload';

const DB_NAME = 'remote_access_transfers';
const DB_VERSION = 1;
const STORE = 'requests';

function openDb() {
    return new Promise((resolve, reject) => {
        const req = indexedDB.open(DB_NAME, DB_VERSION);
        req.onerror = () => reject(req.error);
        req.onupgradeneeded = () => {
            const db = req.result;
            if (!db.objectStoreNames.contains(STORE)) {
                const store = db.createObjectStore(STORE, { keyPath: 'id' });
                store.createIndex('device_path', ['device_id', 'remote_path'], { unique: false });
                store.createIndex('status', 'status', { unique: false });
            }
        };
        req.onsuccess = () => resolve(req.result);
    });
}

async function withStore(mode, fn) {
    const db = await openDb();
    return new Promise((resolve, reject) => {
        const tx = db.transaction(STORE, mode);
        const store = tx.objectStore(STORE);
        Promise.resolve(fn(store))
            .then(resolve)
            .catch(reject);
        tx.oncomplete = () => db.close();
        tx.onerror = () => reject(tx.error);
    });
}

export function transferKey(deviceId, remotePath) {
    return `${deviceId}::${remotePath}`;
}

export async function getLocalTransfers(filters = {}) {
    return withStore('readonly', (store) => {
        return new Promise((resolve, reject) => {
            const req = store.getAll();
            req.onsuccess = () => {
                let rows = req.result || [];
                if (filters.device_id) {
                    rows = rows.filter((r) => r.device_id === filters.device_id);
                }
                if (filters.status) {
                    rows = rows.filter((r) => r.status === filters.status);
                }
                rows.sort((a, b) => new Date(b.started_at) - new Date(a.started_at));
                if (filters.limit) rows = rows.slice(0, filters.limit);
                resolve(rows);
            };
            req.onerror = () => reject(req.error);
        });
    });
}

export async function upsertLocalTransfer(row) {
    const id = row.id || transferKey(row.device_id, row.remote_path);
    const record = {
        ...row,
        id,
        updated_at: new Date().toISOString(),
    };
    await withStore('readwrite', (store) => {
        store.put(record);
    });
    return record;
}

export async function removeLocalTransfer(id) {
    await withStore('readwrite', (store) => {
        store.delete(id);
    });
}

export async function clearCompletedLocal() {
    const all = await getLocalTransfers();
    await Promise.all(
        all.filter((r) => r.status === 'completed').map((r) => removeLocalTransfer(r.id))
    );
}

export function mergeTransfers(serverRows = [], localRows = []) {
    const map = new Map();
    for (const row of localRows) {
        map.set(transferKey(row.device_id, row.remote_path), { ...row, origin: 'local' });
    }
    for (const row of serverRows) {
        const key = transferKey(row.device_id, row.remote_path);
        const local = map.get(key);
        if (
            local &&
            ['pending', 'in_progress', 'waiting'].includes(local.status) &&
            !['completed', 'error', 'cancelled'].includes(row.status)
        ) {
            map.set(key, {
                ...row,
                ...local,
                bytes_written: Math.max(local.bytes_written || 0, row.bytes_written || 0),
                origin: 'merged',
            });
        } else {
            map.set(key, { ...row, origin: 'server' });
        }
    }
    return Array.from(map.values()).sort(
        (a, b) => new Date(b.started_at || b.created_at) - new Date(a.started_at || a.created_at)
    );
}

export async function fetchServerTransfers(filters = {}) {
    try {
        const data = await api.listTransfers(filters);
        return data.transfers || data.items || [];
    } catch {
        return [];
    }
}

const deviceQueues = new Map();
const progressListeners = new Set();
/** Live in-memory progress so the Downloads page never snaps back to 0% on reload. */
const liveProgress = new Map();

export function onTransferProgress(fn) {
    progressListeners.add(fn);
    return () => progressListeners.delete(fn);
}

function emitProgress(payload) {
    if (payload?.id) {
        liveProgress.set(payload.id, {
            bytes_written: payload.bytes_written || 0,
            file_size: payload.file_size ?? null,
            status: payload.status,
            error_message: payload.error_message ?? null,
            updated_at: Date.now(),
        });
    }
    progressListeners.forEach((fn) => fn(payload));
}

export function applyLiveProgress(rows = []) {
    return rows.map((row) => {
        const live = liveProgress.get(row.id);
        if (!live) return row;
        if (row.status === 'completed' || live.status === 'completed') {
            if (live.status === 'completed') {
                return { ...row, ...live, bytes_written: Math.max(row.bytes_written || 0, live.bytes_written || 0) };
            }
            return row;
        }
        return {
            ...row,
            bytes_written: Math.max(row.bytes_written || 0, live.bytes_written || 0),
            file_size: live.file_size || row.file_size,
            status: live.status || row.status,
            error_message: live.error_message ?? row.error_message,
        };
    });
}

async function runQueued(deviceId, task) {
    if (!deviceQueues.has(deviceId)) {
        deviceQueues.set(deviceId, Promise.resolve());
    }
    const chain = deviceQueues.get(deviceId).then(task, task);
    deviceQueues.set(deviceId, chain.catch(() => {}));
    return chain;
}

export async function startDownload({
    deviceId,
    remotePath,
    fileName,
    fileSize,
    deviceOnline,
    autoResume = true,
}) {
    const id = transferKey(deviceId, remotePath);
    let row = {
        id,
        device_id: deviceId,
        remote_path: remotePath,
        file_name: fileName || remotePath.split('/').pop(),
        file_size: fileSize ?? null,
        bytes_written: 0,
        status: deviceOnline ? 'pending' : 'waiting',
        error_message: deviceOnline ? null : 'Device offline — queued for retry',
        resumed: false,
        started_at: new Date().toISOString(),
    };
    await upsertLocalTransfer(row);

    if (!deviceOnline) {
        return row;
    }

    return runQueued(deviceId, () => executeDownload(row, autoResume));
}

async function executeDownload(row, autoResume) {
    const resumeKey = `xfer_resume:${row.id}`;
    const savedOffset = autoResume ? parseInt(localStorage.getItem(resumeKey) || '0', 10) : 0;
    let fileSize = row.file_size;
    if (fileSize == null || fileSize <= 0) {
        fileSize = await statRemoteFileSize(row.device_id, row.remote_path);
    }
    row = {
        ...row,
        status: 'in_progress',
        bytes_written: savedOffset,
        file_size: fileSize ?? row.file_size,
        resumed: savedOffset > 0,
    };
    await upsertLocalTransfer(row);
    emitProgress({ ...row });

    const controller = new AbortController();
    activeControllers.set(row.id, controller);

    let lastPersist = 0;
    const persistProgress = async (partial) => {
        const now = Date.now();
        // Persist often enough for accurate resume without flooding IndexedDB.
        if (now - lastPersist < 400) return;
        lastPersist = now;
        localStorage.setItem(resumeKey, String(partial.bytes_written || 0));
        await upsertLocalTransfer(partial);
    };

    try {
        const blob = await downloadDeviceFileAsBlob(row.device_id, row.remote_path, {
            fileSize: row.file_size,
            offset: savedOffset,
            signal: controller.signal,
            onProgress: (received, total) => {
                const next = {
                    ...row,
                    bytes_written: received,
                    file_size: total != null && total > 0 ? total : row.file_size,
                    status: 'in_progress',
                };
                row = next;
                emitProgress({ ...next });
                persistProgress(next);
            },
        });

        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = row.file_name;
        a.click();
        URL.revokeObjectURL(url);

        localStorage.removeItem(resumeKey);
        row.status = 'completed';
        row.bytes_written = row.file_size != null && row.file_size > 0
            ? row.file_size
            : savedOffset + blob.size;
        row.file_size = row.file_size ?? row.bytes_written;
        row.completed_at = new Date().toISOString();
        await upsertLocalTransfer(row);
        emitProgress({ ...row });
        liveProgress.delete(row.id);
        return row;
    } catch (err) {
        if (controller.signal.aborted) {
            localStorage.setItem(resumeKey, String(row.bytes_written || 0));
            row.status = 'cancelled';
        } else {
            row.status = 'error';
            row.error_message = err.message || 'Download failed';
            localStorage.setItem(resumeKey, String(row.bytes_written || 0));
        }
        await upsertLocalTransfer(row);
        emitProgress({ ...row });
        return row;
    } finally {
        activeControllers.delete(row.id);
    }
}

const activeControllers = new Map();

export function cancelDownload(id) {
    activeControllers.get(id)?.abort();
}

export async function retryDownload(row, deviceOnline, autoResume = true) {
    if (!deviceOnline) {
        await upsertLocalTransfer({
            ...row,
            status: 'waiting',
            error_message: 'Device offline — queued for retry',
        });
        return;
    }
    return startDownload({
        deviceId: row.device_id,
        remotePath: row.remote_path,
        fileName: row.file_name,
        fileSize: row.file_size,
        deviceOnline: true,
        autoResume,
    });
}

const resumeInFlight = new Set();

/** Resume waiting/pending rows when devices come online (smooth auto-resume). */
export async function processWaitingDownloads(rows, isDeviceOnlineFn, autoResume = true) {
    const candidates = rows.filter(
        (r) =>
            ['waiting', 'pending'].includes(r.status) &&
            isDeviceOnlineFn(r.device_id) &&
            !activeControllers.has(r.id) &&
            !resumeInFlight.has(r.id)
    );
    await Promise.all(
        candidates.map(async (row) => {
            resumeInFlight.add(row.id);
            try {
                await retryDownload(row, true, autoResume);
            } finally {
                resumeInFlight.delete(row.id);
            }
        })
    );
}

export async function appealAndRetry(row, deviceOnline) {
    const serverId = row.server_transfer_id || row.transfer_id;
    if (serverId) {
        try {
            await api.appealTransfer(serverId);
        } catch {
            /* optional server API */
        }
    }
    await upsertLocalTransfer({
        ...row,
        status: 'pending',
        error_message: null,
        appealed: true,
    });
    return retryDownload(row, deviceOnline, true);
}
