export function attachTransfersApi(proto) {
    proto.getServerTransfers = async function getServerTransfers(filters = {}) {
        return this.listTransfers(filters);
    };

    proto.listTransfers = async function listTransfers(params = {}) {
        const q = new URLSearchParams();
        if (params.status) q.set('status', params.status);
        if (params.device_id) q.set('device_id', params.device_id);
        if (params.limit) q.set('limit', String(params.limit));
        const qs = q.toString();
        return this.request(`/transfers${qs ? `?${qs}` : ''}`);
    };

    proto.appealTransfer = async function appealTransfer(transferId) {
        return this.request(`/transfers/${transferId}/appeal`, { method: 'POST' });
    };

    proto.purgeCompletedTransfers = async function purgeCompletedTransfers(deviceId) {
        const q = deviceId ? `?device_id=${encodeURIComponent(deviceId)}` : '';
        return this.request(`/transfers/completed${q}`, { method: 'DELETE' });
    };
}
