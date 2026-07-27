import React, { useEffect } from 'react';
import Icon, { fileTypeIcon, statusIcon } from './ui/Icon';

export default function LoadingScreen({ message = 'Loading...' }) {
    return (
        <div className="loading-screen">
            <Icon name="spinner" size={32} className="spin" />
            <p>{message}</p>
        </div>
    );
}

export function LoadingOverlay({ visible }) {
    if (!visible) return null;
    return (
        <div className="loading-overlay">
            <Icon name="spinner" size={24} className="spin" />
        </div>
    );
}

export function ActionButton({ children, onClick, variant = 'primary', disabled, loading, icon, className = '' }) {
    const iconNode =
        typeof icon === 'string' ? <Icon name={icon} size={16} className={loading ? 'spin' : ''} /> : icon;
    return (
        <button
            type="button"
            className={`action-btn action-btn-${variant} ${className}`}
            onClick={onClick}
            disabled={disabled || loading}
        >
            {loading ? <Icon name="spinner" size={16} className="spin" /> : iconNode && <span className="btn-icon">{iconNode}</span>}
            {children}
        </button>
    );
}

export function StatusBadge({ status, children }) {
    const statusClass = {
        online: 'badge-success',
        offline: 'badge-muted',
        success: 'badge-success',
        failed: 'badge-danger',
        error: 'badge-danger',
        pending: 'badge-warning',
        in_progress: 'badge-info',
        completed: 'badge-success',
        cancelled: 'badge-muted',
        waiting: 'badge-warning',
    }[status] || 'badge-default';

    return (
        <span className={`status-badge ${statusClass}`}>
            <Icon name={statusIcon(status)} size={14} />
            {children || status}
        </span>
    );
}

export function EmptyState({ icon = 'folder', title = 'No data', message, action }) {
    return (
        <div className="empty-state">
            <span className="empty-icon"><Icon name={icon} size={40} /></span>
            <h3>{title}</h3>
            {message && <p>{message}</p>}
            {action && (
                <button type="button" className="action-btn action-btn-primary" onClick={action.onClick}>
                    {action.label}
                </button>
            )}
        </div>
    );
}

export function Toast({ message, type = 'info', onClose }) {
    useEffect(() => {
        const timer = setTimeout(onClose, 4000);
        return () => clearTimeout(timer);
    }, [onClose]);

    const iconName = { success: 'success', error: 'error', warning: 'warning', info: 'pending' }[type] || 'pending';

    return (
        <div className={`toast toast-${type}`}>
            <span className="toast-icon"><Icon name={iconName} size={18} /></span>
            <span className="toast-message">{message}</span>
            <button type="button" className="toast-close" onClick={onClose} aria-label="Close">
                <Icon name="close" size={16} />
            </button>
        </div>
    );
}

export function DevicePicker({ devices, selected, onChange, showOffline = true, label = 'Device' }) {
    return (
        <div className="device-picker">
            <label><Icon name="devices" size={14} /> {label}</label>
            <select value={selected || ''} onChange={(e) => onChange(e.target.value || null)}>
                <option value="">Select device...</option>
                {devices.map((device) => (
                    <option key={device.id} value={device.id}>
                        {device.friendly_name}{' '}
                        {device.status === 'online' || device.online ? '(online)' : '(offline)'}
                        {device.battery_level !== undefined && ` — ${device.battery_level}%`}
                    </option>
                ))}
            </select>
            {selected && !showOffline && !devices.find((d) => d.id === selected && (d.status === 'online' || d.online)) && (
                <span className="picker-warning"><Icon name="warning" size={14} /> Device appears offline</span>
            )}
        </div>
    );
}

export function Breadcrumbs({ items, onNavigate }) {
    return (
        <nav className="breadcrumbs">
            {items.map((item, index) => (
                <React.Fragment key={index}>
                    {index > 0 && <Icon name="chevronRight" size={14} className="breadcrumb-sep" />}
                    <button
                        type="button"
                        className={`breadcrumb-item ${index === items.length - 1 ? 'active' : ''}`}
                        onClick={() => onNavigate && onNavigate(item.path)}
                    >
                        {index === 0 && <Icon name="home" size={14} />}
                        {item.label}
                    </button>
                </React.Fragment>
            ))}
        </nav>
    );
}

export function FileIcon({ name, isDirectory }) {
    const iconName = fileTypeIcon(name, isDirectory);
    return (
        <span className={`file-icon ${isDirectory ? 'folder' : 'file'}`}>
            <Icon name={iconName} size={20} />
        </span>
    );
}
