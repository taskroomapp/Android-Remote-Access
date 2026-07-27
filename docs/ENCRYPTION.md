# Encryption coverage (CKX1)

This project uses **CKX1** as the sole active encryption architecture:

```text
X25519 → HKDF-SHA256 → ChaCha20-Poly1305 → Ed25519 authentication
```

| Layer | Construction |
| --- | --- |
| Device WebSocket session | CKX1 handshake + directional ChaCha20-Poly1305 frames |
| Admin panel REST + `/ws/admin` | Same CKX1 session after `/auth/ckx1/offer` + `/exchange` |
| At-rest (server DB) | Separate master key + ChaCha20-Poly1305 (`AT1:`) |
| Transport | HTTPS / WSS (mandatory for production) |

RSA-OAEP, AES-256-GCM session wraps, `CKR1`, `CK01`, and `ENC1:` are **not** part of the active protocol or storage path.

## Device WebSocket

Full contract: [Session encryption](SESSION_ENCRYPTION.md).

- Bootstrap: `key_offer` / `key_exchange` / `session_ready` (plaintext by design; authenticated via Ed25519 + enrolled public keys).
- Application: `type=enc` with per-message nonce and frame AAD.

## Admin / control panel

After JWT login the panel establishes a CKX1 channel:

1. `POST /api/v1/auth/ckx1/offer`
2. `POST /api/v1/auth/ckx1/exchange` (Ed25519 transcript signature; `device_id` = admin UUID)
3. Protected REST uses header `X-CKX1-Session` and `type=enc` JSON bodies/responses
4. `/ws/admin?token=…&ckx1=…` seals presence events with the same session keys

Binary downloads (`/files/stream`, `/files/download/…`, media, exports) keep the CKX1 session header for authorization but are not JSON-wrapped.

## At-rest

`security.DataEncryptor` derives:

```text
K_atrest = SHA-256("CKX1-ATREST" || 0x00 || encryption_key)
```

String format:

```text
AT1:<base64(nonce12 || ciphertext||tag)>
```

Record-bound AAD examples:

| Store | AAD |
| --- | --- |
| Media | `atrest:media:{media_id}` |
| Audit payload | `atrest:audit-payload:{transaction_id}` |
| Contacts / SMS / calls / paths / location | `atrest:field:…` |

Do **not** reuse WebSocket session keys for database encryption.

## Configuration

```yaml
security:
  encryption_key: "<unique secret>"
  ckx1:
    enabled: true
    protocol_version: 1
    server_x25519_private_key_file: "data/server-x25519.pkcs8"
    server_ed25519_private_key_file: "data/server-ed25519.pkcs8"
    handshake_timeout_seconds: 15
    session_timeout_seconds: 3600
```

Private-key files: directory `0700`, file `0600`. Prefer a secret manager in production.

## Verification

```bash
cd server
go test ./internal/security/ ./internal/cryptokit/ ./internal/websocket/ ./internal/api/
```
