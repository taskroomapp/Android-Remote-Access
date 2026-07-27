import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Link, useSearchParams, useLocation } from 'react-router-dom';
import DevicePicker from '../components/DevicePicker';
import Icon from '../components/ui/Icon';
import {
    FileBrowserBanners,
    FileDetailsPanel,
    FileGridView,
    FileListTable,
    FileSelectionBar,
} from '../components/hybrid/FileEntryViews';
import TruncatedText from '../components/hybrid/TruncatedText';
import FileTree from '../components/files/FileTree';
import { useDevices, isDeviceOnline } from '../hooks/useDevices';
import { listFilesLive, readFileLive } from '../lib/commandRunner';
import { toImageDataUrl } from '../lib/media';
import { downloadBlob } from '../lib/download';
import {
    FILE_BOOKMARKS,
    cycleFileSort,
    pathKey,
    sortAndFilterFiles,
    sortFieldLabel,
} from '../lib/fileBrowserUi';
import { usePanelLayout } from '../hooks/usePanelLayout';
import {
    emptyTreeSnapshot,
    fetchServerMirror,
    getBrowsePrefs,
    isMirrorStale,
    loadLocalMirror,
    mirrorChildren,
    mirrorRoots,
    pickNewerSnapshot,
    saveBrowsePrefs,
    saveLocalMirror,
    syncFileTreeMirror,
} from '../lib/mirror';
import { startDownload } from '../lib/transfers';
import { breadcrumbSegments, normalizePath, rewriteEmulatedPath } from '../lib/paths';
import { api } from '../api/client';

const MODES = { live: 'live', mirror: 'mirror' };

