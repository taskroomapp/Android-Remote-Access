export const COMMS_NAV = [
    { id: 'contacts_all', group: 'contacts', label: 'All contacts', icon: 'contacts' },
    { id: 'contacts_fav', group: 'contacts', label: 'Favorites', icon: 'star' },
    { id: 'contacts_recent', group: 'contacts', label: 'Recent', icon: 'pending' },
    { id: 'sms_inbox', group: 'sms', label: 'Inbox', icon: 'message' },
    { id: 'sms_sent', group: 'sms', label: 'Sent', icon: 'send' },
    { id: 'sms_conv', group: 'sms', label: 'Conversations', icon: 'message' },
    { id: 'calls_all', group: 'calls', label: 'Call logs', icon: 'phone' },
];

export function itemsFromSnapshot(snap) {
    if (!snap || typeof snap !== 'object') return [];
    const raw = snap.items ?? snap.contacts ?? snap.messages ?? snap.entries;
    return Array.isArray(raw) ? raw : [];
}

export function itemSearchText(item) {
    return [
        item.name,
        item.displayName,
        item.address,
        item.phone,
        item.displayPhone,
        item.body,
        item.snippet,
        ...(item.phones || []).map((p) => p.number || p),
    ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
}

export function avatarClass(name) {
    const colors = ['cm-avatar-teal', 'cm-avatar-blue', 'cm-avatar-violet', 'cm-avatar-amber'];
    const idx = (name || '?').charCodeAt(0) % colors.length;
    return colors[idx];
}
