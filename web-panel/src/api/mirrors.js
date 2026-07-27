import { ApiError } from './errors.js';

export function attachMirrorsApi(proto) {
    proto.mirrorGet = async function mirrorGet(deviceId, type) {
        return this.getMirrorSnapshot(deviceId, type);
    };

    proto.getMirrorSnapshot = async function getMirrorSnapshot(deviceId, type) {
        try {
            return await this.request(`/mirrors/${deviceId}?type=${encodeURIComponent(type)}`);
        } catch (err) {
            if (err instanceof ApiError && err.status === 404) {
                return null;
            }
            throw err;
        }
    };

    proto.mirrorUpdate = async function mirrorUpdate(deviceId, body) {
        return this.request(`/mirrors/${deviceId}/update`, {
            method: 'POST',
            body: JSON.stringify(body),
        });
    };
}
