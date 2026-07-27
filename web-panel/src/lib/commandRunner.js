import { api } from '../api/client';

export async function pollCommandResult(transactionId, { maxAttempts = 60, intervalMs = 1000 } = {}) {
    for (let i = 0; i < maxAttempts; i++) {
        try {
            const status = await api.getCommandStatus(transactionId);
            if (['success', 'failed', 'timeout', 'cancelled'].includes(status.status)) {
                return status;
            }
            if (status.status === 'pending') {
                await sleep(intervalMs);
                continue;
            }
        } catch (err) {
            if (err?.status !== 404) {
                throw err;
            }
        }
        await sleep(intervalMs);
    }
    return { status: 'timeout', error: 'Command timed out', data: null };
}

export async function runCommand(deviceId, commandType, payload = {}, timeoutSeconds = 60) {
    const response = await api.executeCommand(deviceId, commandType, payload, timeoutSeconds);
    const terminal = ['success', 'failed', 'timeout', 'cancelled'];
    if (terminal.includes(response.status)) {
        return response;
    }
    if (response.status === 'queued' || response.queued) {
        return pollCommandResult(response.transaction_id);
    }
    if (response.transaction_id) {
        return pollCommandResult(response.transaction_id);
    }
    return response;
}

export function parseCommandData(raw) {
    if (raw == null) return null;
    if (typeof raw === 'object') return raw;
    if (typeof raw === 'string') {
        try {
            return JSON.parse(raw);
        } catch {
            try {
                const decoded = atob(raw);
                try {
                    return JSON.parse(decoded);
                } catch {
                    return decoded;
                }
            } catch {
                return raw;
            }
        }
    }
    return raw;
}

export async function listFilesLive(deviceId, path) {
    const res = await api.listFiles(deviceId, path);
    const data = parseCommandData(res.data);
    return normalizeFileList(data, path);
}

export async function readFileLive(deviceId, path) {
    const res = await api.readFile(deviceId, path);
    return parseCommandData(res.data);
}

function sleep(ms) {
    return new Promise((r) => setTimeout(r, ms));
}

function normalizeFileList(data, parentPath) {
    let items = [];
    if (Array.isArray(data)) {
        items = data;
    } else if (data?.files) {
        items = data.files;
    } else if (data?.entries) {
        items = data.entries;
    } else if (data?.items) {
        items = data.items;
    }

    return items.map((f) => ({
        name: f.name,
        path: f.path || joinPath(parentPath, f.name),
        is_directory: f.is_directory ?? f.isDirectory ?? false,
        size: f.size ?? 0,
        modified_time: f.modified_time || f.modifiedTime || null,
    }));
}

function joinPath(base, name) {
    if (!base || base === '/') return `/${name}`.replace('//', '/');
    return `${base.replace(/\/$/, '')}/${name}`;
}
