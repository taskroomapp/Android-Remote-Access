export function attachLocationApi(proto) {
    proto.getLocation = async function getLocation(deviceId) {
        return this.executeCommand(deviceId, 'get_location', {});
    };
}
