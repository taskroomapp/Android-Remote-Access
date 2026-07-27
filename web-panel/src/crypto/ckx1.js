/**
 * CKX1 — X25519 + HKDF-SHA256 + ChaCha20-Poly1305 + Ed25519
 * Wire-compatible with server/internal/cryptokit and android-agent cryptokit.
 */
import { ed25519, x25519 } from '@noble/curves/ed25519.js';
import { chacha20poly1305 } from '@noble/ciphers/chacha.js';
import { hkdf } from '@noble/hashes/hkdf.js';
import { sha256 } from '@noble/hashes/sha2.js';
import { bytesToHex, concatBytes } from '@noble/hashes/utils.js';

export const CKX1 = {
    PROTOCOL: 'CKX1',
    VERSION: 1,
    ALGORITHM: 'X25519-HKDF-SHA256-CHACHA20-POLY1305',
    HANDSHAKE_LABEL: 'CKX1-HANDSHAKE-V1',
    FRAME_LABEL: 'CKX1-FRAME-V1',
    INFO_C2S: 'CKX1/client-to-server',
    INFO_S2C: 'CKX1/server-to-client',
    DIR_C2S: 'device-to-server',
    DIR_S2C: 'server-to-device',
    KEY_SIZE: 32,
    NONCE_SIZE: 12,
};

const te = new TextEncoder();

function u32be(n) {
    const b = new Uint8Array(4);
    new DataView(b.buffer).setUint32(0, n >>> 0, false);
    return b;
}

export function canonicalEncode(...parts) {
    const chunks = [];
    for (const p of parts) {
        const bytes = typeof p === 'string' ? te.encode(p) : p;
        chunks.push(u32be(bytes.length), bytes);
    }
    return concatBytes(...chunks);
}

export function b64encode(bytes) {
    let s = '';
    for (let i = 0; i < bytes.length; i++) s += String.fromCharCode(bytes[i]);
    return btoa(s);
}

export function b64decode(str) {
    const bin = atob(str);
    const out = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out;
}

export function fingerprintSha256Hex(...parts) {
    const h = sha256.create();
    for (const p of parts) h.update(typeof p === 'string' ? b64decode(p) : p);
    return `sha256:${bytesToHex(h.digest())}`;
}

export function fingerprintLooseEqual(a, b) {
    const norm = (s) => String(s || '')
        .toLowerCase()
        .replace(/^sha256:/, '')
        .replace(/[:\s]/g, '');
    return norm(a) === norm(b);
}

export function generateIdentity() {
    const x25519Private = x25519.utils.randomSecretKey();
    const ed25519Private = ed25519.utils.randomSecretKey();
    return {
        x25519Private,
        x25519Public: x25519.getPublicKey(x25519Private),
        ed25519Private,
        ed25519Public: ed25519.getPublicKey(ed25519Private),
        x25519PublicB64: b64encode(x25519.getPublicKey(x25519Private)),
        ed25519PublicB64: b64encode(ed25519.getPublicKey(ed25519Private)),
    };
}

export function handshakeTranscript(fields) {
    return canonicalEncode(
        CKX1.HANDSHAKE_LABEL,
        fields.sessionId,
        fields.deviceId,
        fields.serverX25519B64,
        fields.serverEd25519B64,
        fields.deviceX25519B64,
        fields.deviceEd25519B64,
        fields.serverNonce,
        fields.deviceNonce,
        CKX1.ALGORITHM,
    );
}

export function deriveDirectionalKeys(sharedSecret, transcriptHash) {
    const c2s = hkdf(sha256, sharedSecret, transcriptHash, te.encode(CKX1.INFO_C2S), CKX1.KEY_SIZE);
    const s2c = hkdf(sha256, sharedSecret, transcriptHash, te.encode(CKX1.INFO_S2C), CKX1.KEY_SIZE);
    return { c2s, s2c };
}

export function frameAAD(sessionId, deviceId, direction, seq, txn = '-') {
    return canonicalEncode(
        CKX1.FRAME_LABEL,
        sessionId,
        deviceId,
        direction,
        String(seq),
        txn || '-',
    );
}

function aeadEncrypt(key, nonce, plaintext, aad) {
    return chacha20poly1305(key, nonce, aad).encrypt(plaintext);
}

function aeadDecrypt(key, nonce, ciphertext, aad) {
    return chacha20poly1305(key, nonce, aad).decrypt(ciphertext);
}

function randomBytes(n) {
    const out = new Uint8Array(n);
    crypto.getRandomValues(out);
    return out;
}

/**
 * Client-side admin CKX1 session (panel = "device" direction in protocol).
 */
export class AdminCkx1Session {
    constructor({ adminId, identity, sessionId, protocolSessionId, c2s, s2c }) {
        this.adminId = adminId;
        this.identity = identity;
        this.sessionId = sessionId; // handshake session_id
        this.protocolSessionId = protocolSessionId; // server ckx1_session token for header
        this.c2s = c2s;
        this.s2c = s2c;
        this.sendSeq = 0n;
        this.recvLast = 0n;
        /** @type {Set<bigint>} */
        this.seen = new Set();
    }

