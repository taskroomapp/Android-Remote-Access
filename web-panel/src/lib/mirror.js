import { api } from '../api/client';
import { listFilesLive } from './commandRunner';
import { joinPath, normalizePath, parentPath, pathKey, rewriteEmulatedPath } from './paths';

const LOCAL_PREFIX = 'mirror:';
const PREFS_PREFIX = 'fb_prefs:';
const STALE_MS = 6 * 60 * 60 * 1000;
const SYNC_MAX_DEPTH = 12;
const SYNC_MAX_ENTRIES = 25000;

export function getBrowsePrefs(deviceId) {
    try {
        return JSON.parse(localStorage.getItem(`${PREFS_PREFIX}${deviceId}`) || '{}');
    } catch {
        return {};
    }
}

export function saveBrowsePrefs(deviceId, prefs) {
    localStorage.setItem(`${PREFS_PREFIX}${deviceId}`, JSON.stringify(prefs));
}

export function loadLocalMirror(deviceId, type) {
    try {
        const raw = localStorage.getItem(`${LOCAL_PREFIX}${deviceId}:${type}`);
        return raw ? JSON.parse(raw) : null;
    } catch {
        return null;
    }
}

export function saveLocalMirror(deviceId, type, snapshot) {
    localStorage.setItem(`${LOCAL_PREFIX}${deviceId}:${type}`, JSON.stringify(snapshot));
}

export async function fetchServerMirror(deviceId, type) {
    try {
        return await api.getMirrorSnapshot(deviceId, type);
    } catch {
        return null;
    }
}

export function pickNewerSnapshot(localSnap, serverSnap) {
    if (!localSnap && !serverSnap) return null;
    if (!localSnap) return { ...serverSnap, source: 'server' };
    if (!serverSnap) return { ...localSnap, source: 'local' };
    const lt = new Date(localSnap.updated_at || 0).getTime();
    const st = new Date(serverSnap.updated_at || 0).getTime();
    if (st > lt) return { ...serverSnap, source: 'server' };
    return { ...localSnap, source: 'local' };
}

export function isMirrorStale(snapshot) {
    if (!snapshot?.updated_at) return true;
    return Date.now() - new Date(snapshot.updated_at).getTime() > STALE_MS;
}

export function mirrorChildren(snapshot, currentPath) {
    if (!snapshot?.entries) return [];
    const key = pathKey(currentPath);
    return snapshot.entries.filter((e) => pathKey(e.parent_path) === key);
}

export function mirrorRoots(snapshot) {
    if (snapshot?.roots?.length) return snapshot.roots;
    if (!snapshot?.entries?.length) return [];
    const roots = snapshot.entries.filter((e) => !e.parent_path || e.parent_path === '');
    return roots;
}

export async function buildFileTreeFromDevice(deviceId, onProgress) {
    const entries = [];
    const roots = [];
    const visited = new Set();
    let count = 0;

    const rootRes = await listFilesLive(deviceId, '/storage/emulated/0');
    const rootList = rootRes.length ? rootRes : await listFilesLive(deviceId, '/');

    for (const item of rootList) {
        if (count >= SYNC_MAX_ENTRIES) break;
        const path = rewriteEmulatedPath(item.path);
        roots.push({ ...item, path, parent_path: '' });
        entries.push({ ...item, path, parent_path: '' });
        count++;
        if (item.is_directory) {
            await walk(path, '', 1);
        }
    }

    async function walk(dirPath, parent, depth) {
        if (depth > SYNC_MAX_DEPTH || count >= SYNC_MAX_ENTRIES) return;
        const pk = pathKey(dirPath);
        if (visited.has(pk)) return;
        visited.add(pk);

        onProgress?.(`Scanning ${dirPath}…`);
        let children = [];
        try {
            children = await listFilesLive(deviceId, dirPath);
        } catch {
            return;
        }

        for (const child of children) {
            if (count >= SYNC_MAX_ENTRIES) break;
            const path = rewriteEmulatedPath(child.path || joinPath(dirPath, child.name));
            const row = {
                name: child.name,
                path,
                parent_path: normalizePath(dirPath),
                is_directory: child.is_directory,
                size: child.size ?? 0,
                modified_time: child.modified_time,
            };
            entries.push(row);
            count++;
            if (child.is_directory) {
                await walk(path, dirPath, depth + 1);
            }
        }
    }

    return {
        type: 'file_tree',
        updated_at: new Date().toISOString(),
        source: 'device',
        roots,
        entries,
        entry_count: entries.length,
    };
}

export async function syncFileTreeMirror(deviceId, { silent = false, onProgress } = {}) {
    if (!silent) onProgress?.('Fetching tree and saving…');
    let snapshot = await buildFileTreeFromDevice(deviceId, onProgress);
    try {
        const serverResult = await api.mirrorUpdate(deviceId, {
            types: ['file_tree'],
            snapshots: { file_tree: snapshot },
        });
        if (serverResult?.file_tree) {
            snapshot = { ...serverResult.file_tree, source: 'device' };
        }
    } catch {
        // server mirror API optional
    }
    try {
        const files = Array.isArray(snapshot.entries) ? snapshot.entries : [];
        if (files.length) {
            await api.saveDeviceArtifacts(deviceId, { files });
        }
    } catch {
        // DB persistence optional
    }
    saveLocalMirror(deviceId, 'file_tree', snapshot);
    return snapshot;
}

export function emptyTreeSnapshot() {
    return {
        type: 'file_tree',
        updated_at: null,
        source: 'none',
        roots: [],
        entries: [],
        entry_count: 0,
    };
}
