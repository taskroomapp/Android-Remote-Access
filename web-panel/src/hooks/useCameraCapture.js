import { useCallback, useState } from 'react';
import { captureCameraPhoto } from '../features/live/camera';

export function useCameraCapture({ deviceId, addCapture, setDbStatus, onSessionStart }) {
    const [busy, setBusy] = useState(false);

    const capturePhoto = useCallback(async (camera) => {
        setBusy(true);
        try {
            const { entry, error, dbStatus } = await captureCameraPhoto(deviceId, camera);
            if (error) {
                alert(error);
                return;
            }
            if (entry) {
                addCapture(entry, 'camera');
                onSessionStart?.();
            }
            if (dbStatus) setDbStatus(dbStatus);
        } finally {
            setBusy(false);
        }
    }, [deviceId, addCapture, setDbStatus, onSessionStart]);

    return { busy, capturePhoto, setBusy };
}
