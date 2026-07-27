import { runCommand, parseCommandData } from '../../lib/commandRunner';
import { looksLikeImagePayload, toImageDataUrl } from '../../lib/media';
import { api } from '../../api/client';

/**
 * Capture a still photo from the device camera and optionally persist to artifacts DB.
 * @returns {{ entry: object|null, error?: string, dbStatus?: string }}
 */
export async function captureCameraPhoto(deviceId, camera) {
    let status;
    try {
        // Long timeout: modern cameras need AE settle + large CKX1 photo round-trip.
        status = await runCommand(deviceId, 'camera_snapshot', { camera }, 90);
    } catch (err) {
        const msg = err?.message || String(err);
        if (/CKX1|decrypt/i.test(msg)) {
            return {
                entry: null,
                error:
                    `Decrypt failed while receiving the photo (${msg}). ` +
                    'Refresh the page to renew the CKX1 session, then try again.',
            };
        }
        return {
            entry: null,
            error: msg || 'Capture request failed',
        };
    }

    if (status.status !== 'success') {
        const detail = status.error || status.message || status.error_code || status.status;
        return {
            entry: null,
            error: detail
                ? `Capture failed: ${detail}`
                : 'Capture failed — ensure the agent is connected and CAMERA permission is granted.',
        };
    }

    const data = parseCommandData(status.data);
    const candidates = [
        status.data,
        data,
        typeof data === 'string' ? data : null,
        data?.image,
        data?.base64,
        data?.jpeg,
        data?.content,
    ];
    let previewUrl = null;
    for (const candidate of candidates) {
        if (candidate == null) continue;
        if (!looksLikeImagePayload(candidate) && typeof candidate === 'string' && candidate.length < 64) {
            continue;
        }
        previewUrl = toImageDataUrl(candidate);
        if (previewUrl) break;
    }

    if (!previewUrl) {
        return {
            entry: null,
            error:
                'Photo arrived but could not be decoded as JPEG. ' +
                'If this keeps happening, reconnect the agent and capture again.',
        };
    }

    const entry = {
        id: Date.now(),
        kind: 'camera',
        camera,
        previewUrl,
        fileName: `capture_${camera}_${Date.now()}.jpg`,
        size: Math.round((previewUrl.length * 3) / 4),
        timestamp: new Date().toISOString(),
    };

    let dbStatus = '';
    try {
        await api.saveDeviceArtifacts(deviceId, {
            media: [{
                file_name: entry.fileName,
                file_type: 'image',
                source: 'camera_snapshot',
                camera,
                data_url: previewUrl,
            }],
        });
        dbStatus = 'Photo saved to database';
    } catch (err) {
        dbStatus = err.message || 'DB save failed';
    }

    return { entry, dbStatus };
}
