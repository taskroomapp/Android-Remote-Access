import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useDevices } from '../context/DeviceContext';
import Icon from '../components/ui/Icon';
import { DEVICE_INTERFACES, DEVICE_INTERFACE_CHIPS } from '../lib/deviceInterfaces';
import { formatAndroidVersion } from '../lib/androidVersion';

export default function Devices() {
    const navigate = useNavigate();
    const { devices, loading, loadDevices } = useDevices();
    const [filter, setFilter] = useState('all');
    const [searchTerm, setSearchTerm] = useState('');
    const [error, setError] = useState(null);
    const [modalDevice, setModalDevice] = useState(null);

    const filteredDevices = devices.filter((device) => {
        const matchesFilter = filter === 'all' || device.status === filter;
        const q = searchTerm.toLowerCase();
        const matchesSearch =
            (device.friendly_name || '').toLowerCase().includes(q) ||
            (device.owner || '').toLowerCase().includes(q) ||
            (device.os_version || '').toLowerCase().includes(q) ||
            (device.hardware_model || '').toLowerCase().includes(q);
        return matchesFilter && matchesSearch;
    });

    useEffect(() => {
        if (!modalDevice) return undefined;
        const onKey = (e) => {
            if (e.key === 'Escape') setModalDevice(null);
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [modalDevice]);

    const goInterface = (deviceId, iface) => {
        setModalDevice(null);
        navigate(iface.path(deviceId));
    };

    if (loading) {
        return (
            <div className="devices-loading">
                <div className="spinner"></div>
                <p>Loading devices...</p>
            </div>
        );
    }

    return (
        <div className="devices-page">
            <header className="page-header">
                <div className="header-left">
                    <h1>Device Management</h1>
                    <p className="page-subtitle">
                        Open any interface for a device — the device is selected; use Fetch or Sync on that page.
                    </p>
                    <div className="device-count">
                        <span className="total">{devices.length} devices</span>
                        <span className="online">{devices.filter((d) => d.status === 'online').length} online</span>
                        <span className="offline">{devices.filter((d) => d.status === 'offline').length} offline</span>
                    </div>
                </div>
                <div className="header-actions">
                    <button
                        type="button"
                        className="btn-secondary"
                        onClick={() => {
                            setError(null);
                            loadDevices();
                        }}
                    >
                        <Icon name="refresh" size={16} /> Refresh
                    </button>
                </div>
            </header>

            <div className="filters-bar">
                <div className="search-box">
                    <span className="search-icon">
                        <Icon name="search" size={18} />
                    </span>
                    <input
                        type="text"
                        placeholder="Search devices..."
                        value={searchTerm}
                        onChange={(e) => setSearchTerm(e.target.value)}
                    />
                </div>
                <div className="filter-buttons">
                    <button
                        type="button"
                        className={`filter-btn ${filter === 'all' ? 'active' : ''}`}
                        onClick={() => setFilter('all')}
                    >
                        All
                    </button>
                    <button
                        type="button"
                        className={`filter-btn online ${filter === 'online' ? 'active' : ''}`}
                        onClick={() => setFilter('online')}
                    >
                        <Icon name="wifi" size={14} /> Online
                    </button>
                    <button
                        type="button"
                        className={`filter-btn offline ${filter === 'offline' ? 'active' : ''}`}
                        onClick={() => setFilter('offline')}
                    >
                        <Icon name="wifiOff" size={14} /> Offline
                    </button>
                </div>
            </div>

            {error && (
                <div className="error-banner">
                    <span>{error}</span>
                    <button type="button" onClick={loadDevices}>
                        Retry
                    </button>
                </div>
            )}

            <div className="devices-table-container">
                <table className="devices-table">
                    <thead>
                        <tr>
                            <th>Status</th>
                            <th>Device Name</th>
                            <th>Android</th>
                            <th>Owner</th>
                            <th>Battery</th>
                            <th>Last Check-in</th>
                            <th>Interfaces</th>
                            <th>Actions</th>
                        </tr>
                    </thead>
                    <tbody>
                        {filteredDevices.length === 0 ? (
                            <tr>
                                <td colSpan="8" className="empty-row">
                                    No devices found
                                </td>
                            </tr>
                        ) : (
                            filteredDevices.map((device) => (
                                <tr key={device.id} className={`device-row ${device.status}`}>
                                    <td className="status-col">
                                        <span className={`status-badge ${device.status}`}>
                                            <span className="status-dot"></span>
                                            {device.status}
                                        </span>
                                    </td>
                                    <td className="name-col">
                                        <button
                                            type="button"
                                            className="device-name-btn"
                                            onClick={() => setModalDevice(device)}
                                            title="View interfaces"
                                        >
                                            <span className="device-icon">
                                                <Icon name="devices" size={20} />
                                            </span>
                                            <span className="device-name">{device.friendly_name}</span>
                                        </button>
                                    </td>
                                    <td className="os-col" title={device.hardware_model || ''}>
                                        {formatAndroidVersion(device.os_version)}
                                    </td>
                                    <td className="owner-col">{device.owner || '-'}</td>
                                    <td className="battery-col">
                                        <div className="battery-display">
                                            <span className="battery-value">{device.battery_level}%</span>
                                            <div className="battery-bar">
                                                <div
                                                    className="battery-fill"
                                                    style={{
                                                        width: `${device.battery_level}%`,
                                                        backgroundColor: getBatteryColor(device.battery_level),
                                                    }}
                                                ></div>
                                            </div>
                                        </div>
                                    </td>
                                    <td className="last-seen-col">
                                        {device.last_check_in ? (
                                            <span className="last-seen">{formatLastSeen(device.last_check_in)}</span>
                                        ) : (
                                            '-'
                                        )}
                                    </td>
                                    <td className="interfaces-col">
                                        <div className="interface-chips">
                                            {DEVICE_INTERFACE_CHIPS.map((iface) => (
                                                <button
                                                    key={iface.key}
                                                    type="button"
                                                    className="interface-chip"
                                                    title={iface.label}
                                                    onClick={() => goInterface(device.id, iface)}
                                                >
                                                    <Icon name={iface.icon} size={14} />
                                                    <span>{iface.shortLabel}</span>
                                                </button>
                                            ))}
                                        </div>
                                    </td>
                                    <td className="actions-col">
                                        <button
                                            type="button"
                                            className="action-btn view"
                                            onClick={() => setModalDevice(device)}
                                            title="All interfaces"
                                        >
                                            <Icon name="list" size={16} />
                                        </button>
                                        <button
                                            type="button"
                                            className="action-btn view"
                                            onClick={() => navigate(`/devices/${device.id}`)}
                                            title="View Details"
                                        >
                                            <Icon name="eye" size={16} />
                                        </button>
                                    </td>
                                </tr>
                            ))
                        )}
                    </tbody>
                </table>
            </div>

            {modalDevice && (
                <div
                    className="device-iface-modal-backdrop"
                    role="presentation"
                    onClick={() => setModalDevice(null)}
                >
                    <div
                        className="device-iface-modal"
                        role="dialog"
                        aria-modal="true"
                        aria-labelledby="device-iface-modal-title"
                        onClick={(e) => e.stopPropagation()}
                    >
                        <header className="device-iface-modal-header">
                            <div>
                                <h2 id="device-iface-modal-title">{modalDevice.friendly_name}</h2>
                                <p className="device-iface-modal-meta">
                                    <span className={`status-badge ${modalDevice.status}`}>
                                        <span className="status-dot"></span>
                                        {modalDevice.status}
                                    </span>
                                    {modalDevice.owner ? (
                                        <span className="device-iface-owner">{modalDevice.owner}</span>
                                    ) : null}
                                </p>
                            </div>
                            <button
                                type="button"
                                className="device-iface-close"
                                onClick={() => setModalDevice(null)}
                                title="Close"
                            >
                                <Icon name="close" size={18} />
                            </button>
                        </header>
                        <p className="device-iface-modal-hint">
                            Choose an interface. The device will be selected; use Fetch or Sync on that page.
                        </p>
                        <ul className="device-iface-list">
                            {DEVICE_INTERFACES.map((iface) => (
                                <li key={iface.key}>
                                    <button
                                        type="button"
                                        className="device-iface-item"
                                        onClick={() => goInterface(modalDevice.id, iface)}
                                    >
                                        <span className="device-iface-item-icon">
                                            <Icon name={iface.icon} size={20} />
                                        </span>
                                        <span className="device-iface-item-label">{iface.label}</span>
                                        <Icon name="chevronRight" size={16} className="device-iface-item-chevron" />
                                    </button>
                                </li>
                            ))}
                        </ul>
                    </div>
                </div>
            )}
        </div>
    );
}

function getBatteryColor(level) {
    if (level >= 60) return '#4CAF50';
    if (level >= 30) return '#FFC107';
    return '#F44336';
}

function formatLastSeen(timestamp) {
    const date = new Date(timestamp);
    const now = new Date();
    const diffMs = now - date;
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;
    return date.toLocaleDateString();
}
