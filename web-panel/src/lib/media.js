/**
 * Build a safe data: URL from API payload (base64 string, data URL, or byte-ish string).
 * Accepts decrypted/base64 JPEG from the API — never display ciphertext blobs as images.
 */
export function extractBase64Payload(data) {
    if (data == null) return null;
    if (typeof data === 'object') {
        if (data instanceof ArrayBuffer) {
            return bytesToBase64Payload(new Uint8Array(data));
        }
        if (ArrayBuffer.isView(data)) {
            return bytesToBase64Payload(new Uint8Array(data.buffer, data.byteOffset, data.byteLength));
        }
        const raw = data.image ?? data.base64 ?? data.jpeg ?? data.content ?? data.data ?? data.bytes ?? data.audio_base64 ?? data.audio;
        if (raw != null) return extractBase64Payload(raw);
        return null;
    }
    if (typeof data !== 'string') return null;

    let s = data.trim();
    const dataUrlMatch = /^data:([^;]+);base64,(.+)$/is.exec(s);
    if (dataUrlMatch) return { mime: dataUrlMatch[1], b64: cleanBase64(dataUrlMatch[2]) };

    if (s.startsWith('{') || s.startsWith('[')) {
        try {
            return extractBase64Payload(JSON.parse(s));
        } catch {
            return null;
        }
    }

    // Binary JPEG/PNG already decoded to a JS string (e.g. via atob)
    if (s.length >= 3) {
        const b0 = s.charCodeAt(0) & 0xff;
        const b1 = s.charCodeAt(1) & 0xff;
        if ((b0 === 0xff && b1 === 0xd8) || (b0 === 0x89 && b1 === 0x50)) {
            return binaryStringToBase64(s);
        }
    }

    const cleaned = cleanBase64(s);
    if (!cleaned || cleaned.length < 16) return null;
    if (!isLikelyBase64(cleaned)) {
        return binaryStringToBase64(s);
    }

    const mime = sniffMimeFromBase64(cleaned);
    return { mime, b64: cleaned };
}

export function toDataUrl(data, defaultMime = 'application/octet-stream') {
    const extracted = extractBase64Payload(data);
    if (!extracted?.b64) return null;
    const mime = extracted.mime || defaultMime;
    return `data:${mime};base64,${extracted.b64}`;
}

export function toImageDataUrl(data) {
    return toDataUrl(data, 'image/jpeg');
}

export function toAudioDataUrl(data, mime = 'audio/mp4') {
    return toDataUrl(data, mime);
}

function cleanBase64(s) {
    return s.replace(/\s/g, '').replace(/-/g, '+').replace(/_/g, '/');
}

function isLikelyBase64(s) {
    if (!/^[A-Za-z0-9+/=]+$/.test(s)) return false;
    if (s.includes('=') && s.length % 4 === 1) return false;
    try {
        atob(s.slice(0, Math.min(256, s.length)));
        return true;
    } catch {
        return false;
    }
}

function binaryStringToBase64(binary) {
    if (!binary || typeof binary !== 'string') return null;
    const len = binary.length;
    if (len < 4) return null;
    const bytes = new Uint8Array(len);
    for (let i = 0; i < len; i++) bytes[i] = binary.charCodeAt(i) & 0xff;
    return bytesToBase64Payload(bytes);
}

function bytesToBase64Payload(bytes) {
    if (!bytes || bytes.length < 4) return null;
    let mime = 'application/octet-stream';
    if (bytes[0] === 0xff && bytes[1] === 0xd8) mime = 'image/jpeg';
    else if (bytes[0] === 0x89 && bytes[1] === 0x50) mime = 'image/png';
    else if (bytes[0] === 0x47 && bytes[1] === 0x49 && bytes[2] === 0x46) mime = 'image/gif';
    else if (bytes[0] === 0x4f && bytes[1] === 0x67 && bytes[2] === 0x67) mime = 'audio/ogg';
    let bin = '';
    const chunk = 0x8000;
    for (let i = 0; i < bytes.length; i += chunk) {
        bin += String.fromCharCode(...bytes.subarray(i, i + chunk));
    }
    return { mime, b64: btoa(bin) };
}

function sniffMimeFromBase64(b64) {
    // Common prefixes after base64 encoding
    if (b64.startsWith('/9j/')) return 'image/jpeg';
    if (b64.startsWith('iVBOR')) return 'image/png';
    if (b64.startsWith('R0lGOD')) return 'image/gif';
    if (b64.startsWith('UklGR')) return 'image/webp';
    if (b64.startsWith('T2dnUw')) return 'audio/ogg';
    return null;
}

/** True when payload looks like displayable image bytes/base64 (not AES ciphertext). */
export function looksLikeImagePayload(data) {
    const extracted = extractBase64Payload(data);
    if (!extracted?.b64) return false;
    if (extracted.mime && extracted.mime.startsWith('image/')) return true;
    return Boolean(sniffMimeFromBase64(extracted.b64));
}
