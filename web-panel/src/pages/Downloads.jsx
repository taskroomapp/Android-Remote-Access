import React, { useCallback, useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import DevicePicker from '../components/DevicePicker';
import Icon, { statusIcon } from '../components/ui/Icon';
import TruncatedText from '../components/hybrid/TruncatedText';
import { useDevices, isDeviceOnline } from '../hooks/useDevices';
import {
    appealAndRetry,
    applyLiveProgress,
    cancelDownload,
    clearCompletedLocal,
    fetchServerTransfers,
    getLocalTransfers,
    mergeTransfers,
    onTransferProgress,
    processWaitingDownloads,
    removeLocalTransfer,
    retryDownload,
} from '../lib/transfers';
import { api } from '../api/client';
import { parentPath } from '../lib/paths';

function formatSize(bytes) {
    if (bytes == null || Number.isNaN(Number(bytes))) return '—';
    const n = Number(bytes);
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
    return `${(n / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

function xferBadgeClass(status) {
    if (status === 'completed') return 'xfer-badge-ok';
    if (['in_progress', 'pending', 'waiting'].includes(status)) return 'xfer-badge-active';
    if (['error', 'cancelled'].includes(status)) return 'xfer-badge-err';
    return 'xfer-badge-neutral';
}

export default function DownloadsPage() {
    const [searchParams, setSearchParams] = useSearchParams();
    const { devices, loading: devicesLoading } = useDevices();
    const [filterDevice, setFilterDevice] = useState(() => searchParams.get('device') || '');
    const [filterStatus, setFilterStatus] = useState('');
    const [rows, setRows] = useState([]);
    const [loading, setLoading] = useState(true);
    const [autoResume, setAutoResume] = useState(true);

    useEffect(() => {
        if (filterDevice) {
            setSearchParams({ device: filterDevice }, { replace: true });
        } else {
            setSearchParams({}, { replace: true });
        }
    }, [filterDevice, setSearchParams]);

    const load = useCallback(async ({ quiet = false } = {}) => {
        if (!quiet) setLoading(true);
        try {
            const filters = { limit: 200 };
            if (filterDevice) filters.device_id = filterDevice;
            if (filterStatus) filters.status = filterStatus;
            const [server, local] = await Promise.all([
                fetchServerTransfers(filters),
                getLocalTransfers(filters),
            ]);
            setRows((prev) => {
                const merged = applyLiveProgress(mergeTransfers(server, local));
                return merged.map((row) => {
                    const cur = prev.find((r) => r.id === row.id);
                    if (
                        cur &&
                        ['in_progress', 'pending'].includes(cur.status) &&
                        (cur.bytes_written || 0) > (row.bytes_written || 0)
                    ) {
                        return {
                            ...row,
                            bytes_written: cur.bytes_written,
                            file_size: cur.file_size || row.file_size,
                            status: cur.status,
                        };
                    }
                    return row;
                });
            });
        } finally {
            setLoading(false);
        }
    }, [filterDevice, filterStatus]);

    useEffect(() => {
        load();
    }, [load]);

    useEffect(() => {
        if (!autoResume) return undefined;
        let cancelled = false;
        (async () => {
            const local = await getLocalTransfers({ limit: 200 });
            if (!cancelled) {
                await processWaitingDownloads(
                    local,
                    (id) => isDeviceOnline(devices.find((d) => d.id === id)),
                    autoResume
                );
            }
        })();
        return () => {
            cancelled = true;
        };
    }, [devices, autoResume]);

    useEffect(() => {
        const unsub = onTransferProgress((row) => {
            setRows((prev) => {
                const idx = prev.findIndex((r) => r.id === row.id);
                if (idx === -1) {
                    return applyLiveProgress([row, ...prev]);
                }
                const next = [...prev];
                next[idx] = {
                    ...next[idx],
                    ...row,
                    bytes_written: Math.max(next[idx].bytes_written || 0, row.bytes_written || 0),
                };
                return next;
            });
        });
        return unsub;
    }, []);

    useEffect(() => {
        const hasActive = rows.some((r) => ['pending', 'in_progress', 'waiting'].includes(r.status));
        if (!hasActive) return undefined;
        const t = setInterval(() => load({ quiet: true }), 4000);
        return () => clearInterval(t);
    }, [rows, load]);

    const stats = {
        total: rows.length,
        completed: rows.filter((r) => r.status === 'completed').length,
        active: rows.filter((r) => ['pending', 'in_progress', 'waiting'].includes(r.status)).length,
        failed: rows.filter((r) => ['error', 'cancelled'].includes(r.status)).length,
    };

    const deviceOnline = (id) => isDeviceOnline(devices.find((d) => d.id === id));

    const handleRetry = async (row) => {
        await retryDownload(row, deviceOnline(row.device_id), autoResume);
        await load();
    };

    const handleAppeal = async (row) => {
        await appealAndRetry(row, deviceOnline(row.device_id));
        await load();
    };

    const handleRemove = async (row) => {
        await removeLocalTransfer(row.id);
        await load();
    };

    const purgeCompleted = async () => {
        try {
            await api.purgeCompletedTransfers(filterDevice || undefined);
        } catch {
            /* local only */
        }
        await clearCompletedLocal();
        await load();
    };

    const deviceOptions = [{ id: '', friendly_name: 'All devices' }, ...devices];

    return (
        <div className="hybrid-op-shell">
            <div className="xfer-page">
                <header className="xfer-topbar">
                    <h3 className="xfer-title">
                        <Icon name="downloads" size={20} />
                        Download Requests
                    </h3>
                    <div className="xfer-topbar-actions">
                        <DevicePicker
                            id="xfer-device-filter"
                            variant="hybrid"
                            devices={deviceOptions}
                            value={filterDevice}
                            onChange={setFilterDevice}
                            loading={devicesLoading}
                            placeholder="All devices"
                        />
                        <select
                            className="fb-select"
                            value={filterStatus}
                            onChange={(e) => setFilterStatus(e.target.value)}
                        >
                            <option value="">All statuses</option>
                            <option value="waiting">Waiting</option>
                            <option value="pending">Pending</option>
                            <option value="in_progress">In progress</option>
                            <option value="completed">Completed</option>
                            <option value="error">Failed</option>
                            <option value="cancelled">Cancelled</option>
                        </select>
                        <label className="suite-muted xfer-auto-resume">
                            <input type="checkbox" checked={autoResume} onChange={(e) => setAutoResume(e.target.checked)} />
                            Auto-resume
                        </label>
                        <button type="button" className="fb-btn-sm" onClick={load}>
                            <Icon name="refresh" size={16} />
                        </button>
                        <button type="button" className="fb-btn-sm" onClick={purgeCompleted}>
                            Clear completed
                        </button>
                    </div>
                </header>

                <div className="xfer-stats">
                    <div className="xfer-stat">
                        <span className="xfer-stat-num">{stats.total}</span>
                        <span className="xfer-stat-label">Total</span>
                    </div>
                    <div className="xfer-stat">
                        <span className="xfer-stat-num xfer-stat-ok">{stats.completed}</span>
                        <span className="xfer-stat-label">Completed</span>
                    </div>
                    <div className="xfer-stat">
                        <span className="xfer-stat-num xfer-stat-active">{stats.active}</span>
                        <span className="xfer-stat-label">Pending</span>
                    </div>
                    <div className="xfer-stat">
                        <span className="xfer-stat-num xfer-stat-err">{stats.failed}</span>
                        <span className="xfer-stat-label">Incomplete</span>
                    </div>
                </div>

                <div className="xfer-table-wrap">
                    <table className="xfer-table">
                        <thead>
                            <tr>
                                <th>Device</th>
                                <th>File</th>
                                <th className="xfer-col-path">Path</th>
                                <th>Size</th>
                                <th>Progress</th>
                                <th>Status</th>
                                <th>Started</th>
                                <th>Actions</th>
                            </tr>
                        </thead>
                        <tbody>
                            {loading && (
                                <tr>
                                    <td colSpan={8} className="xfer-loading">
                                        Loading…
                                    </td>
                                </tr>
                            )}
                            {!loading && rows.length === 0 && (
                                <tr>
                                    <td colSpan={8} className="xfer-empty">
                                        No download requests yet.
                                    </td>
                                </tr>
                            )}
                            {!loading &&
                                rows.map((row) => (
                                    <TransferRow
                                        key={row.id}
                                        row={row}
                                        devices={devices}
                                        onRetry={() => handleRetry(row)}
                                        onAppeal={() => handleAppeal(row)}
                                        onRemove={() => handleRemove(row)}
                                        onCancel={() => {
                                            cancelDownload(row.id);
                                            window.setTimeout(load, 400);
                                        }}
                                    />
                                ))}
                        </tbody>
                    </table>
                </div>
            </div>
        </div>
    );
}

function TransferRow({ row, devices, onRetry, onAppeal, onRemove, onCancel }) {
    const device = devices.find((d) => d.id === row.device_id);
    const fileName = row.remote_path?.split(/[/\\]/).pop() || '—';
    const pct =
        row.file_size > 0
            ? Math.min(100, Math.floor(((row.bytes_written || 0) / row.file_size) * 1000) / 10)
            : null;
    const parent = parentPath(row.remote_path);

    return (
        <tr>
            <td className="xfer-col-device">
                <TruncatedText text={device?.friendly_name || row.device_id?.slice(0, 8)} />
            </td>
            <td className="xfer-col-name">
                <TruncatedText text={fileName} title={row.remote_path} />
            </td>
            <td className="xfer-col-path">
                <TruncatedText text={row.remote_path} title={row.remote_path} />
            </td>
            <td className="xfer-col-size">{formatSize(row.file_size)}</td>
            <td>
                {pct != null ? (
                    <div className="xfer-progress">
                        <div className="xfer-progress-bar" style={{ width: `${pct}%` }} />
                        <span>
                            {Number.isInteger(pct) ? pct : pct.toFixed(1)}% ·{' '}
                            {formatSize(row.bytes_written)} / {formatSize(row.file_size)}
                        </span>
                    </div>
                ) : (
                    <span>{formatSize(row.bytes_written)}</span>
                )}
                {row.error_message && <div className="error-text">{row.error_message}</div>}
            </td>
            <td>
                <span className={`xfer-badge ${xferBadgeClass(row.status)}`}>
                    <Icon name={statusIcon(row.status)} size={12} className={row.status === 'in_progress' ? 'spin' : ''} />
                    {row.status}
                </span>
                {row.resumed && <span className="xfer-appeal-tag">resumed</span>}
                {row.transfer_id && <span className="xfer-tid">{row.transfer_id}</span>}
            </td>
            <td className="xfer-col-time">
                {row.started_at ? new Date(row.started_at).toLocaleString() : '—'}
            </td>
            <td className="xfer-col-actions">
                {['error', 'cancelled', 'waiting'].includes(row.status) && (
                    <button type="button" className="xfer-act-btn" title="Retry" onClick={onRetry}>
                        <Icon name="retry" size={16} />
                    </button>
                )}
                {row.status === 'error' && (
                    <button type="button" className="xfer-act-btn" title="Appeal & retry" onClick={onAppeal}>
                        <Icon name="send" size={16} />
                    </button>
                )}
                {row.status === 'in_progress' && (
                    <button type="button" className="xfer-act-btn xfer-act-danger" title="Cancel" onClick={onCancel}>
                        <Icon name="cancel" size={16} />
                    </button>
                )}
                <Link
                    className="xfer-act-btn"
                    title="Open in file browser"
                    to={`/files?device=${encodeURIComponent(row.device_id)}`}
                    state={{ path: parent }}
                >
                    <Icon name="folder" size={16} />
                </Link>
                <button type="button" className="xfer-act-btn xfer-act-danger" title="Remove from list" onClick={onRemove}>
                    <Icon name="trash" size={16} />
                </button>
            </td>
        </tr>
    );
}
