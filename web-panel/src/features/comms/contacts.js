import { formatPhoneDisplay, primaryPhone } from './phones.js';

export function storedContactsToUi(rows) {
    return (rows || []).map((r) => ({
        id: r.native_id || r.id,
        name: r.display_name || r.name || '',
        phones: r.number ? [{ number: r.number }] : [],
        data_entry_date: r.data_entry_date,
    }));
}

export function contactPrefsKey(deviceId) {
    return `cm_prefs_${deviceId}`;
}

export function loadContactPrefs(deviceId) {
    if (!deviceId) return { favorites: [], recent: [] };
    try {
        const raw = JSON.parse(localStorage.getItem(contactPrefsKey(deviceId)) || '{}');
        return {
            favorites: Array.isArray(raw.favorites) ? raw.favorites : [],
            recent: Array.isArray(raw.recent) ? raw.recent : [],
        };
    } catch {
        return { favorites: [], recent: [] };
    }
}

export function touchRecentContact(deviceId, contactId) {
    if (!deviceId || !contactId) return;
    const prefs = loadContactPrefs(deviceId);
    const recent = [contactId, ...prefs.recent.filter((id) => id !== contactId)].slice(0, 40);
    localStorage.setItem(contactPrefsKey(deviceId), JSON.stringify({ ...prefs, recent }));
}

export function enrichContactItem(item) {
    const phone = primaryPhone(item);
    return {
        ...item,
        displayName: item.name || 'Unknown',
        displayPhone: formatPhoneDisplay(phone),
        title: item.name || formatPhoneDisplay(phone) || 'Unknown',
        subtitle: formatPhoneDisplay(phone),
        timeLabel: '',
        kind: 'contact',
    };
}
