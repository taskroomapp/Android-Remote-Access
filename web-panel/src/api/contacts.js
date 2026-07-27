export function attachContactsApi(proto) {
    proto.getContacts = async function getContacts(deviceId) {
        return this.request(`/contacts/${deviceId}`);
    };
}
