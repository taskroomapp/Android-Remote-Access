export function attachRecordsApi(proto) {
    proto.getCallLogs = async function getCallLogs(deviceId) {
        return this.request(`/calls/${deviceId}`);
    };
}
