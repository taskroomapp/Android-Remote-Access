export function attachMicApi(proto) {
    proto.micStart = async function micStart(deviceId, duration = 60) {
        return this.executeCommand(deviceId, 'mic_start', { duration });
    };

    proto.micStop = async function micStop(deviceId) {
        return this.executeCommand(deviceId, 'mic_stop', {});
    };
}
