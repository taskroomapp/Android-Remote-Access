import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { api } from '../api/client';
import Icon from '../components/ui/Icon';
import { formatAndroidVersion } from '../lib/androidVersion';

export default function DeviceDetail() {
    const { id } = useParams();
    const navigate = useNavigate();
    const [device, setDevice] = useState(null);
    const [deviceStatus, setDeviceStatus] = useState(null);
    const [deviceInfo, setDeviceInfo] = useState(null);
    const [contacts, setContacts] = useState([]);
    const [callLogs, setCallLogs] = useState([]);
    const [loading, setLoading] = useState(true);
    const [activeTab, setActiveTab] = useState('overview');
    const [loadingData, setLoadingData] = useState({});

    useEffect(() => {
        loadDeviceData();
    }, [id]);

    const loadDeviceData = async () => {
        try {
            setLoading(true);
            const [deviceData, statusData] = await Promise.all([
                api.getDevice(id),
                api.getDeviceStatus(id)
            ]);
            setDevice(deviceData);
            setDeviceStatus(statusData);
        } catch (err) {
            console.error('Failed to load device:', err);
        } finally {
            setLoading(false);
        }
    };

    const parsePayload = (raw) => {
        if (raw == null) return null;
        if (typeof raw === 'object') return raw;
        try {
            return JSON.parse(raw);
        } catch {
            return raw;
        }
    };

    const fetchDeviceInfo = async () => {
        setLoadingData(prev => ({ ...prev, deviceInfo: true }));
        try {
            const result = await api.getDeviceInfo(id);
            if (result.status === 'success' && result.data != null) {
                setDeviceInfo(parsePayload(result.data));
            }
        } catch (err) {
            console.error('Failed to fetch device info:', err);
        } finally {
            setLoadingData(prev => ({ ...prev, deviceInfo: false }));
        }
    };

    const fetchContacts = async () => {
        setLoadingData(prev => ({ ...prev, contacts: true }));
        try {
            const result = await api.getContacts(id);
            if (result.status === 'success' && result.data != null) {
                const data = parsePayload(result.data);
                const list = data.contacts || data || [];
                setContacts(list);
                try {
                    await api.saveDeviceComms(id, { contacts: Array.isArray(list) ? list : [] });
                } catch {
                    /* optional persistence */
                }
            }
        } catch (err) {
            console.error('Failed to fetch contacts:', err);
        } finally {
            setLoadingData(prev => ({ ...prev, contacts: false }));
        }
    };

    const fetchCallLogs = async () => {
        setLoadingData(prev => ({ ...prev, callLogs: true }));
        try {
            const result = await api.getCallLogs(id);
            if (result.status === 'success' && result.data != null) {
                const data = parsePayload(result.data);
                const list = data.calls || data || [];
                setCallLogs(list);
                try {
                    await api.saveDeviceComms(id, { calls: Array.isArray(list) ? list : [] });
                } catch {
                    /* optional persistence */
                }
            }
        } catch (err) {
            console.error('Failed to fetch call logs:', err);
        } finally {
            setLoadingData(prev => ({ ...prev, callLogs: false }));
        }
    };

    const exportCommsExcel = async (type) => {
        try {
            const blob = await api.exportDeviceComms(id, type);
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `device-${type}-${new Date().toISOString().slice(0, 10)}.xlsx`;
            document.body.appendChild(a);
            a.click();
            a.remove();
            URL.revokeObjectURL(url);
        } catch (err) {
            console.error('Excel export failed:', err);
            alert(err.message || 'Excel export failed');
        }
    };

    if (loading) {
        return (
            <div className="device-detail-loading">
                <div className="spinner"></div>
                <p>Loading device details...</p>
            </div>
        );
    }

    if (!device) {
        return (
            <div className="device-not-found">
                <h2>Device Not Found</h2>
                <p>The requested device could not be found.</p>
                <button onClick={() => navigate('/devices')}>Back to Devices</button>
            </div>
        );
    }

    return (
        <div className="device-detail">
            <header className="page-header">
                <button className="btn-back" onClick={() => navigate('/devices')}>
                    <Icon name="chevronLeft" size={17} />
                    Back
                </button>
                <div className="header-content">
                    <div className="device-title">
                        <span className="device-icon"><Icon name="devices" size={24} /></span>
                        <div>
                            <h1>{device.friendly_name}</h1>
                            <span className={`status-badge ${deviceStatus?.online ? 'online' : 'offline'}`}>
                                {deviceStatus?.online ? 'Online' : 'Offline'}
                            </span>
                        </div>
                    </div>
                    <div className="header-actions">
                        <button
                            className="btn-primary"
                            onClick={() => navigate(`/console?device=${id}`)}
                        >
                            <Icon name="console" size={17} />
                            Send Command
                        </button>
                    </div>
                </div>
            </header>

            {/* Quick Stats */}
            <div className="device-quick-stats">
                <div className="stat">
                    <span className="stat-icon"><Icon name="battery" size={20} /></span>
                    <span className="stat-value">{device.battery_level}%</span>
                    <span className="stat-label">Battery</span>
                </div>
                <div className="stat">
                    <span className="stat-icon"><Icon name="calendar" size={20} /></span>
                    <span className="stat-value">
                        {device.last_check_in ? formatDate(device.last_check_in) : 'Never'}
                    </span>
                    <span className="stat-label">Last Check-in</span>
                </div>
                <div className="stat">
                    <span className="stat-icon"><Icon name="package" size={20} /></span>
                    <span className="stat-value">{formatAndroidVersion(device.os_version)}</span>
                    <span className="stat-label">Android Version</span>
                </div>
                <div className="stat">
                    <span className="stat-icon"><Icon name="factory" size={20} /></span>
                    <span className="stat-value">{device.hardware_model || 'Unknown'}</span>
                    <span className="stat-label">Model</span>
                </div>
            </div>

            {/* Tabs */}
            <div className="detail-tabs">
                <button
                    className={`tab ${activeTab === 'overview' ? 'active' : ''}`}
                    onClick={() => setActiveTab('overview')}
                >
                    Overview
                </button>
                <button
                    className={`tab ${activeTab === 'contacts' ? 'active' : ''}`}
                    onClick={() => {
                        setActiveTab('contacts');
                        if (contacts.length === 0) fetchContacts();
                    }}
                >
                    Contacts
                </button>
                <button
                    className={`tab ${activeTab === 'calls' ? 'active' : ''}`}
                    onClick={() => {
                        setActiveTab('calls');
                        if (callLogs.length === 0) fetchCallLogs();
                    }}
                >
                    Call Logs
                </button>
                <button
                    className={`tab ${activeTab === 'info' ? 'active' : ''}`}
                    onClick={() => {
                        setActiveTab('info');
                        if (!deviceInfo) fetchDeviceInfo();
                    }}
                >
                    Device Info
                </button>
            </div>

            {/* Tab Content */}
            <div className="tab-content">
                {activeTab === 'overview' && (
                    <div className="overview-tab">
                        <div className="info-section">
                            <h3>Device Information</h3>
                            <dl className="info-list">
                                <dt>Device UUID</dt>
                                <dd><code>{device.device_uuid}</code></dd>
                                <dt>Owner</dt>
                                <dd>{device.owner || 'Not assigned'}</dd>
                                <dt>Enrolled At</dt>
                                <dd>{device.enrolled_at ? formatDate(device.enrolled_at) : 'Unknown'}</dd>
                                <dt>Hardware Model</dt>
                                <dd>{device.hardware_model || 'Unknown'}</dd>
                                <dt>Android Version</dt>
                                <dd>{formatAndroidVersion(device.os_version)}</dd>
                            </dl>
                        </div>
                    </div>
                )}

                {activeTab === 'contacts' && (
                    <div className="contacts-tab">
                        <div className="tab-actions" style={{ marginBottom: '0.75rem', display: 'flex', gap: '0.5rem' }}>
                            <button type="button" className="btn btn-secondary" onClick={fetchContacts}>Refresh</button>
                            <button type="button" className="btn btn-secondary" onClick={() => exportCommsExcel('contacts')}>Export Excel</button>
                        </div>
                        {loadingData.contacts ? (
                            <div className="tab-loading">
                                <span className="spinner-small"></span> Loading contacts...
                            </div>
                        ) : contacts.length > 0 ? (
                            <table className="data-table">
                                <thead>
                                    <tr>
                                        <th>Name</th>
                                        <th>Phone Numbers</th>
                                        <th>Emails</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {contacts.map((contact, idx) => (
                                        <tr key={idx}>
                                            <td>{contact.name}</td>
                                            <td>
                                                {contact.phones?.map((p, i) => (
                                                    <span key={i} className="phone-number">{p.number}</span>
                                                ))}
                                            </td>
                                            <td>
                                                {contact.emails?.map((e, i) => (
                                                    <span key={i} className="email">{e.address}</span>
                                                ))}
                                            </td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        ) : (
                            <div className="empty-state">No contacts found</div>
                        )}
                    </div>
                )}

                {activeTab === 'calls' && (
                    <div className="calls-tab">
                        <div className="tab-actions" style={{ marginBottom: '0.75rem', display: 'flex', gap: '0.5rem' }}>
                            <button type="button" className="btn btn-secondary" onClick={fetchCallLogs}>Refresh</button>
                            <button type="button" className="btn btn-secondary" onClick={() => exportCommsExcel('calls')}>Export Excel</button>
                        </div>
                        {loadingData.callLogs ? (
                            <div className="tab-loading">
                                <span className="spinner-small"></span> Loading call logs...
                            </div>
                        ) : callLogs.length > 0 ? (
                            <table className="data-table">
                                <thead>
                                    <tr>
                                        <th>Type</th>
                                        <th>Number</th>
                                        <th>Name</th>
                                        <th>Duration</th>
                                        <th>Date/Time</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {callLogs.map((call, idx) => (
                                        <tr key={idx}>
                                            <td>
                                                <span className={`call-type ${call.type}`}>
                                                    <Icon name={call.type === 'incoming' ? 'download' : call.type === 'outgoing' ? 'upload' : 'cancel'} size={14} />
                                                    {call.type}
                                                </span>
                                            </td>
                                            <td><code>{call.number}</code></td>
                                            <td>{call.name || '-'}</td>
                                            <td>{formatDuration(call.duration)}</td>
                                            <td>{formatDateTime(call.timestamp)}</td>
                                        </tr>
                                    ))}
                                </tbody>
                            </table>
                        ) : (
                            <div className="empty-state">No call logs found</div>
                        )}
                    </div>
                )}

                {activeTab === 'info' && (
                    <div className="info-tab">
                        {loadingData.deviceInfo ? (
                            <div className="tab-loading">
                                <span className="spinner-small"></span> Loading device info...
                            </div>
                        ) : deviceInfo ? (
                            <dl className="info-grid">
                                <dt>Device ID</dt>
                                <dd><code>{deviceInfo.device_id}</code></dd>
                                <dt>Model</dt>
                                <dd>{deviceInfo.model}</dd>
                                <dt>Manufacturer</dt>
                                <dd>{deviceInfo.manufacturer}</dd>
                                <dt>Android Version</dt>
                                <dd>{formatAndroidVersion(deviceInfo.android_version, deviceInfo.sdk_version)}</dd>
                                <dt>Build Number</dt>
                                <dd><code>{deviceInfo.build_number}</code></dd>
                                <dt>Battery Level</dt>
                                <dd>{deviceInfo.battery_level}% ({deviceInfo.battery_status})</dd>
                                <dt>Storage Total</dt>
                                <dd>{formatBytes(deviceInfo.storage_total)}</dd>
                                <dt>Storage Available</dt>
                                <dd>{formatBytes(deviceInfo.storage_available)}</dd>
                                {deviceInfo.location && (
                                    <>
                                        <dt>Latitude</dt>
                                        <dd>{deviceInfo.location.latitude}</dd>
                                        <dt>Longitude</dt>
                                        <dd>{deviceInfo.location.longitude}</dd>
                                    </>
                                )}
                            </dl>
                        ) : (
                            <div className="empty-state">No device info available</div>
                        )}
                    </div>
                )}
            </div>
        </div>
    );
}

function formatDate(dateString) {
    return new Date(dateString).toLocaleDateString();
}

function formatDateTime(timestamp) {
    return new Date(timestamp).toLocaleString();
}

function formatDuration(seconds) {
    if (!seconds) return '0s';
    const mins = Math.floor(seconds / 60);
    const secs = seconds % 60;
    return mins > 0 ? `${mins}m ${secs}s` : `${secs}s`;
}

function formatBytes(bytes) {
    if (!bytes) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    let i = 0;
    while (bytes >= 1024 && i < units.length - 1) {
        bytes /= 1024;
        i++;
    }
    return `${bytes.toFixed(2)} ${units[i]}`;
}
