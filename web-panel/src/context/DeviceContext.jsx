import React, { createContext, useContext, useEffect, useState } from 'react';
import { api } from '../api/client';
import { subscribeAdminDeviceEvents } from '../lib/adminWebSocket';

const DeviceContext = createContext(null);

export function DeviceProvider({ children }) {
    const [devices, setDevices] = useState([]);
    const [loading, setLoading] = useState(true);

    const loadDevices = async () => {
        try {
            const data = await api.getDevices();
            setDevices(data.devices || []);
        } catch (err) {
            console.error('Failed to load devices:', err);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        if (!api.isAuthenticated()) {
            setLoading(false);
            return undefined;
        }
        loadDevices();
        const interval = setInterval(() => {
            if (api.isAuthenticated()) {
                loadDevices();
            }
        }, 15000);
        const unsubDevices = subscribeAdminDeviceEvents(() => {
            loadDevices();
        });
        return () => {
            clearInterval(interval);
            unsubDevices();
        };
    }, []);

    const onlineDevices = devices.filter((d) => d.status === 'online' || d.online);

    return (
        <DeviceContext.Provider value={{ devices, onlineDevices, loading, loadDevices }}>
            {children}
        </DeviceContext.Provider>
    );
}

export function useDevices() {
    const ctx = useContext(DeviceContext);
    if (!ctx) {
        throw new Error('useDevices must be used within DeviceProvider');
    }
    return ctx;
}
