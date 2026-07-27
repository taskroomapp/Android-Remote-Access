import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../api/client';
import { useAuth } from '../App';
import Icon from '../components/ui/Icon';
import { formatAndroidVersion } from '../lib/androidVersion';

export default function Dashboard() {
    const navigate = useNavigate();
    const { user } = useAuth();
    const [stats, setStats] = useState(null);
    const [devices, setDevices] = useState([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState(null);

    useEffect(() => {
        loadDashboardData();
    }, []);

    const loadDashboardData = async () => {
        try {
            setLoading(true);
            const [statsData, devicesData] = await Promise.all([
                api.getDashboardStats(),
                api.getDevices(),
            ]);
            setStats(statsData);
            setDevices(devicesData.devices || []);
        } catch (err) {
            setError(err.message);
        } finally {
            setLoading(false);
        }
    };

    if (loading) {
        return (
            <div className="dashboard-loading">
                <div className="spinner"></div>
                <p>Loading dashboard...</p>
            </div>
        );
    }

    if (error) {
        return (
            <div className="dashboard-error">
                <h2>Error Loading Dashboard</h2>
                <p>{error}</p>
                <button onClick={loadDashboardData}>Retry</button>
            </div>
        );
    }

    return (
        <div className="dashboard">
            <header className="dashboard-header">
                <div className="welcome">
                    <h1>Welcome back, {user?.username}</h1>
                    <p className="subtitle">Android Remote Access Control Panel</p>
                </div>
                <div className="quick-actions">
                    <button className="btn-primary" onClick={() => navigate('/orders')}>
                        <Icon name="orders" size={18} />
                        Orders
                    </button>
                    <button className="btn-secondary" onClick={() => navigate('/files')}>
                        <Icon name="files" size={18} />
                        Files
                    </button>
                </div>
            </header>

            {/* Stats Cards */}
            <section className="stats-grid">
                <div className="stat-card devices-total" onClick={() => navigate('/devices')}>
                    <div className="stat-icon"><Icon name="devices" size={28} /></div>
                    <div className="stat-content">
                        <span className="stat-value">{stats?.total_devices || 0}</span>
                        <span className="stat-label">Total Devices</span>
                    </div>
                </div>

                <div className="stat-card devices-online">
                    <div className="stat-icon"><Icon name="wifi" size={28} /></div>
                    <div className="stat-content">
                        <span className="stat-value">{stats?.online_devices || 0}</span>
                        <span className="stat-label">Online</span>
                    </div>
                </div>

                <div className="stat-card devices-offline">
                    <div className="stat-icon"><Icon name="wifiOff" size={28} /></div>
                    <div className="stat-content">
                        <span className="stat-value">{stats?.offline_devices || 0}</span>
                        <span className="stat-label">Offline</span>
                    </div>
                </div>

                <div className="stat-card commands-today">
                    <div className="stat-icon"><Icon name="dashboard" size={28} /></div>
                    <div className="stat-content">
                        <span className="stat-value">{stats?.commands_today || 0}</span>
                        <span className="stat-label">Commands Today</span>
                    </div>
                </div>
            </section>

            {/* Main Content Grid */}
            <div className="dashboard-grid">
                {/* Recent Activity */}
                <section className="dashboard-section recent-activity">
                    <div className="section-header">
                        <h2>Recent Activity</h2>
                        <a href="/audit" className="view-all">View All</a>
                    </div>
                    <div className="activity-list">
                        {stats?.recent_alerts?.length > 0 ? (
                            stats.recent_alerts.map((alert, index) => (
                                <div key={index} className={`activity-item ${alert.severity}`}>
                                    <span className="activity-icon">
                                        <Icon name={alert.severity === 'critical' ? 'warning' : 'pending'} size={18} />
                                    </span>
                                    <div className="activity-content">
                                        <p>{alert.message}</p>
                                        <span className="activity-time">
                                            {new Date(alert.timestamp).toLocaleString()}
                                        </span>
                                    </div>
                                </div>
                            ))
                        ) : (
                            <div className="empty-state">
                                <p>No recent activity</p>
                            </div>
                        )}
                    </div>
                </section>

                {/* Top Commands */}
                <section className="dashboard-section top-commands">
                    <div className="section-header">
                        <h2>Command Usage</h2>
                    </div>
                    <div className="command-stats">
                        {stats?.top_commands && Object.entries(stats.top_commands).length > 0 ? (
                            Object.entries(stats.top_commands)
                                .sort(([, a], [, b]) => b - a)
                                .slice(0, 5)
                                .map(([command, count]) => (
                                    <div key={command} className="command-stat">
                                        <span className="command-name">{formatCommandName(command)}</span>
                                        <div className="command-bar-container">
                                            <div
                                                className="command-bar"
                                                style={{
                                                    width: `${Math.min((count / (stats.commands_today || 1)) * 100, 100)}%`
                                                }}
                                            ></div>
                                        </div>
                                        <span className="command-count">{count}</span>
                                    </div>
                                ))
                        ) : (
                            <div className="empty-state">
                                <p>No commands executed yet</p>
                            </div>
                        )}
                    </div>
                </section>

                {/* Online Devices */}
                <section className="dashboard-section online-devices">
                    <div className="section-header">
                        <h2>Online Devices</h2>
                        <a href="/devices" className="view-all">View All</a>
                    </div>
                    <div className="device-list">
                        {devices.filter(d => d.status === 'online').length > 0 ? (
                            devices
                                .filter(d => d.status === 'online')
                                .slice(0, 5)
                                .map(device => (
                                    <div
                                        key={device.id}
                                        className="device-item online"
                                        onClick={() => navigate(`/devices/${device.id}`)}
                                    >
                                        <div className="device-status-indicator"></div>
                                        <div className="device-info">
                                            <span className="device-name">{device.friendly_name}</span>
                                            <span className="device-owner">
                                                {formatAndroidVersion(device.os_version)}
                                                {device.owner ? ` · ${device.owner}` : ''}
                                            </span>
                                        </div>
                                        <div className="device-battery">
                                            <span className="battery-level">{device.battery_level}%</span>
                                            <Icon name={getBatteryIconName(device.battery_level)} size={16} />
                                        </div>
                                    </div>
                                ))
                        ) : (
                            <div className="empty-state">
                                <p>No devices online</p>
                            </div>
                        )}
                    </div>
                </section>

                {/* System Status */}
                <section className="dashboard-section system-status">
                    <div className="section-header">
                        <h2>System Status</h2>
                    </div>
                    <div className="status-items">
                        <div className="status-item">
                            <span className="status-label">Success Rate</span>
                            <span className="status-value success-rate">
                                {(stats?.success_rate || 0).toFixed(1)}%
                            </span>
                        </div>
                        <div className="status-item">
                            <span className="status-label">Total Commands</span>
                            <span className="status-value">{stats?.total_commands || 0}</span>
                        </div>
                        <div className="status-item">
                            <span className="status-label">This Week</span>
                            <span className="status-value">{stats?.commands_this_week || 0}</span>
                        </div>
                        <div className="status-item">
                            <span className="status-label">Active Admins</span>
                            <span className="status-value">{stats?.active_admins || 0}</span>
                        </div>
                    </div>
                </section>
            </div>
        </div>
    );
}

function formatCommandName(cmd) {
    return cmd
        .replace(/_/g, ' ')
        .replace(/\b\w/g, c => c.toUpperCase());
}

function getBatteryIconName(level) {
    if (level >= 75) return 'batteryFull';
    if (level >= 40) return 'batteryMedium';
    if (level >= 15) return 'batteryLow';
    return 'batteryLow';
}
