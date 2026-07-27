import { formatPhoneDisplay, resolveContact } from './phones.js';
import { formatMsgTime, messageDateToMillis } from './messages.js';

export function storedCallsToUi(rows) {
    return (rows || []).map((r) => ({
        id: r.call_id || r.id,
        number: r.number || '',
        name: r.name_call || r.name || '',
        duration: r.duration || 0,
        type: r.type_call || r.type || 'unknown',
        timestamp: messageDateToMillis(r.date_call || r.timestamp),
        contact_id: r.id_contacts || '',
        data_entry_date: r.data_entry_date,
    }));
}

export function formatDurationSec(sec) {
    const n = Number(sec) || 0;
    const m = Math.floor(n / 60);
    const s = n % 60;
    return m > 0 ? `${m}m ${s}s` : `${s}s`;
}

export function enrichCallItem(item, contactsByPhone) {
    const contact = resolveContact(contactsByPhone, item.number);
    const displayName = item.name || contact?.name || '';
    const displayPhone = formatPhoneDisplay(item.number);
    return {
        ...item,
        contact,
        contactId: contact?.id || item.contact_id || null,
        displayName,
        displayPhone,
        title: displayName || displayPhone || 'Unknown',
        subtitle: displayName ? `${displayPhone} · ${item.type || ''}` : (item.type || ''),
        timeLabel: formatMsgTime(item.timestamp || item.date),
        body: item.duration != null ? `${item.duration}s` : '',
        kind: 'call',
    };
}
