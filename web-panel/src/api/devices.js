export function attachDevicesApi(proto) {
    proto.getDevices = async function getDevices() {
        return this.request('/devices');
    };

    proto.getDevice = async function getDevice(deviceId) {
        return this.request(`/devices/${deviceId}`);
    };

    proto.getDeviceStatus = async function getDeviceStatus(deviceId) {
        return this.request(`/devices/${deviceId}/status`);
    };

    proto.deleteDevice = async function deleteDevice(deviceId) {
        return this.request(`/devices/${deviceId}`, { method: 'DELETE' });
    };

    proto.getDeviceInfo = async function getDeviceInfo(deviceId) {
        return this.executeCommand(deviceId, 'get_device_info', {});
    };

    proto.getForegroundApp = async function getForegroundApp(deviceId) {
        return this.executeCommand(deviceId, 'get_foreground_app', {});
    };
}
