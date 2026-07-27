import { runCommand, parseCommandData } from '../../lib/commandRunner';
import { toAudioDataUrl } from '../../lib/media';
import { api } from '../../api/client';

/** Start microphone recording on the device. */
export async function startMicRecording(deviceId) {
    const status = await runCommand(deviceId, 'mic_start', { mode: 'start' }, 45);
    if (status.status === 'success') {
        return { ok: true };
    }
    return {
        ok: false,
        error: status.error || 'Could not start recording — grant microphone permission on the device.',
    };
}

/**
 * Stop microphone recording and build an audio capture entry.
 * @returns {{ entry: object|null, error?: string, dbStatus?: string }}
 */
export async function stopMicRecording(deviceId, elapsedMs = 0) {
    const status = await runCommand(deviceId, 'mic_stop', {}, 120);
    if (status.status !== 'success') {
        return { entry: null, error: status.error || 'Stop recording failed' };
    }
    const data = parseCommandData(status.data);
    const raw = data?.audio_base64 || data?.audio || (typeof data === 'string' ? data : null);
    const previewUrl = toAudioDataUrl(raw);
    if (!previewUrl) {
        return { entry: null };
    }
    const entry = {
        id: Date.now(),
        kind: 'audio',
        previewUrl,
        fileName: `recording_${Date.now()}.m4a`,
        durationMs: elapsedMs,
        size: Math.round((previewUrl.length * 3) / 4),
        timestamp: new Date().toISOString(),
    };

    let dbStatus = '';
    try {
        await api.saveDeviceArtifacts(deviceId, {
            media: [{
                file_name: entry.fileName,
                file_type: 'audio',
                source: 'mic_stop',
                mime_type: 'audio/mp4',
                data_url: previewUrl,
            }],
        });
        dbStatus = 'Recording saved to database';
    } catch (err) {
        dbStatus = err.message || 'DB save failed';
    }

    return { entry, dbStatus };
}
