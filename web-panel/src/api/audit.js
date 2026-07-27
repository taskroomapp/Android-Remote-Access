export function attachAuditApi(proto) {
    proto.searchAuditLogs = async function searchAuditLogs(params) {
        return this.request('/audit/logs', {
            method: 'POST',
            body: JSON.stringify(params),
        });
    };

    proto.getDashboardStats = async function getDashboardStats() {
        return this.request('/dashboard/stats');
    };
}
