# Full system lifecycle (CKX1)

End-to-end path for the Android agent, Go server, and control panel under the **CKX1** crypto standard.

> **Authoritative crypto:** X25519 + HKDF-SHA256 + ChaCha20-Poly1305 + Ed25519.  
> Details: [SESSION_ENCRYPTION.md](SESSION_ENCRYPTION.md), [ENCRYPTION.md](ENCRYPTION.md).

## Components

```text
┌─────────────┐     HTTPS/WSS + CKX1      ┌──────────────┐
│  web-panel  │◄─────────────────────────►│    server    │
│  (React)    │  JWT + admin CKX1 session │     (Go)     │
└─────────────┘                           │  AT1 at-rest │
                                          └──────┬───────┘
                                                 │ WSS + CKX1
                                          ┌──────▼───────┐
                                          │ android-agent│
                                          │  (Kotlin)    │
                                          └──────────────┘
```

| Plane | Parties | Crypto |
| --- | --- | --- |
| A — Operator auth | Panel ↔ server REST | TLS + JWT; then CKX1 for protected JSON |
| B — Device session | Agent ↔ server WS | CKX1 handshake → directional `enc` frames |
| C — At-rest | Server DB | `AT1:` ChaCha20-Poly1305, record-bound AAD |
| D — Transport | All | HTTPS / WSS mandatory in production |

## Server boot

1. Load YAML (`encryption_key`, `ckx1.*` key files, JWT, DB, TLS).
2. Load or generate server X25519 + Ed25519 identity (`0600` files).
3. Derive at-rest key: `SHA-256("CKX1-ATREST" || 0x00 || encryption_key)`.
4. Start hub with `SetCKX1Identity`, HTTP/WS listeners.

## Agent: identity → enroll → session

1. `KeyStoreIdentity` loads/creates long-term X25519 + Ed25519 keys and agent UUID.
2. REST login (admin credentials) → `POST /devices` with public keys + fingerprint.
3. Connect `/ws/devices/{server_device_id}` with `X-Device-UUID`.
4. CKX1: `key_offer` → verify fingerprint → sign transcript → `key_exchange` → `session_ready`.
5. Derive directional keys; all application JSON is `type=enc`.
6. Inner messages: enrollment, heartbeats, command responses.

## Panel: login → CKX1 → operate

1. `POST /auth/login` → JWT in `localStorage`.
2. `POST /auth/ckx1/offer` + `/exchange` (panel generates ephemeral X25519 + Ed25519).
3. Store `ckx1_session`; send `X-CKX1-Session` on protected REST.
4. Encrypt JSON bodies / decrypt `X-CKX1-Encrypted` responses.
5. Open `/ws/admin?token=…&ckx1=…` for sealed presence events.
6. Commands: REST → dispatcher → device only if `IsDeviceSessionReady`.

## Handshake transcript (both device and admin)

Length-prefixed canonical fields:

```text
CKX1-HANDSHAKE-V1
session_id
device_id          # agent UUID or admin UUID
server_x25519_public_key
server_ed25519_public_key
device_x25519_public_key
device_ed25519_public_key
server_nonce
device_nonce
X25519-HKDF-SHA256-CHACHA20-POLY1305
```

```text
shared = X25519(local_sk, peer_pk)
th = SHA-256(transcript)
c2s = HKDF(shared, salt=th, info="CKX1/client-to-server")
s2c = HKDF(shared, salt=th, info="CKX1/server-to-client")
```

## Frame rules

```text
AAD = Canonical(CKX1-FRAME-V1, session_id, device_id, dir, seq, txn)
```

Reject wrong direction, session, duplicate/older `seq`, bad nonce/tag.

## Teardown

| Event | Effect |
| --- | --- |
| Agent stop / uninstall | Local keys gone; server row/`AT1` history may remain |
| Admin logout | Clear JWT + CKX1 client state |
| Server key rotation | Devices must re-handshake; revoke live sessions |
| Device delete | Close socket; remove registry row |

## Verification

```bash
cd server && go test ./internal/cryptokit/ ./internal/websocket/ ./internal/security/ ./internal/api/
```

Manual: agent reaches `session_ready` with only rising `enc` seq; panel login completes CKX1 offer/exchange; protected REST fails without `X-CKX1-Session`; DB sensitive columns show `AT1:` not plaintext.
