import React from 'react';
import { useAuth } from '../App';
import Icon from '../components/ui/Icon';

export default function Settings() {
    const { user, logout } = useAuth();

    return (
        <div className="settings-page">
            <header className="page-header">
                <h1><Icon name="settings" size={24} /> Settings</h1>
            </header>

            <div className="settings-content">
                <section className="settings-section">
                    <h2>Account Information</h2>
                    <div className="settings-card">
                        <div className="setting-item">
                            <span className="setting-label">Username</span>
                            <span className="setting-value">{user?.username}</span>
                        </div>
                        <div className="setting-item">
                            <span className="setting-label">Email</span>
                            <span className="setting-value">{user?.email}</span>
                        </div>
                        <div className="setting-item">
                            <span className="setting-label">Role</span>
                            <span className="setting-value role-badge">{user?.role}</span>
                        </div>
                        <div className="setting-item">
                            <span className="setting-label">Permissions</span>
                            <div className="permissions-list">
                                {user?.permissions?.map((perm, idx) => (
                                    <span key={idx} className="permission-tag">{perm}</span>
                                ))}
                            </div>
                        </div>
                    </div>
                </section>

                <section className="settings-section">
                    <h2>Session</h2>
                    <div className="settings-card">
                        <button type="button" className="btn-danger" onClick={logout}>
                            <Icon name="logout" size={16} /> Sign Out
                        </button>
                    </div>
                </section>

                <section className="settings-section">
                    <h2>About</h2>
                    <div className="settings-card">
                        <div className="setting-item">
                            <span className="setting-label">Version</span>
                            <span className="setting-value">1.0.0</span>
                        </div>
                        <div className="setting-item">
                            <span className="setting-label">Server</span>
                            <span className="setting-value">
                                {import.meta.env.VITE_API_URL || (import.meta.env.DEV ? '/api/v1 (proxied)' : 'http://localhost:8443/api/v1')}
                            </span>
                        </div>
                    </div>
                </section>
            </div>
        </div>
    );
}
