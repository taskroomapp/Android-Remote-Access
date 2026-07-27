export function formatSessionClock(ms) {
    const s = Math.floor(ms / 1000);
    const h = Math.floor(s / 3600);
    const m = Math.floor((s % 3600) / 60);
    const r = s % 60;
    return `${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(r).padStart(2, '0')}`;
}

export function formatShortTime(iso) {
    try {
        return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    } catch {
        return '';
    }
}

export function formatBytes(n) {
    if (!n || n < 1024) return `${n || 0} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

export function formatStorage(value) {
    if (value == null || value === '—') return '—';
    const n = Number(value);
    if (!Number.isFinite(n)) return String(value);
    if (n > 1024 * 1024) return `${(n / (1024 * 1024 * 1024)).toFixed(1)} GB`;
    return String(value);
}
