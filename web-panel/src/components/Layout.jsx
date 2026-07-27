import React, { useState, useEffect } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useAuth } from '../App';
import { getAdminWebSocketUrl } from '../api/client';
import Icon from './ui/Icon';

export default function Layout({ children }) {
    const location = useLocation();
    const { user, logout } = useAuth();
    const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
    const [wsConnected, setWsConnected] = useState(false);

    useEffect(() => {
        const wsUrl = getAdminWebSocketUrl();
        if (!wsUrl) {
            setWsConnected(false);
            return undefined;
        }

        const ws = new WebSocket(wsUrl);

        ws.onopen = () => {
            setWsConnected(true);
        };

        ws.onclose = () => {
            setWsConnected(false);
        };

        ws.onerror = () => {
            setWsConnected(false);
        };

        return () => {
            ws.close();
        };
    }, [user]);

    const navItems = [
        { path: '/', label: 'Dashboard', icon: 'dashboard' },
        { path: '/devices', label: 'Devices', icon: 'devices' },
        { path: '/files', label: 'Files', icon: 'files' },
        { path: '/downloads', label: 'Downloads', icon: 'downloads' },
        { path: '/orders', label: 'Orders', icon: 'orders' },
        { path: '/location', label: 'Location', icon: 'location' },
        { path: '/contacts', label: 'Contacts & SMS', icon: 'contacts' },
        { path: '/live', label: 'Live View', icon: 'live' },
        { path: '/audit', label: 'Audit Logs', icon: 'audit' },
        { path: '/settings', label: 'Settings', icon: 'settings' },
    ];

    const isActive = (path) => {
        if (path === '/') return location.pathname === '/';
        return location.pathname.startsWith(path);
    };

    return (
        <div className="app-layout">
            <aside className="sidebar">
                <div className="sidebar-header">
                    <div className="logo">
                        <span className="logo-icon"><Icon name="logo" size={24} /></span>
                        <span className="logo-text">Remote Access</span>
                    </div>
                    <div className={`connection-status ${wsConnected ? 'connected' : 'disconnected'}`}>
                        <span className="status-dot"></span>
                        {wsConnected ? 'Connected' : 'Disconnected'}
                    </div>
                </div>

                <nav className="sidebar-nav">
                    {navItems.map(item => (
                        <Link
                            key={item.path}
                            to={item.path}
                            className={`nav-item ${isActive(item.path) ? 'active' : ''}`}
                        >
                            <span className="nav-icon"><Icon name={item.icon} size={18} /></span>
                            <span className="nav-label">{item.label}</span>
                        </Link>
                    ))}
                </nav>

                <div className="sidebar-footer">
                    <div className="user-info">
                        <span className="user-avatar">{user?.username?.[0]?.toUpperCase() || 'U'}</span>
                        <div className="user-details">
                            <span className="user-name">{user?.username}</span>
                            <span className="user-role">{user?.role}</span>
                        </div>
                    </div>
                    <button className="logout-btn" onClick={logout} title="Sign Out" type="button">
                        <Icon name="logout" size={18} />
                    </button>
                </div>
            </aside>

            <header className="mobile-header">
                <button className="menu-toggle" type="button" onClick={() => setMobileMenuOpen(!mobileMenuOpen)}>
                    <Icon name={mobileMenuOpen ? 'close' : 'menu'} size={22} />
                </button>
                <div className="mobile-logo">
                    <span className="logo-icon"><Icon name="logo" size={22} /></span>
                    <span>Remote Access</span>
                </div>
                <div className={`connection-status ${wsConnected ? 'connected' : 'disconnected'}`}>
                    <span className="status-dot"></span>
                </div>
            </header>

            {mobileMenuOpen && (
                <div className="mobile-nav-overlay" onClick={() => setMobileMenuOpen(false)}>
                    <nav className="mobile-nav" onClick={(e) => e.stopPropagation()}>
                        {navItems.map(item => (
                            <Link
                                key={item.path}
                                to={item.path}
                                className={`nav-item ${isActive(item.path) ? 'active' : ''}`}
                                onClick={() => setMobileMenuOpen(false)}
                            >
                                <span className="nav-icon"><Icon name={item.icon} size={18} /></span>
                                <span className="nav-label">{item.label}</span>
                            </Link>
                        ))}
                        <div className="mobile-nav-footer">
                            <button type="button" onClick={logout}>Sign Out</button>
                        </div>
                    </nav>
                </div>
            )}

            <main className="main-content">
                {children}
            </main>
        </div>
    );
}