    ready() {
        return this.c2s?.length === CKX1.KEY_SIZE && this.s2c?.length === CKX1.KEY_SIZE;
    }

    seal(plaintextBytes, txn = '-') {
        if (!this.ready()) throw new Error('CKX1 session not ready');
        this.sendSeq += 1n;
        const seq = this.sendSeq;
        const nonce = randomBytes(CKX1.NONCE_SIZE);
        const aad = frameAAD(this.sessionId, this.adminId, CKX1.DIR_C2S, seq, txn);
        const ct = aeadEncrypt(this.c2s, nonce, plaintextBytes, aad);
        return {
            type: 'enc',
            protocol: CKX1.PROTOCOL,
            version: CKX1.VERSION,
            session_id: this.sessionId,
            seq: Number(seq),
            dir: CKX1.DIR_C2S,
            txn: txn || '-',
            nonce: b64encode(nonce),
            ciphertext: b64encode(ct),
        };
    }

    /**
     * Decrypt a server→admin frame.
     * Uses a sliding receive window so large camera payloads that finish after
     * smaller concurrent responses still decrypt (otherwise seq looks "old").
     */
    open(frame) {
        if (!this.ready()) throw new Error('CKX1 session not ready');
        if (frame.protocol !== CKX1.PROTOCOL) throw new Error('bad protocol');
        if (frame.session_id !== this.sessionId) throw new Error('session_id mismatch');
        if (frame.dir !== CKX1.DIR_S2C) throw new Error(`incorrect direction ${frame.dir}`);
        const seq = BigInt(frame.seq);
        if (seq < 1n) throw new Error('bad seq');
        if (this.seen.has(seq)) throw new Error('duplicate sequence');
        const maxSkew = 64n;
        if (this.recvLast > 0n && seq + maxSkew < this.recvLast) {
            throw new Error('replayed or older sequence');
        }
        this.seen.add(seq);
        if (seq > this.recvLast) this.recvLast = seq;
        if (this.seen.size > 4096) {
            this.seen = new Set([seq]);
        }
        const nonce = b64decode(frame.nonce);
        const ct = b64decode(frame.ciphertext);
        const aad = frameAAD(this.sessionId, this.adminId, frame.dir, seq, frame.txn || '-');
        return aeadDecrypt(this.s2c, nonce, ct, aad);
    }
}

/**
 * Complete admin CKX1 handshake against /auth/ckx1/offer + /exchange.
 * @param {(path: string, init?: RequestInit) => Promise<any>} postJson authenticated JSON helper
 */
export async function establishAdminCkx1(adminId, postJson) {
    const identity = generateIdentity();
    const offer = await postJson('/auth/ckx1/offer', { method: 'POST', body: '{}' });
    if (offer.type !== 'key_offer' || offer.protocol !== CKX1.PROTOCOL) {
        throw new Error('invalid key_offer');
    }
    const computed = fingerprintSha256Hex(
        b64decode(offer.server_x25519_public_key),
        b64decode(offer.server_ed25519_public_key),
    );
    if (!fingerprintLooseEqual(computed, offer.server_fingerprint)) {
        throw new Error('server fingerprint mismatch');
    }

    const deviceNonce = b64encode(randomBytes(16));
    const transcript = handshakeTranscript({
        sessionId: offer.session_id,
        deviceId: adminId,
        serverX25519B64: offer.server_x25519_public_key,
        serverEd25519B64: offer.server_ed25519_public_key,
        deviceX25519B64: identity.x25519PublicB64,
        deviceEd25519B64: identity.ed25519PublicB64,
        serverNonce: offer.server_nonce,
        deviceNonce,
    });
    const signature = ed25519.sign(transcript, identity.ed25519Private);
    const exchangeBody = {
        type: 'key_exchange',
        protocol: CKX1.PROTOCOL,
        version: CKX1.VERSION,
        session_id: offer.session_id,
        device_id: adminId,
        device_x25519_public_key: identity.x25519PublicB64,
        device_ed25519_public_key: identity.ed25519PublicB64,
        device_nonce: deviceNonce,
        signature: b64encode(signature),
    };
    const ready = await postJson('/auth/ckx1/exchange', {
        method: 'POST',
        body: JSON.stringify(exchangeBody),
    });
    if (ready.type !== 'session_ready' || !ready.ckx1_session) {
        throw new Error('session_ready missing ckx1_session');
    }

    const peerPub = b64decode(offer.server_x25519_public_key);
    const shared = x25519.getSharedSecret(identity.x25519Private, peerPub);
    const th = sha256(transcript);
    const { c2s, s2c } = deriveDirectionalKeys(shared, th);

    return new AdminCkx1Session({
        adminId,
        identity,
        sessionId: offer.session_id,
        protocolSessionId: ready.ckx1_session,
        c2s,
        s2c,
    });
}
