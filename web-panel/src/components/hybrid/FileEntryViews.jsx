import React from 'react';
import { Link } from 'react-router-dom';
import Icon, { fileTypeIcon } from '../ui/Icon';
import TruncatedText from './TruncatedText';
import { formatFileModified, formatFileSize, getFileTypeLabel } from '../../lib/fileBrowserUi';

export function FileBrowserBanners({ mode, online, mirrorStale, mirrorTruncated }) {
    const mirror = mode === 'mirror';
    if (!mirror) return null;
    const banners = [];
    if (!online) {
        banners.push('Offline cached view — downloads require the device online.');
    }
    if (mirrorTruncated) {
        banners.push('Tree truncated — sync when online for a full snapshot.');
    }
    if (online) {
        banners.push(
            mirrorStale
                ? 'Cached folder tree — listings may be stale. Sync tree or switch to direct browsing before downloading.'
                : 'Cached folder tree — use Sync tree or direct browsing before downloading if listings look wrong.'
        );
    }
    if (!banners.length) return null;
    return banners.map((text) => (
        <div key={text} className="fb-banner fb-banner-warn">
            {text}
        </div>
    ));
}

export function FileSelectionBar({ visible, totalFiles, selectedCount, onSelectAll, onClear, onBatchDownload }) {
    if (!visible) return null;
    return (
        <div className="fb-selection-bar">
            <div className="fb-selection-left">
                <label className="fb-select-all">
                    <input type="checkbox" onChange={(e) => onSelectAll(e.target.checked)} />
                    <span>Select all</span>
                </label>
                <span className="suite-muted">({totalFiles} files)</span>
            </div>
            <div className="fb-selection-actions">
                {selectedCount > 0 && (
                    <>
                        <button type="button" className="fb-btn-sm" onClick={onClear}>
                            Clear
                        </button>
                        <button type="button" className="fb-btn-sm fb-btn-accent" onClick={onBatchDownload}>
                            <Icon name="download" size={16} />
                            Download selected ({selectedCount})
                        </button>
                    </>
                )}
            </div>
        </div>
    );
}

export function FileListTable({
    files,
    mode,
    online,
    selectedPaths,
    activePath,
    onRowClick,
    onOpen,
    onToggleSelect,
    onDownload,
    onShowDetails,
}) {
    const canDownload = mode !== 'mirror' || online;

    return (
        <div className="fb-table-wrap">
            <table className="fb-table">
                <thead>
                    <tr>
                        <th className="fb-col-check" />
                        <th className="fb-col-icon" />
                        <th>Name</th>
                        <th className="fb-col-type">Type</th>
                        <th className="fb-col-size">Size</th>
                        <th className="fb-col-date">Modified</th>
                        <th className="fb-col-actions" />
                    </tr>
                </thead>
                <tbody>
                    {files.map((file) => {
                        const sel = selectedPaths.has(file.path);
                        const active = activePath === file.path;
                        return (
                            <tr
                                key={file.path}
                                className={`fb-row ${sel ? 'fb-row-selected' : ''} ${active ? 'fb-row-active' : ''}`}
                                title={file.path}
                                onClick={() => onRowClick(file)}
                                onDoubleClick={() => onOpen(file)}
                            >
                                <td className="fb-col-check" onClick={(e) => e.stopPropagation()}>
                                    {!file.is_directory && mode === 'live' && (
                                        <input
                                            type="checkbox"
                                            checked={sel}
                                            onChange={() => onToggleSelect(file.path)}
                                        />
                                    )}
                                </td>
                                <td className="fb-col-icon">
                                    <Icon name={fileTypeIcon(file.name, file.is_directory)} size={20} />
                                </td>
                                <td>
                                    <div className="fb-name-cell">
                                        <TruncatedText text={file.name} className="fb-name" />
                                        <TruncatedText text={file.path} className="fb-subtitle" />
                                    </div>
                                </td>
                                <td className="fb-col-type">{getFileTypeLabel(file)}</td>
                                <td className="fb-col-size">
                                    {file.is_directory ? '—' : formatFileSize(file.size)}
                                </td>
                                <td className="fb-col-date">{formatFileModified(file)}</td>
                                <td className="fb-col-actions">
                                    <div className="fb-row-actions">
                                        <button
                                            type="button"
                                            title="Open"
                                            onClick={(e) => {
                                                e.stopPropagation();
                                                onOpen(file);
                                            }}
                                        >
                                            <Icon name="folderOpen" size={16} />
                                        </button>
                                        {canDownload && !file.is_directory && (
                                            <button
                                                type="button"
                                                title="Download"
                                                onClick={(e) => {
                                                    e.stopPropagation();
                                                    onDownload(file);
                                                }}
                                            >
                                                <Icon name="download" size={16} />
                                            </button>
                                        )}
                                        <button
                                            type="button"
                                            title="Info"
                                            onClick={(e) => {
                                                e.stopPropagation();
                                                onShowDetails(file);
                                            }}
                                        >
                                            <Icon name="info" size={16} />
                                        </button>
                                    </div>
                                </td>
                            </tr>
                        );
                    })}
                </tbody>
            </table>
        </div>
    );
}

