import React, { useState, useEffect, createContext, useContext } from 'react';
import { BrowserRouter, Routes, Route, Navigate, useNavigate, useLocation } from 'react-router-dom';
import { api } from './api/client';

// Styles
import './styles/global.css';
import './styles/hybrid-theme.css';
import './styles/dashboard.css';
import './styles/devices.css';
import './styles/commands.css';
import './styles/audit.css';
import './styles/operations.css';
import './styles/hybrid-templates.css';
import './styles/suite-dashboard.css';
import './styles/layout.css';

// Pages
import Login from './pages/Login';
import Dashboard from './pages/Dashboard';
import Devices from './pages/Devices';
import DeviceDetail from './pages/DeviceDetail';
import AuditLogs from './pages/AuditLogs';
import Settings from './pages/Settings';
import FilesPage from './pages/Files';
import DownloadsPage from './pages/Downloads';
import OrdersPage from './pages/Orders';
import LocationPage from './pages/Location';
import ContactsSmsPage from './pages/ContactsSms';
import LiveViewPage from './pages/LiveView';

// Components
import Sidebar from './components/Sidebar';
import LoadingScreen from './components/LoadingScreen';
import { DeviceProvider, useDevices } from './context/DeviceContext';

// Auth Context
const AuthContext = createContext(null);

export function useAuth() {
    const context = useContext(AuthContext);
    if (!context) {
        throw new Error('useAuth must be used within AuthProvider');
    }
    return context;
}

function AuthProvider({ children }) {
    const [user, setUser] = useState(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        let cancelled = false;
        (async () => {
            if (api.isAuthenticated()) {
                const storedUser = localStorage.getItem('user');
                if (storedUser) {
                    const userData = JSON.parse(storedUser);
                    try {
                        await api.ensureCkx1(userData.id);
                        if (!cancelled) setUser(userData);
                    } catch {
                        api.clearTokens();
                        localStorage.removeItem('user');
                    }
                }
            }
            if (!cancelled) setLoading(false);
        })();
        return () => { cancelled = true; };
    }, []);

    const login = async (username, password) => {
        const response = await api.login(username, password);
        const userData = {
            id: response.admin.id,
            username: response.admin.username,
            email: response.admin.email,
            role: response.admin.role,
            permissions: response.admin.permissions,
        };
        setUser(userData);
        localStorage.setItem('user', JSON.stringify(userData));
        return userData;
    };

    const logout = async () => {
        await api.logout();
        setUser(null);
        localStorage.removeItem('user');
    };

    return (
        <AuthContext.Provider value={{ user, login, logout, loading }}>
            {children}
        </AuthContext.Provider>
    );
}

function AppLayout({ children }) {
    const { user, logout } = useAuth();
    const { onlineDevices } = useDevices();

    return (
        <div className="app-layout hybrid-layout">
            <Sidebar user={user} logout={logout} onlineCount={onlineDevices.length} />
            <main className="main-content">{children}</main>
        </div>
    );
}

// Protected Route Component
function ProtectedRoute({ children }) {
    const { user, loading } = useAuth();
    const location = useLocation();

    if (loading) {
        return <LoadingScreen />;
    }

    if (!user) {
        return <Navigate to="/login" state={{ from: location }} replace />;
    }

    return children;
}

// Main App Component
export default function App() {
    return (
        <AuthProvider>
            <BrowserRouter
                future={{
                    v7_startTransition: true,
                    v7_relativeSplatPath: true,
                }}
            >
                <Routes>
                    <Route path="/login" element={<Login />} />
                    <Route
                        path="/*"
                        element={
                            <ProtectedRoute>
                                <DeviceProvider>
                                    <AppLayout>
                                        <Routes>
                                            <Route path="/" element={<Dashboard />} />
                                            <Route path="/devices" element={<Devices />} />
                                            <Route path="/devices/:id" element={<DeviceDetail />} />
                                            <Route path="/files" element={<FilesPage />} />
                                            <Route path="/downloads" element={<DownloadsPage />} />
                                            <Route path="/orders" element={<OrdersPage />} />
                                            <Route path="/console" element={<OrdersPage />} />
                                            <Route path="/location" element={<LocationPage />} />
                                            <Route path="/contacts" element={<ContactsSmsPage />} />
                                            <Route path="/live" element={<LiveViewPage />} />
                                            <Route path="/audit" element={<AuditLogs />} />
                                            <Route path="/settings" element={<Settings />} />
                                            <Route path="*" element={<Navigate to="/" replace />} />
                                        </Routes>
                                    </AppLayout>
                                </DeviceProvider>
                            </ProtectedRoute>
                        }
                    />
                </Routes>
            </BrowserRouter>
        </AuthProvider>
    );
}
