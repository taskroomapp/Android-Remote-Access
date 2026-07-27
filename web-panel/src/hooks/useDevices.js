import { useMemo } from 'react';
import { useDevices as useAppDevices } from '../context/DeviceContext';

export function useDevices({ onlineOnly = false, storageAccessOnly = false } = {}) {
    const { devices, onlineDevices, loading, loadDevices } = useAppDevices();

    const filtered = useMemo(() => {
        let list = devices;
        if (onlineOnly) {
            list = onlineDevices;
        }
        if (storageAccessOnly) {
            list = list.filter(
                (d) => d.storage_access !== false && d.permissions?.storage !== false
            );
        }
        return list;
    }, [devices, onlineDevices, onlineOnly, storageAccessOnly]);

    return { devices: filtered, loading, error: null, reload: loadDevices, setDevices: () => {} };
}

export function deviceLabel(device) {
    if (!device) return '';
    const name = device.friendly_name || device.device_name || device.id?.slice(0, 8);
    const online = device.status === 'online' || device.online;
    return `${name}${online ? '' : ' (offline)'}`;
}

export function isDeviceOnline(device) {
    return device?.status === 'online' || device?.online === true;
}
