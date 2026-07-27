export function attachMediaApi(proto) {
    proto.getMedia = async function getMedia(deviceId) {
        return this.request(`/media/${deviceId}`);
    };

    proto.downloadMedia = async function downloadMedia(fileId) {
        return this.request(`/media/file/${fileId}`, { responseType: 'blob' });
    };
}
