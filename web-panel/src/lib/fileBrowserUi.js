import { normalizePath } from './paths';

export const FILE_BOOKMARKS = [
    { label: 'DCIM / Camera', path: '/storage/emulated/0/DCIM', icon: 'camera', colorClass: 'fb-folder-camera' },
    { label: 'Downloads', path: '/storage/emulated/0/Download', icon: 'download', colorClass: 'fb-folder-download' },
    { label: 'Music', path: '/storage/emulated/0/Music', icon: 'music', colorClass: 'fb-folder-music' },
    { label: 'Movies', path: '/storage/emulated/0/Movies', icon: 'film', colorClass: 'fb-folder-movies' },
    { label: 'Pictures', path: '/storage/emulated/0/Pictures', icon: 'image', colorClass: 'fb-folder-pictures' },
    { label: 'Documents', path: '/storage/emulated/0/Documents', icon: 'fileText', colorClass: 'fb-folder-docs' },
    { label: 'Android', path: '/storage/emulated/0/Android', icon: 'smartphone', colorClass: 'fb-folder-android' },
];

export function formatFileSize(bytes) {
    if (bytes == null) return '—';
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

export function formatFileModified(entry) {
    const t = entry?.modified_time || entry?.modified || entry?.mtime;
    if (!t) return '—';
    try {
        return new Date(t).toLocaleString();
    } catch {
        return String(t);
    }
}

export function getFileTypeLabel(entry) {
    if (entry?.is_directory) return 'Folder';
    const name = String(entry?.name || '');
    const ext = name.includes('.') ? name.split('.').pop().toLowerCase() : '';
    if (!ext) return 'File';
    return ext.toUpperCase();
}

export function sortAndFilterFiles(entries, { query, sortField, sortAsc }) {
    let rows = [...(entries || [])];
    const q = String(query || '').trim().toLowerCase();
    if (q) {
        rows = rows.filter(
            (r) =>
                r.name?.toLowerCase().includes(q) ||
                r.path?.toLowerCase().includes(q)
        );
    }
    rows.sort((a, b) => {
        let cmp = 0;
        if (sortField === 'size') {
            cmp = (a.size || 0) - (b.size || 0);
        } else if (sortField === 'modified') {
            cmp = String(a.modified_time || '').localeCompare(String(b.modified_time || ''));
        } else {
            cmp = String(a.name || '').localeCompare(String(b.name || ''), undefined, { sensitivity: 'base' });
        }
        return sortAsc ? cmp : -cmp;
    });
    return rows;
}

/** Hybrid cycle: asc → desc → next field (name → size → modified). */
export function cycleFileSort(currentField, sortAsc) {
    const order = ['name', 'size', 'modified'];
    const idx = order.indexOf(currentField);
    if (idx < 0) return { sortField: 'name', sortAsc: true };
    if (sortAsc) return { sortField: currentField, sortAsc: false };
    return { sortField: order[(idx + 1) % order.length], sortAsc: true };
}

export function sortFieldLabel(field) {
    return field.charAt(0).toUpperCase() + field.slice(1);
}

export function pathKey(path) {
    return normalizePath(path).toLowerCase();
}