export default function FilesPage() {
    const [searchParams, setSearchParams] = useSearchParams();
    const routerLocation = useLocation();
    const { devices, loading: devicesLoading, reload } = useDevices({ storageAccessOnly: true });
    const [deviceId, setDeviceId] = useState(searchParams.get('device') || '');
    const [mode, setMode] = useState(MODES.live);
    const [mirror, setMirror] = useState(emptyTreeSnapshot());
    const [currentPath, setCurrentPath] = useState('');
    const [entries, setEntries] = useState([]);
    const [sidebarTree, setSidebarTree] = useState({});
    const [statusText, setStatusText] = useState('');
    const [sourceBadge, setSourceBadge] = useState('local');
    const [bootstrapLoading, setBootstrapLoading] = useState(false);
    const [syncLoading, setSyncLoading] = useState(false);
    const [listLoading, setListLoading] = useState(false);
    const [selected, setSelected] = useState(null);
    const [selectedPaths, setSelectedPaths] = useState(new Set());
    const [view, setView] = useState('list');
    const [sortField, setSortField] = useState('name');
    const [sortAsc, setSortAsc] = useState(true);
    const [query, setQuery] = useState('');
    const [preview, setPreview] = useState(null);
    const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
    const [detailsOpen, setDetailsOpen] = useState(true);
    const [expanded, setExpanded] = useState(new Set(['']));

    const { shellRef, bindSplitter } = usePanelLayout('fb');

    const device = devices.find((d) => d.id === deviceId);
    const online = isDeviceOnline(device);

    const mirrorRef = useRef(mirror);
    const sidebarTreeRef = useRef(sidebarTree);
    const entriesRef = useRef(entries);
    const onlineRef = useRef(online);
    const loadGenRef = useRef(0);
    mirrorRef.current = mirror;
    sidebarTreeRef.current = sidebarTree;
    entriesRef.current = entries;
    onlineRef.current = online;

    const bootstrap = useCallback(async (id) => {
        if (!id) return;
        setBootstrapLoading(true);
        setStatusText('Loading saved folder structure…');
        try {
            const prefs = getBrowsePrefs(id);
            const localSnap = loadLocalMirror(id, 'file_tree');
            const serverSnap = await fetchServerMirror(id, 'file_tree');
            const chosen = pickNewerSnapshot(localSnap, serverSnap);
            if (chosen && serverSnap && chosen.source === 'server') {
                saveLocalMirror(id, 'file_tree', serverSnap);
            }

            const deviceOnline = onlineRef.current;

            if (chosen?.entries?.length) {
                setMirror(chosen);
                setSourceBadge(chosen.source || 'local');
                setStatusText(`Cached folder tree · ${chosen.entry_count || chosen.entries.length} entries`);
            } else if (!deviceOnline) {
                setMirror(emptyTreeSnapshot());
                setStatusText('No cached tree — device offline');
            } else {
                setStatusText('Direct browsing — listings refresh from the device');
            }

            // Prefer live when online so newly created files appear immediately.
            if (deviceOnline) {
                if (prefs.mode === MODES.mirror && chosen?.entries?.length) {
                    setMode(MODES.mirror);
                } else {
                    setMode(MODES.live);
                }
                if (!chosen || isMirrorStale(chosen)) {
                    syncFileTreeMirror(id, { silent: true }).then((snap) => {
                        if (snap?.entries?.length) {
                            setMirror(snap);
                            setSourceBadge('device');
                        }
                    }).catch(() => {});
                }
            } else if (chosen?.entries?.length) {
                setMode(MODES.mirror);
            } else {
                setMode(MODES.live);
            }
            if (prefs.last_path) setCurrentPath(prefs.last_path);
        } finally {
            setBootstrapLoading(false);
        }
    }, []);

    useEffect(() => {
        if (deviceId) {
            bootstrap(deviceId);
            setSearchParams({ device: deviceId });
        }
    }, [deviceId, bootstrap, setSearchParams]);

    useEffect(() => {
        const deepPath = routerLocation.state?.path;
        if (deviceId && deepPath != null && deepPath !== '') {
            setCurrentPath(normalizePath(deepPath));
            setMode(MODES.live);
        }
    }, [deviceId, routerLocation.state?.path]);

    useEffect(() => {
        setSelected(null);
        setPreview(null);
        setSelectedPaths(new Set());
        setEntries([]);
    }, [deviceId]);

    useEffect(() => {
        if (!currentPath) return;
        const parts = breadcrumbSegments(currentPath);
        setExpanded((prev) => {
            const next = new Set(prev);
            next.add('');
            parts.forEach((p) => next.add(normalizePath(p.path)));
            return next;
        });
    }, [currentPath]);

    const loadFolder = useCallback(async ({ silent = false } = {}) => {
        if (!deviceId) return;
        const gen = ++loadGenRef.current;
        if (!silent) {
            setListLoading(true);
        }
        try {
            if (mode === MODES.mirror) {
                const snap = mirrorRef.current;
                const path = normalizePath(currentPath);
                const rows = path ? mirrorChildren(snap, path) : mirrorRoots(snap);
                if (gen !== loadGenRef.current) return;
                setEntries(rows);
                setSourceBadge(snap.source || 'local');
                return;
            }

            if (!silent) {
                setStatusText('Direct browsing');
                setSourceBadge('device');
            }
            const path = rewriteEmulatedPath(currentPath || '/storage/emulated/0');
            let rows = [];
            try {
                rows = await listFilesLive(deviceId, path || '/');
            } catch {
                rows = sidebarTreeRef.current[pathKey(path)] || [];
                if (!silent) setStatusText('Direct browsing · using cached listing');
            }
            if (gen !== loadGenRef.current) return;
            setEntries(rows);
            setSidebarTree((prev) => {
                const key = pathKey(path);
                const prevRows = prev[key];
                if (
                    prevRows &&
                    prevRows.length === rows.length &&
                    prevRows.every((r, i) => r.path === rows[i]?.path && r.size === rows[i]?.size)
                ) {
                    return prev;
                }
                return { ...prev, [key]: rows };
            });
        } finally {
            if (gen === loadGenRef.current) {
                setListLoading(false);
            }
        }
    }, [deviceId, mode, currentPath]);

    useEffect(() => {
        entriesRef.current = [];
        setEntries([]);
        setSelected(null);
        setPreview(null);
        loadFolder({ silent: false });
    }, [loadFolder]);

    // Refresh live listings when the panel tab returns to the foreground (keep list visible).
    useEffect(() => {
        const onVisibility = () => {
            if (document.visibilityState !== 'visible') return;
            if (!deviceId || !online || mode !== MODES.live) return;
            loadFolder({ silent: true });
        };
        document.addEventListener('visibilitychange', onVisibility);
        return () => document.removeEventListener('visibilitychange', onVisibility);
    }, [deviceId, online, mode, loadFolder]);

    // Soft poll: update quietly so the page does not flap between list and Loading…
    useEffect(() => {
        if (!deviceId || !online || mode !== MODES.live) return undefined;
        const id = window.setInterval(() => {
            if (document.visibilityState === 'visible') loadFolder({ silent: true });
        }, 30000);
        return () => window.clearInterval(id);
    }, [deviceId, online, mode, loadFolder]);

    useEffect(() => {
        if (deviceId) saveBrowsePrefs(deviceId, { mode, last_path: currentPath });
    }, [deviceId, mode, currentPath]);

    const filtered = useMemo(
        () => sortAndFilterFiles(entries, { query, sortField, sortAsc }),
        [entries, query, sortField, sortAsc]
    );

    const storageRoots = useMemo(() => {
        if (mode === MODES.mirror) {
            return mirrorRoots(mirror).map((r) => (typeof r === 'string' ? r : r.path));
        }
        const cached = sidebarTree[''] || [];
        if (cached.length) return cached.filter((e) => e.is_directory).map((e) => e.path);
        return [];
    }, [mode, mirror, sidebarTree]);

    const goTo = (path) => {
        setSelected(null);
        setPreview(null);
        setCurrentPath(normalizePath(path));
    };

    const switchToLive = async () => {
        if (!online) {
            alert('Direct browsing requires an online device.');
            return;
        }
        setMirror(emptyTreeSnapshot());
        setSidebarTree({});
        setMode(MODES.live);
        setCurrentPath('');
        setStatusText('Direct browsing');
        setSourceBadge('device');
    };

    const syncTree = async () => {
        if (!online) {
            alert('Sync tree requires an online device with storage access.');
            return;
        }
        setSyncLoading(true);
        setStatusText('Fetching tree and saving to server…');
        try {
            const snap = await syncFileTreeMirror(deviceId, {
                onProgress: (msg) => setStatusText(msg),
            });
            setMirror(snap);
            setMode(MODES.mirror);
            setCurrentPath('');
            setEntries(mirrorRoots(snap));
            setSourceBadge('device');
            setStatusText(`Source: Device · ${snap.entry_count} entries · saved to database`);
        } catch (err) {
            setStatusText(err.message || 'Sync failed');
        } finally {
            setSyncLoading(false);
        }
    };

    const saveInventoryToDatabase = async () => {
        if (!deviceId) return;
        const files = Array.isArray(mirror?.entries) ? mirror.entries : [];
        if (!files.length) {
            setStatusText('No file tree loaded — sync first');
            return;
        }
        setSyncLoading(true);
        setStatusText('Saving file inventory to database…');
        try {
            const res = await api.saveDeviceArtifacts(deviceId, { files });
            setStatusText(`Saved ${res?.saved?.files_saved || 0} file entries to database`);
        } catch (err) {
            setStatusText(err.message || 'Database save failed');
        } finally {
            setSyncLoading(false);
        }
    };

    const loadInventoryFromDatabase = async () => {
        if (!deviceId) return;
        setSyncLoading(true);
        setStatusText('Loading file inventory from database…');
        try {
            const res = await api.listDeviceArtifacts(deviceId, 'files', 100000);
            const entries = (res.files || []).map((e) => ({
                path: e.path,
                name: e.name,
                is_directory: e.is_directory,
                size: e.size || 0,
                permissions: e.permissions || '',
                modified_time: e.modified_time,
                parent_path: e.path.includes('/') ? e.path.replace(/\/[^/]+$/, '') : '',
            }));
            const snap = {
                type: 'file_tree',
                updated_at: new Date().toISOString(),
                source: 'database',
                roots: entries.filter((e) => !e.parent_path || e.parent_path === '' || e.parent_path === '/'),
                entries,
                entry_count: entries.length,
            };
            setMirror(snap);
            saveLocalMirror(deviceId, 'file_tree', snap);
            setMode(MODES.mirror);
            setCurrentPath('');
            setEntries(mirrorRoots(snap));
            setSourceBadge('database');
            setStatusText(`Loaded ${entries.length} file entries from database`);
        } catch (err) {
            setStatusText(err.message || 'Failed to load from database');
        } finally {
            setSyncLoading(false);
        }
    };

    const exportInventoryExcel = async () => {
        if (!deviceId) return;
        setSyncLoading(true);
        try {
            try {
                const files = Array.isArray(mirror?.entries) ? mirror.entries : [];
                if (files.length) await api.saveDeviceArtifacts(deviceId, { files });
            } catch {
                /* export stored rows */
            }
            const blob = await api.exportDeviceArtifacts(deviceId, 'files');
            downloadBlob(blob, `device-files-${new Date().toISOString().slice(0, 10)}.xlsx`);
            setStatusText('Excel export ready');
        } catch (err) {
            setStatusText(err.message || 'Excel export failed');
        } finally {
            setSyncLoading(false);
        }
    };

    const selectEntry = (entry) => {
        setSelected(entry);
        setDetailsOpen(true);
        setPreview(null);
    };

    const openEntry = (entry) => {
        if (entry.is_directory) {
            goTo(entry.path);
            return;
        }
        setSelected(entry);
        if (mode === MODES.mirror) setPreview(null);
        else if ((entry.size || 0) <= 5 * 1024 * 1024) loadPreview(entry);
    };

    const loadPreview = async (entry) => {
        try {
            const data = await readFileLive(deviceId, entry.path);
            const isImage = /\.(png|jpe?g|gif|webp)$/i.test(entry.name);
            let content = null;
            let text = '';
            if (isImage) {
                content =
                    toImageDataUrl(data) ||
                    toImageDataUrl(typeof data === 'object' ? data?.content : null);
                if (!content) {
                    setPreview({ entry, error: true });
                    return;
                }
            } else {
                text = typeof data === 'string' ? data : JSON.stringify(data, null, 2);
            }
            setPreview({ entry, content, text, isImage, error: false });
        } catch {
            setPreview({ entry, error: true });
        }
    };

    const downloadEntry = async (entry) => {
        await startDownload({
            deviceId,
            remotePath: entry.path,
            fileName: entry.name,
            fileSize: entry.size,
            deviceOnline: online,
        });
        if (!online) alert('Device is offline — download queued. See Download Requests.');
    };

    const toggleSelect = (path) => {
        setSelectedPaths((prev) => {
            const next = new Set(prev);
            if (next.has(path)) next.delete(path);
            else next.add(path);
            return next;
        });
    };

    const batchDownload = async () => {
        for (const path of selectedPaths) {
            const entry = entries.find((e) => e.path === path);
            if (entry) await downloadEntry(entry);
        }
    };

    const cycleSort = () => {
        const next = cycleFileSort(sortField, sortAsc);
        setSortField(next.sortField);
        setSortAsc(next.sortAsc);
    };

    const crumbs = breadcrumbSegments(currentPath);
    const modeLabel = mode === MODES.live ? 'Direct browsing' : 'Cached folder tree';
    const selectableFiles = filtered.filter((e) => !e.is_directory);

    return (
        <div className="hybrid-op-shell">
            <div className="fb-page">
                <header className="fb-topbar">
                    <div className="fb-topbar-row fb-topbar-primary">
                        <button
                            type="button"
                            className="fb-icon-btn"
                            onClick={() => setSidebarCollapsed((v) => !v)}
                            title="Toggle tree panel"
                        >
                            <Icon name="panelOpen" size={16} />
                        </button>
                        <nav className="fb-breadcrumb">
                            <button type="button" onClick={() => goTo('')} title="Home">
                                Home
                            </button>
                            {crumbs.map((c) => (
                                <React.Fragment key={c.path}>
                                    <Icon name="chevronRight" size={14} />
                                    <button
                                        type="button"
                                        className={currentPath === c.path ? 'fb-crumb-current' : ''}
                                        onClick={() => goTo(c.path)}
                                        title={c.path}
                                    >
                                        <TruncatedText text={c.label} />
                                    </button>
                                </React.Fragment>
                            ))}
                        </nav>
                        <div className="fb-topbar-primary-actions">
                            <Link to="/downloads" className="fb-btn-sm" title="Download requests">
                                <Icon name="listChecks" size={16} />
                                <span className="fb-btn-label">Requests</span>
                            </Link>
                            <button
                                type="button"
                                className="fb-btn-sm fb-btn-accent"
                                onClick={syncTree}
                                disabled={!deviceId || syncLoading || !online}
                                title="Sync full tree to server + local cache"
                            >
                                <Icon name="refresh" size={16} className={syncLoading ? 'spin' : ''} />
                                <span className="fb-btn-label">Sync tree</span>
                            </button>
                            <button
                                type="button"
                                className="fb-btn-sm"
                                onClick={saveInventoryToDatabase}
                                disabled={!deviceId || syncLoading}
                                title="Save current tree inventory to database"
                            >
                                <Icon name="server" size={16} />
                                <span className="fb-btn-label">Save DB</span>
                            </button>
                            <button
                                type="button"
                                className="fb-btn-sm"
                                onClick={loadInventoryFromDatabase}
                                disabled={!deviceId || syncLoading}
                                title="Load file inventory from database"
                            >
                                <Icon name="cloud" size={16} />
                                <span className="fb-btn-label">Load DB</span>
                            </button>
                            <button
                                type="button"
                                className="fb-btn-sm"
                                onClick={exportInventoryExcel}
                                disabled={!deviceId || syncLoading}
                                title="Export file inventory to Excel"
                            >
                                <Icon name="fileText" size={16} />
                                <span className="fb-btn-label">Excel</span>
                            </button>
                            {mode === MODES.mirror && (
                                <button
                                    type="button"
                                    className="fb-btn-sm"
                                    onClick={switchToLive}
                                    disabled={!deviceId || !online}
                                    title="Direct browsing"
                                >
                                    <Icon name="wifi" size={16} />
                                    <span className="fb-btn-label">Direct browse</span>
                                </button>
                            )}
                            <button
                                type="button"
                                className="fb-icon-btn"
                                onClick={() => setDetailsOpen((v) => !v)}
                                title="Toggle details"
                            >
                                <Icon name="panelClose" size={16} />
                            </button>
                        </div>
                    </div>
                    <div className="fb-topbar-row fb-topbar-secondary">
                        <DevicePicker
                            id="file-device-select"
                            variant="hybrid"
                            devices={devices}
                            value={deviceId}
                            onChange={setDeviceId}
                            loading={devicesLoading}
                            placeholder="Select device…"
                        />
                        <button type="button" className="fb-icon-btn" onClick={() => reload()} title="Refresh devices">
                            <Icon name="refresh" size={16} />
                        </button>
                        <div className="fb-search-wrap">
                            <Icon name="search" size={16} />
                            <input
                                type="search"
                                className="fb-search"
                                placeholder="Search name or path…"
                                value={query}
                                onChange={(e) => setQuery(e.target.value)}
                            />
                        </div>
                        <button type="button" className="fb-btn-sm" onClick={cycleSort} title="Change sort order">
                            <Icon name="sort" size={16} />
                            <span id="fb-sort-label">{sortFieldLabel(sortField)}</span>
                            <Icon name={sortAsc ? 'arrowUp' : 'arrowDown'} size={14} />
                        </button>
                        <div className="fb-view-toggle">
                            <button
                                type="button"
                                className={`fb-icon-btn ${view === 'list' ? 'fb-active' : ''}`}
                                onClick={() => setView('list')}
                                title="List view"
                            >
                                <Icon name="list" size={16} />
                            </button>
                            <button
                                type="button"
                                className={`fb-icon-btn ${view === 'grid' ? 'fb-active' : ''}`}
                                onClick={() => setView('grid')}
                                title="Grid view"
                            >
                                <Icon name="grid" size={16} />
                            </button>
                        </div>
                    </div>
                </header>

                <p className="fb-mirror-meta">
                    {modeLabel}
                    {statusText ? ` · ${statusText}` : ''}
                    {' · '}
                    <span className={`fb-device-badge ${online ? 'fb-device-online' : 'fb-device-offline'}`}>
                        Source: {sourceBadge}
                    </span>
                </p>

                <div className="fb-shell rp-shell" ref={shellRef} data-rp-layout="fb">
                    <aside className={`fb-sidebar rp-col ${sidebarCollapsed ? 'fb-sidebar-collapsed' : ''}`}>
                        <section className="fb-sidebar-section">
                            <div className="fb-section-head">
                                <h5 className="fb-section-title">
                                    <Icon name="star" size={14} /> Bookmarks
                                </h5>
                            </div>
                            <div className="fb-sidebar-scroll">
                                {FILE_BOOKMARKS.map((b) => (
                                    <button
                                        key={b.path}
                                        type="button"
                                        className="fb-bookmark-row"
                                        onClick={() => goTo(b.path)}
                                        title={b.path}
                                    >
                                        <Icon name={b.icon} size={16} className={b.colorClass} />
                                        <TruncatedText text={b.label} title={b.path} />
                                    </button>
                                ))}
                            </div>
                        </section>
                        <section className="fb-sidebar-section fb-sidebar-grow">
                            <div className="fb-section-head">
                                <h5 className="fb-section-title">
                                    <Icon name="folder" size={14} /> File tree
                                </h5>
                                <div className="fb-tree-tools">
                                    <button type="button" className="fb-tree-tool" onClick={() => setExpanded(new Set(['']))} title="Expand all">
                                        <Icon name="unfold" size={12} />
                                    </button>
                                    <button type="button" className="fb-tree-tool" onClick={() => setExpanded(new Set())} title="Collapse all">
                                        <Icon name="fold" size={12} />
                                    </button>
                                </div>
                            </div>
                            <div className="fb-sidebar-scroll fb-tree-host">
                                {!deviceId ? (
                                    <p className="fb-empty-hint">Select a device</p>
                                ) : (
                                    <FileTree
                                        mode={mode}
                                        mirror={mirror}
                                        sidebarTree={sidebarTree}
                                        currentPath={currentPath}
                                        expanded={expanded}
                                        setExpanded={setExpanded}
                                        onSelect={goTo}
                                        online={online}
                                    />
                                )}
                            </div>
                        </section>
                        <section className="fb-sidebar-section">
                            <h5 className="fb-section-title">
                                <Icon name="storage" size={14} /> Storage roots
                            </h5>
                            <div className="fb-sidebar-scroll">
                                {storageRoots.length === 0 ? (
                                    <p className="fb-empty-hint">Browse live or sync tree</p>
                                ) : (
                                    storageRoots.map((root) => {
                                        const name = String(root).replace(/[/\\]+$/, '').split(/[/\\]/).pop() || root;
                                        return (
                                            <button
                                                key={root}
                                                type="button"
                                                className="fb-bookmark-row"
                                                onClick={() => goTo(root)}
                                                title={root}
                                            >
                                                <Icon name="storage" size={16} />
                                                <TruncatedText text={name} title={root} />
                                            </button>
                                        );
                                    })
                                )}
                            </div>
                        </section>
                    </aside>

                    {!sidebarCollapsed && <div {...bindSplitter('sidebar')} />}

                    <main className="fb-main rp-col-main">
                        <div className="fb-file-area">
                            {!deviceId ? (
                                <p className="fb-empty-state">Select a device to browse files</p>
                            ) : (bootstrapLoading || listLoading) && filtered.length === 0 ? (
                                <div className="fb-loading">
                                    <Icon name="spinner" size={28} className="spin" />
                                    Loading…
                                </div>
                            ) : filtered.length === 0 ? (
                                <p className="fb-empty-state">Empty folder — sync tree or wait for bootstrap.</p>
                            ) : (
                                <>
                                    <FileSelectionBar
                                        visible={mode === MODES.live && selectableFiles.length > 0}
                                        totalFiles={selectableFiles.length}
                                        selectedCount={selectedPaths.size}
                                        onSelectAll={(checked) => {
                                            if (checked) setSelectedPaths(new Set(selectableFiles.map((f) => f.path)));
                                            else setSelectedPaths(new Set());
                                        }}
                                        onClear={() => setSelectedPaths(new Set())}
                                        onBatchDownload={batchDownload}
                                    />
                                    <FileBrowserBanners
                                        mode={mode}
                                        online={online}
                                        mirrorStale={isMirrorStale(mirror)}
                                        mirrorTruncated={mirror?.truncated}
                                    />
                                    {view === 'grid' ? (
                                        <FileGridView
                                            files={filtered}
                                            mode={mode}
                                            online={online}
                                            selectedPaths={selectedPaths}
                                            activePath={selected?.path}
                                            onRowClick={selectEntry}
                                            onOpen={openEntry}
                                            onToggleSelect={toggleSelect}
                                            onDownload={downloadEntry}
                                            onShowDetails={selectEntry}
                                        />
                                    ) : (
                                        <FileListTable
                                            files={filtered}
                                            mode={mode}
                                            online={online}
                                            selectedPaths={selectedPaths}
                                            activePath={selected?.path}
                                            onRowClick={selectEntry}
                                            onOpen={openEntry}
                                            onToggleSelect={toggleSelect}
                                            onDownload={downloadEntry}
                                            onShowDetails={selectEntry}
                                        />
                                    )}
                                </>
                            )}
                        </div>
                    </main>

                    {detailsOpen && <div {...bindSplitter('details')} />}

                    <aside className={`fb-details rp-col ${detailsOpen ? '' : 'fb-details-hidden'}`}>
                        <div className="fb-details-header">
                            <h5>Details</h5>
                        </div>
                        <div className="fb-details-body">
                            <FileDetailsPanel
                                entry={selected}
                                mode={mode}
                                online={online}
                                preview={preview}
                                onPreview={loadPreview}
                                onDownload={downloadEntry}
                                onLivePreview={loadPreview}
                                onOpen={openEntry}
                            />
                        </div>
                    </aside>
                </div>

                <footer className="fb-statusbar">
                    <span className="fb-status-item">
                        <span className={`fb-status-dot ${online ? 'fb-online' : 'fb-offline'}`} />
                        {online ? 'Online' : 'Offline'}
                    </span>
                    <span className="fb-status-item fb-status-path" title={currentPath || '/'}>
                        <TruncatedText text={currentPath || 'Home'} />
                    </span>
                    <span className="fb-status-count">
                        {filtered.length} item{filtered.length === 1 ? '' : 's'}
                    </span>
                </footer>
            </div>
        </div>
    );
}

