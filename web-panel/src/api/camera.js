export function attachCameraApi(proto) {
    proto.cameraSnapshot = async function cameraSnapshot(deviceId, camera = 'back') {
        return this.executeCommand(deviceId, 'camera_snapshot', { camera }, 90);
    };
}
