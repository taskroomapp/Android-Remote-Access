/** Operator interface shortcuts for Device Management hub navigation. */

export const DEVICE_INTERFACES = [
    { key: 'files', label: 'Files', shortLabel: 'Files', icon: 'files', path: (id) => `/files?device=${encodeURIComponent(id)}` },
    { key: 'downloads', label: 'Downloads', shortLabel: 'Downloads', icon: 'downloads', path: (id) => `/downloads?device=${encodeURIComponent(id)}` },
    { key: 'location', label: 'Location', shortLabel: 'Location', icon: 'location', path: (id) => `/location?device=${encodeURIComponent(id)}` },
    { key: 'contacts', label: 'Contacts & SMS', shortLabel: 'Contacts', icon: 'contacts', path: (id) => `/contacts?device=${encodeURIComponent(id)}` },
    { key: 'live', label: 'Live View', shortLabel: 'Live', icon: 'live', path: (id) => `/live?device=${encodeURIComponent(id)}` },
    { key: 'orders', label: 'Orders', shortLabel: 'Orders', icon: 'orders', path: (id) => `/orders?device=${encodeURIComponent(id)}` },
    { key: 'detail', label: 'Device detail', shortLabel: 'Detail', icon: 'eye', path: (id) => `/devices/${encodeURIComponent(id)}` },
];

/** Compact chip set shown in the devices table (excludes detail — use row/modal). */
export const DEVICE_INTERFACE_CHIPS = DEVICE_INTERFACES.filter((i) => i.key !== 'detail');
