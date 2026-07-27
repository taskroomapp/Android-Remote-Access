# Device WebSocket session encryption (CKX1)

Application-layer encryption for the Android agent ↔ Go server device WebSocket path.

> **Authoritative construction:** `CKX1` = X25519 + HKDF-SHA256 + ChaCha20-Poly1305 + Ed25519.  
> The former RSA-OAEP / AES-256-GCM / `CKR1` session path is **removed** from the active protocol.  
> TLS remains mandatory for production (`wss://`).

## Canonical summary

Device WebSocket application messages are plaintext only during the authenticated CKX1 bootstrap. The server sends a `key_offer` with its X25519/Ed25519 public keys; the device verifies the pinned fingerprint, signs the handshake transcript with Ed25519, and both sides derive directional ChaCha20-Poly1305 keys via X25519 + HKDF-SHA256. After `session_ready`, all application messages use `type=enc` frames. Commands are rejected before session readiness.

## Key model

| Party | Keys |
| --- | --- |
| Device | Long-term X25519 (agreement) + Ed25519 (signing); private keys in Android Keystore-wrapped files |
| Server | Long-term X25519 + Ed25519; files `data/server-x25519.pkcs8` / `data/server-ed25519.pkcs8` mode `0600` |

Never use X25519 keys for signing or Ed25519 keys for agreement.

## Handshake

```text
Device                         Server
  |-- WS connect -------------->|
  |<-- key_offer ---------------| protocol=CKX1, public keys, nonce, fingerprint
  |-- key_exchange ------------>| Ed25519 signature over transcript
  |<-- session_ready -----------|
  |-- enc / enc --------------->|
```

### Transcript (length-prefixed canonical encoding)

```text
CKX1-HANDSHAKE-V1
session_id
device_id
server_x25519_public_key
server_ed25519_public_key
device_x25519_public_key
device_ed25519_public_key
server_nonce
device_nonce
X25519-HKDF-SHA256-CHACHA20-POLY1305
```

### Session key derivation

```text
shared = X25519(local_private, peer_public)
transcript_hash = SHA-256(transcript)
c2s = HKDF-SHA256(IKM=shared, salt=transcript_hash, info="CKX1/client-to-server")
s2c = HKDF-SHA256(IKM=shared, salt=transcript_hash, info="CKX1/server-to-client")
```

## Encrypted frame

```json
{
  "type": "enc",
  "protocol": "CKX1",
  "version": 1,
  "session_id": "uuid",
  "seq": 1,
  "dir": "device-to-server",
  "txn": "-",
  "nonce": "<base64 12 bytes>",
  "ciphertext": "<base64 ChaCha20-Poly1305>"
}
```

### Frame AAD (length-prefixed)

```text
CKX1-FRAME-V1
session_id
device_id
direction
sequence
transaction_id
```

Reject duplicate/older `seq`, wrong `dir`, wrong session/device binding, bad tag.

## Configuration

```yaml
security:
  encryption_key: "..."
  ckx1:
    enabled: true
    protocol_version: 1
    server_x25519_private_key_file: "data/server-x25519.pkcs8"
    server_ed25519_private_key_file: "data/server-ed25519.pkcs8"
```

## Admin channel

Operators use the same CKX1 construction after JWT auth. Handshake `device_id` is the admin UUID. The server returns `ckx1_session` for `X-CKX1-Session` and `/ws/admin?ckx1=…`. Frame directions still use `device-to-server` / `server-to-device` (panel = client).

## Verification

```bash
cd server && go test ./internal/cryptokit/ ./internal/websocket/
```

See also: [Encryption coverage](ENCRYPTION.md), [Security and privacy](SECURITY_AND_PRIVACY.md).
