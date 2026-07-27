export function attachArtifactsApi(proto) {
    proto.saveDeviceArtifacts = async function saveDeviceArtifacts(deviceId, { locations, files, media } = {}) {
        return this.request(`/devices/${deviceId}/artifacts/save`, {
            method: 'POST',
            body: JSON.stringify({
                locations: locations || [],
                files: files || [],
                media: media || [],
            }),
            timeoutMs: 120000,
        });
    };

    proto.listDeviceArtifacts = async function listDeviceArtifacts(deviceId, type = 'all', limit = 5000) {
        const q = new URLSearchParams({ type, limit: String(limit) });
        return this.request(`/devices/${deviceId}/artifacts?${q}`);
    };

    proto.exportDeviceArtifacts = async function exportDeviceArtifacts(deviceId, type = 'all') {
        const q = new URLSearchParams({ type });
        return this.request(`/devices/${deviceId}/artifacts/export?${q}`, {
            responseType: 'blob',
            timeoutMs: 120000,
        });
    };
}
