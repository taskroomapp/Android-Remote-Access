export function attachCommsApi(proto) {
    proto.saveDeviceComms = async function saveDeviceComms(deviceId, { contacts, messages, calls } = {}) {
        return this.request(`/devices/${deviceId}/comms/save`, {
            method: 'POST',
            body: JSON.stringify({
                contacts: contacts || [],
                messages: messages || [],
                calls: calls || [],
            }),
        });
    };

    proto.listDeviceComms = async function listDeviceComms(deviceId, type = 'all', limit = 5000) {
        const q = new URLSearchParams({ type, limit: String(limit) });
        return this.request(`/devices/${deviceId}/comms?${q}`);
    };

    proto.exportDeviceComms = async function exportDeviceComms(deviceId, type = 'all') {
        const q = new URLSearchParams({ type });
        return this.request(`/devices/${deviceId}/comms/export?${q}`, {
            responseType: 'blob',
            timeoutMs: 120000,
        });
    };
}
