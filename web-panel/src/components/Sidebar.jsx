import React from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useAdminWebSocket } from '../hooks/useAdminWebSocket';
import Icon from './ui/Icon';

const navItems = [
    { path: '/', label: 'Dashboard', icon: 'dashboard' },
    { path: '/files', label: 'File Browser', icon: 'files' },
    { path: '/downloads', label: 'Downloads', icon: 'downloads' },
    { path: '/orders', label: 'Orders', icon: 'orders' },
    { path: '/location', label: 'Location', icon: 'location' },
    { path: '/contacts', label: 'Contacts & SMS', icon: 'contacts' },
    { path: '/live', label: 'Live View', icon: 'live' },
    { path: '/devices', label: 'Devices', icon: 'devices' },
    { path: '/audit', label: 'Audit Logs', icon: 'audit' },
];

export default function Sidebar({ user, logout, onlineCount }) {
    const location = useLocation();
    const wsConnected = useAdminWebSocket();

    const isActive = (path) =>
        path === '/' ? location.pathname === '/' : location.pathname.startsWith(path);

    return (
        <aside className="sidebar">
            <div className="sidebar-header">
                <div className="logo">
                    <span className="logo-icon"><Icon name="logo" size={24} /></span>
                    <span className="logo-text">Remote Access</span>
                </div>
                <div className={`connection-status ${wsConnected ? 'connected' : 'disconnected'}`}>
                    <span className="status-dot" />
                    <span className="status-text">{wsConnected ? 'Live' : 'Offline'}</span>
                </div>
                <div className="online-counter">
                    <span className="counter-value">{onlineCount}</span>
                    <span className="counter-label">{' devices online'}</span>
                </div>
            </div>

            <nav className="sidebar-nav">
                {navItems.map((item) => (
                    <Link key={item.path} to={item.path} className={`nav-item ${isActive(item.path) ? 'active' : ''}`}>
                        <span className="nav-icon"><Icon name={item.icon} size={18} /></span>
                        <span className="nav-label">{item.label}</span>
                    </Link>
                ))}
            </nav>

            <div className="sidebar-footer">
                <Link to="/settings" className="user-info">
                    <span className="user-avatar">{user?.username?.[0]?.toUpperCase() || 'U'}</span>
                    <div className="user-details">
                        <span className="user-name">{user?.username}</span>
                        <span className="user-role">{user?.role}</span>
                    </div>
                </Link>
                <button type="button" className="logout-btn" onClick={logout} title="Sign Out">
                    <Icon name="logout" size={18} />
                </button>
            </div>
        </aside>
    );
}