export function FileGridView({
    files,
    mode,
    online,
    selectedPaths,
    activePath,
    onRowClick,
    onOpen,
    onToggleSelect,
    onDownload,
    onShowDetails,
}) {
    const canDownload = mode !== 'mirror' || online;

    return (
        <div className="fb-grid">
            {files.map((file) => {
                const sel = selectedPaths.has(file.path);
                const active = activePath === file.path;
                return (
                    <div
                        key={file.path}
                        className={`fb-grid-item ${sel ? 'fb-grid-selected' : ''} ${active ? 'fb-grid-active' : ''}`}
                        title={`${file.name}\n${file.path}`}
                        onClick={(e) => onRowClick(file, e)}
                        onDoubleClick={() => onOpen(file)}
                    >
                        {!file.is_directory && mode === 'live' && (
                            <label className="fb-grid-check" onClick={(e) => e.stopPropagation()}>
                                <input
                                    type="checkbox"
                                    checked={sel}
                                    onChange={() => onToggleSelect(file.path)}
                                />
                            </label>
                        )}
                        <div className="fb-grid-icon">
                            <Icon name={fileTypeIcon(file.name, file.is_directory)} size={36} />
                        </div>
                        <TruncatedText text={file.name} lines={2} className="fb-grid-name" as="p" />
                        <p className="fb-grid-meta">
                            {file.is_directory ? 'Folder' : formatFileSize(file.size)}
                        </p>
                        <div className="fb-grid-actions">
                            <button type="button" title="Open" onClick={(e) => { e.stopPropagation(); onOpen(file); }}>
                                <Icon name="folderOpen" size={14} />
                            </button>
                            {canDownload && !file.is_directory && (
                                <button type="button" title="Download" onClick={(e) => { e.stopPropagation(); onDownload(file); }}>
                                    <Icon name="download" size={14} />
                                </button>
                            )}
                            <button type="button" title="Info" onClick={(e) => { e.stopPropagation(); onShowDetails(file); }}>
                                <Icon name="info" size={14} />
                            </button>
                        </div>
                    </div>
                );
            })}
        </div>
    );
}

export function FileDetailsPanel({ entry, mode, online, preview, onPreview, onDownload, onLivePreview, onOpen }) {
    if (!entry) {
        return <p className="fb-empty-hint">Select a file or folder</p>;
    }

    return (
        <>
            <div className="fb-details-head">
                <div className="fb-details-icon">
                    <Icon name={fileTypeIcon(entry.name, entry.is_directory)} size={40} />
                </div>
                <TruncatedText text={entry.name} className="fb-details-title" lines={2} as="div" />
                <div className="fb-details-type">{getFileTypeLabel(entry)}</div>
            </div>
            <dl className="fb-details-meta">
                <div>
                    <dt>Path</dt>
                    <dd className="fb-details-path hy-path-wrap" title={entry.path}>
                        {entry.path}
                    </dd>
                </div>
                {!entry.is_directory && (
                    <>
                        <div>
                            <dt>Size</dt>
                            <dd>{formatFileSize(entry.size)}</dd>
                        </div>
                        <div>
                            <dt>Modified</dt>
                            <dd>{formatFileModified(entry)}</dd>
                        </div>
                    </>
                )}
            </dl>
            <div className="fb-details-actions">
                {entry.is_directory ? (
                    <button type="button" className="fb-btn fb-btn-primary" onClick={() => onOpen?.(entry)}>
                        <Icon name="folderOpen" size={16} />
                        Open folder
                    </button>
                ) : mode === 'mirror' ? (
                    <>
                        <p className="suite-muted">Mirror metadata only.</p>
                        {online && !entry.is_directory && (
                            <button type="button" className="fb-btn fb-btn-primary" onClick={() => onDownload(entry)}>
                                <Icon name="download" size={16} />
                                Download from device
                            </button>
                        )}
                    </>
                ) : (
                    <>
                        {!entry.is_directory && (entry.size || 0) <= 5 * 1024 * 1024 && (
                            <button type="button" className="fb-btn" onClick={() => onLivePreview(entry)}>
                                <Icon name="eye" size={16} />
                                Preview
                            </button>
                        )}
                        {!entry.is_directory && (
                            <button type="button" className="fb-btn fb-btn-primary" onClick={() => onDownload(entry)}>
                                <Icon name="download" size={16} />
                                Download
                            </button>
                        )}
                    </>
                )}
            </div>
            {preview?.entry?.path === entry.path && preview.content && (
                <div className="fb-details-preview">
                    {preview.isImage ? (
                        <img src={preview.content} alt={entry.name} className="file-preview-img" />
                    ) : (
                        <pre className="file-preview-text">{preview.text}</pre>
                    )}
                </div>
            )}
            {preview?.error && <p className="suite-muted">Preview unavailable.</p>}
            <p className="suite-muted">
                <Link to="/downloads">Download requests</Link>
            </p>
        </>
    );
}
