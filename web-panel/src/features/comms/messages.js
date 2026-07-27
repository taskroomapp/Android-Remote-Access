import { formatPhoneDisplay, normalizePhone, resolveContact } from './phones.js';

export function messageDateToMillis(v) {
    if (v == null || v === '') return 0;
    if (typeof v === 'number') return v < 1e11 ? v * 1000 : v;
    const t = Date.parse(v);
    return Number.isNaN(t) ? 0 : t;
}

export function smsTypeToNumber(type) {
    const map = { inbox: 1, sent: 2, draft: 3, outbox: 4, failed: 5, queued: 6 };
    if (typeof type === 'number') return type;
    const s = String(type || '').toLowerCase();
    if (map[s] != null) return map[s];
    const n = Number(type);
    return Number.isFinite(n) ? n : 1;
}

export function storedSmsToUi(rows) {
    return (rows || []).map((r) => ({
        id: r.native_id || r.id,
        read: !!r.is_read,
        address: r.address || '',
        body: r.message || r.body || '',
        name: r.name || '',
        person: r.person || '',
        date: messageDateToMillis(r.message_date || r.date),
        type: smsTypeToNumber(r.message_type || r.type),
        data_entry_date: r.data_entry_date,
    }));
}

export function formatMsgTime(ts) {
    if (ts == null || ts === '') return '';
    const n = Number(ts);
    if (!Number.isFinite(n) || n < 1e11) return String(ts);
    const d = new Date(n);
    if (Number.isNaN(d.getTime())) return String(ts);
    const now = new Date();
    const sameDay = d.toDateString() === now.toDateString();
    if (sameDay) return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    const sameYear = d.getFullYear() === now.getFullYear();
    return d.toLocaleDateString([], sameYear
        ? { month: 'short', day: 'numeric' }
        : { year: 'numeric', month: 'short', day: 'numeric' });
}

/** One row per phone number — latest snippet + total count (no duplicate names). */
export function buildConversations(messages, contactsByPhone) {
    const map = new Map();
    for (const msg of messages || []) {
        const address = msg.address || msg.phone;
        if (!address) continue;
        const key = normalizePhone(address) || String(address);
        const date = Number(msg.date || msg.timestamp || 0) || 0;
        const prev = map.get(key);
        if (!prev) {
            const contact = resolveContact(contactsByPhone, address);
            map.set(key, {
                id: `conv-${key}`,
                address,
                name: contact?.name || '',
                contactId: contact?.id || null,
                body: msg.body,
                date,
                snippet: msg.body,
                count: 1,
                kind: 'conversation',
            });
            continue;
        }
        prev.count += 1;
        if (date >= (prev.date || 0)) {
            prev.date = date;
            prev.body = msg.body;
            prev.snippet = msg.body;
            prev.address = address;
        }
    }
    return Array.from(map.values()).sort((a, b) => (b.date || 0) - (a.date || 0));
}

export function buildInboxConversations(inbox, contactsByPhone) {
    return buildConversations(inbox, contactsByPhone);
}

export function buildSentConversations(sent, contactsByPhone) {
    return buildConversations(sent, contactsByPhone);
}

export function buildAllConversations(inbox, sent, contactsByPhone) {
    return buildConversations([...(inbox || []), ...(sent || [])], contactsByPhone);
}

export function enrichSmsItem(item, contactsByPhone) {
    const address = item.address || item.phone || '';
    const contact = resolveContact(contactsByPhone, address);
    const displayPhone = formatPhoneDisplay(address);
    const displayName = contact?.name || item.name || '';
    return {
        ...item,
        address,
        contact,
        contactId: contact?.id || item.contactId || null,
        displayName,
        displayPhone,
        title: displayName || displayPhone || 'Unknown',
        subtitle: displayName ? displayPhone : '',
        timeLabel: formatMsgTime(item.date || item.timestamp),
        kind: item.kind || 'sms',
    };
}
