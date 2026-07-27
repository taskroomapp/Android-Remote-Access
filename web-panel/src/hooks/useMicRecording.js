import { useCallback, useState } from 'react';
import { startMicRecording, stopMicRecording } from '../features/live/mic';

export function useMicRecording({ deviceId, addCapture, setDbStatus, onSessionStart, elapsed }) {
    const [recording, setRecording] = useState(false);
    const [busy, setBusy] = useState(false);

    const startAudio = useCallback(async () => {
        setBusy(true);
        try {
            const result = await startMicRecording(deviceId);
            if (result.ok) {
                setRecording(true);
                onSessionStart?.();
            } else {
                alert(result.error);
            }
        } catch (err) {
            alert(err.message || 'Start recording failed');
        } finally {
            setBusy(false);
        }
    }, [deviceId, onSessionStart]);

    const stopAudio = useCallback(async () => {
        setBusy(true);
        try {
            const { entry, error, dbStatus } = await stopMicRecording(deviceId, elapsed);
            if (error) alert(error);
            if (entry) {
                addCapture(entry, 'audio');
            }
            if (dbStatus) setDbStatus(dbStatus);
            setRecording(false);
        } catch (err) {
            setRecording(false);
            alert(err.message || 'Stop recording failed');
        } finally {
            setBusy(false);
        }
    }, [deviceId, elapsed, addCapture, setDbStatus]);

    return { recording, busy, startAudio, stopAudio };
}
