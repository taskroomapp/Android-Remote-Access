/** Format enrolled / live Android version for the control panel. */
export function formatAndroidVersion(osVersion, sdkVersion) {
    const raw = String(osVersion || '').trim();
    if (raw) {
        if (/^android\b/i.test(raw)) return raw;
        if (sdkVersion != null && sdkVersion !== '') {
            return `Android ${raw} (API ${sdkVersion})`;
        }
        return `Android ${raw}`;
    }
    if (sdkVersion != null && sdkVersion !== '') {
        return `Android (API ${sdkVersion})`;
    }
    return 'Unknown';
}
