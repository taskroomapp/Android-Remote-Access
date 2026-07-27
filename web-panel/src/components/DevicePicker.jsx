import React from 'react';
import Icon from './ui/Icon';
import { deviceLabel } from '../hooks/useDevices';

export default function DevicePicker({
    devices,
    value,
    onChange,
    loading,
    placeholder = 'Choose a device…',
    onlineOnly = false,
    disabled,
    id,
    variant = 'default',
}) {
    const list = onlineOnly
        ? devices.filter((d) => d.status === 'online' || d.online)
        : devices;

    const isHybrid = variant === 'hybrid';

    return (
        <div className={isHybrid ? 'fb-device-picker' : 'device-picker'}>
            {!isHybrid && (
                <label htmlFor={id} className="device-picker-label">
                    <Icon name="devices" size={16} />
                    Device
                </label>
            )}
            <select
                id={id}
                className={isHybrid ? 'fb-select fb-device-select' : 'device-picker-select'}
                value={value || ''}
                onChange={(e) => onChange(e.target.value)}
                disabled={disabled || loading}
                title="Select device"
            >
                <option value="">{placeholder}</option>
                {list.map((device) => (
                    <option key={device.id} value={device.id}>
                        {deviceLabel(device)}
                    </option>
                ))}
            </select>
        </div>
    );
}
