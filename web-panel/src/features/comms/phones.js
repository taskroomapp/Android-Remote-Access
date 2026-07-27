/** Shared phone/contact index helpers for contacts, SMS, and call logs. */

export function normalizePhone(raw) {
    if (raw == null) return '';
    const digits = String(raw).replace(/\D/g, '');
    if (!digits) return '';
    return digits.length > 10 ? digits.slice(-10) : digits;
}

export function contactPhones(contact) {
    if (!contact) return [];
    const out = [];
    for (const p of contact.phones || []) {
        const num = typeof p === 'string' ? p : p?.number;
        if (num) out.push(String(num));
    }
    if (contact.phone) out.push(String(contact.phone));
    return out;
}

export function buildContactIndex(contacts) {
    const byPhone = new Map();
    for (const c of contacts || []) {
        for (const num of contactPhones(c)) {
            const key = normalizePhone(num);
            if (key && !byPhone.has(key)) byPhone.set(key, c);
        }
    }
    return byPhone;
}

export function resolveContact(contactsByPhone, address) {
    const key = normalizePhone(address);
    if (!key) return null;
    return contactsByPhone.get(key) || null;
}

export function primaryPhone(contact) {
    const phones = contactPhones(contact);
    return phones[0] || '';
}

export function formatPhoneDisplay(raw) {
    if (!raw) return '';
    const s = String(raw).trim();
    const digits = s.replace(/\D/g, '');
    if (digits.length === 11 && digits.startsWith('1')) {
        return `+1 ${digits.slice(1, 4)}-${digits.slice(4, 7)}-${digits.slice(7)}`;
    }
    if (digits.length === 10) {
        return `${digits.slice(0, 3)}-${digits.slice(3, 6)}-${digits.slice(6)}`;
    }
    return s;
}
