import { useCallback, useEffect, useRef, useState } from 'react';

const LAYOUTS = {
    fb: {
        cols: {
            sidebar: { cssVar: '--rp-sidebar-w', min: 160, max: 480, default: 224 },
            details: { cssVar: '--rp-details-w', min: 200, max: 560, default: 256 },
        },
    },
    cm: {
        cols: {
            sidebar: { cssVar: '--rp-sidebar-w', min: 180, max: 420, default: 240 },
            details: { cssVar: '--rp-details-w', min: 220, max: 560, default: 320 },
        },
    },
    lv: {
        cols: {
            sidebar: { cssVar: '--rp-sidebar-w', min: 140, max: 320, default: 240 },
            details: { cssVar: '--rp-details-w', min: 180, max: 400, default: 272 },
        },
    },
};

function storageKey(id) {
    return `remote_panel_layout:${id}`;
}

function clamp(n, min, max) {
    return Math.min(max, Math.max(min, Math.round(n)));
}

function loadWidths(id) {
    const cfg = LAYOUTS[id];
    if (!cfg) return null;
    let stored = null;
    try {
        stored = JSON.parse(localStorage.getItem(storageKey(id)) || 'null');
    } catch {
        stored = null;
    }
    const out = {};
    for (const [name, col] of Object.entries(cfg.cols)) {
        const saved = stored?.[name];
        out[name] = clamp(Number.isFinite(saved) ? saved : col.default, col.min, col.max);
    }
    return out;
}

export function usePanelLayout(layoutId) {
    const shellRef = useRef(null);
    const [widths, setWidths] = useState(() => loadWidths(layoutId) || {});

    useEffect(() => {
        const shell = shellRef.current;
        const cfg = LAYOUTS[layoutId];
        if (!shell || !cfg) return;
        for (const [name, col] of Object.entries(cfg.cols)) {
            const w = widths[name] ?? col.default;
            shell.style.setProperty(col.cssVar, `${w}px`);
        }
    }, [layoutId, widths]);

    const setColumnWidth = useCallback(
        (name, width, persist = true) => {
            const col = LAYOUTS[layoutId]?.cols[name];
            if (!col) return;
            const w = clamp(width, col.min, col.max);
            setWidths((prev) => {
                const next = { ...prev, [name]: w };
                if (persist) {
                    try {
                        localStorage.setItem(storageKey(layoutId), JSON.stringify(next));
                    } catch {
                        /* ignore */
                    }
                }
                return next;
            });
        },
        [layoutId]
    );

    const resetColumn = useCallback(
        (name) => {
            const col = LAYOUTS[layoutId]?.cols[name];
            if (col) setColumnWidth(name, col.default);
        },
        [layoutId, setColumnWidth]
    );

    const bindSplitter = useCallback(
        (target) => ({
            role: 'separator',
            'aria-orientation': 'vertical',
            tabIndex: 0,
            'data-rp-target': target,
            className: 'rp-splitter',
            title: 'Drag to resize · double-click to reset',
            onPointerDown: (ev) => {
                if (ev.button !== 0 || window.matchMedia('(max-width: 1100px)').matches) return;
                ev.preventDefault();
                const startX = ev.clientX;
                const startW = widths[target] ?? LAYOUTS[layoutId].cols[target].default;
                document.body.classList.add('rp-dragging');
                const onMove = (moveEv) => {
                    const delta = moveEv.clientX - startX;
                    const next = target === 'details' ? startW - delta : startW + delta;
                    setColumnWidth(target, next, false);
                };
                const finish = () => {
                    document.body.classList.remove('rp-dragging');
                    document.removeEventListener('pointermove', onMove);
                    document.removeEventListener('pointerup', finish);
                    document.removeEventListener('pointercancel', finish);
                    setWidths((prev) => {
                        try {
                            localStorage.setItem(storageKey(layoutId), JSON.stringify(prev));
                        } catch {
                            /* ignore */
                        }
                        return prev;
                    });
                };
                document.addEventListener('pointermove', onMove);
                document.addEventListener('pointerup', finish);
                document.addEventListener('pointercancel', finish);
            },
            onDoubleClick: (ev) => {
                ev.preventDefault();
                resetColumn(target);
            },
        }),
        [layoutId, resetColumn, setColumnWidth, widths]
    );

    return { shellRef, bindSplitter, widths };
}
