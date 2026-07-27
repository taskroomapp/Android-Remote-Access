/** Normalize Android-style paths for display and mirror keys. */
export function normalizePath(path) {
    if (path == null || path === '') return '';
    let p = String(path).replace(/\\/g, '/');
    if (p.length > 1 && p.endsWith('/')) p = p.slice(0, -1);
    return p;
}

export function pathKey(path) {
    return normalizePath(path).toLowerCase();
}

export function parentPath(path) {
    const p = normalizePath(path);
    if (!p || p === '/') return '';
    const idx = p.lastIndexOf('/');
    if (idx <= 0) return '';
    return p.slice(0, idx);
}

export function joinPath(base, name) {
    const b = normalizePath(base);
    const n = String(name || '').replace(/^\//, '');
    if (!b) return normalizePath(n);
    return normalizePath(`${b}/${n}`);
}

/** Emulated storage path rewrite per spec (e.g. .../0 suffix). */
export function rewriteEmulatedPath(path) {
    const p = normalizePath(path);
    if (/^\/storage\/emulated$/i.test(p)) {
        return '/storage/emulated/0';
    }
    return p;
}

export function breadcrumbSegments(path) {
    const p = rewriteEmulatedPath(path);
    if (!p) return [];
    const parts = p.split('/').filter(Boolean);
    const segments = [];
    let acc = '';
    for (const part of parts) {
        acc = acc ? `${acc}/${part}` : `/${part}`;
        segments.push({ label: part, path: acc });
    }
    return segments;
}
