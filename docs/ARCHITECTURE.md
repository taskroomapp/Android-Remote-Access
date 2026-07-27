# Architecture

## Scope and maturity

This document describes the implementation in the repository, not an aspirational architecture. The stack is suitable for isolated development with an emulator after authorization. Encryption controls are implemented, but the deployment remains unsuitable for production until authorization scoping, origin validation, TLS configuration, path restrictions, audit correctness, and operational secret management are completed. Read [Security and privacy](SECURITY_AND_PRIVACY.md) first.

## Runtime topology

```text
                             ┌─────────────────────────┐
                             │ React web panel          │
                             │ Vite in development      │
                             │ :3000                    │
                             └───────────┬─────────────┘
                     REST /api/v1       │       WS /ws/admin?token=…
                                         ▼
┌────────────────────────────────────────────────────────────────────────────┐
│ Go server (:8443 by default)                                                │
│                                                                            │
│  Gorilla Mux router ── JWT middleware ── handlers                          │
│         │                            │                                     │
│         │                            ├── CommandDispatcher (10 workers)   │
│         │                            └── mirrorStore (process memory)     │
│         │                                                                  │
│  WebSocket Hub                                                           │
│  /ws/devices/{device_id} ───── device connections & response correlation  │
│  /ws/admin ─────────────────── admin status-update sessions                │
└───────────────┬───────────────────────────┬────────────────────────────────┘
                │                           │
                ▼                           ▼
      PostgreSQL (required)          Redis (optional)
      users, sessions, devices,      online-device cache, queue helpers,
      audit rows, pending commands,  command-response helpers, rate-limit
      encrypted media data           primitive (not wired as middleware)
                │
                │ WebSocket
                ▼
┌────────────────────────────────────────────────────────────────────────────┐
│ Android agent                                                               │
│ MainActivity → enrollment / permission UI → AgentService (foreground)     │
│ AgentService → OkHttp WebSocket → CommandHandler → Android platform APIs  │
└────────────────────────────────────────────────────────────────────────────┘
```

## Components

### Go server

| Package | Role |
| --- | --- |
| `cmd/` | Reads YAML configuration, initializes dependencies, creates default development admin, starts the HTTP server, and handles shutdown. |
| `internal/api/` | Gorilla Mux routes, JWT middleware, REST handlers, file stream aggregator, and in-memory mirror snapshot store. |
| `internal/websocket/` | Device/admin WebSocket connections, ping/pong loops, device presence, status broadcasts, and command-response correlation. |
| `internal/dispatcher/` | Fixed worker pool, command dispatch, PostgreSQL pending-command scan, command result lookup, audit/media persistence. |
| `internal/database/` | PostgreSQL schema/queries and optional Redis cache adapter. |
| `internal/security/` | JWTs, bcrypt password hashes, AES-GCM helper, permission checker, and TLS helpers. |
| `internal/models/` | API, domain, and device protocol structures. |

The Go module requires Go 1.21. `main.go` configures 180-second HTTP read/write timeouts and a 120-second idle timeout.

### Android agent

The Android app has application ID `com.remoteagent`, `minSdk 26`, `compileSdk 34`, `targetSdk 34`, and Java 17 source/target compatibility.

- `MainActivity` displays the server URL, server-assigned device ID, agent ID, credentials for interactive enrollment, connect/disconnect controls, and requests runtime permissions.
- `AgentService` runs as a visible `dataSync` foreground service, starts a WebSocket connection, sends a heartbeat every 30 seconds, and retries connection after five seconds when disconnected.
- `CommandHandler` dispatches implemented commands and checks the relevant Android permission before using platform APIs.
- `EnrollmentClient` logs in through the server REST API and registers the agent's device UUID to obtain a server UUID.
- `BootReceiver` and `NetworkReceiver` are declared; see the Android source before relying on restart behavior for an operational design.

### Web panel

The panel uses React 18, Vite 5, React Router, Leaflet/React Leaflet, and Lucide SVG icons. In development Vite proxies `/api` and `/ws` to `VITE_DEV_API_TARGET`, defaulting to `http://localhost:8443`.

The panel keeps access and refresh tokens in `localStorage` and uses IndexedDB database `android_remote_access` for local cached UI state. Those facts are security-relevant and are not recommendations for a production design.

## Device enrollment and identity

There are two identifiers:

| Identifier | Created by | Purpose |
| --- | --- | --- |
| **Agent UUID** | Generated locally (`KeyStoreIdentity` / preferences) | Stable identifier reported by the app in enrollment and WebSocket headers. |
| **Server device ID** | Generated by PostgreSQL-backed enrollment | UUID used in API URLs, WebSocket path, registry rows, and command targets. |
| **Device CKX1 keys** | Device X25519 + Ed25519 public keys | Enrolled on the server; private keys never leave the device |

Interactive enrollment follows this path:

1. The app calls `POST /api/v1/auth/login` with administrator credentials.
2. It sends `POST /api/v1/devices` with its agent UUID, metadata, and CKX1 public keys / fingerprint.
3. It saves the returned server device ID in local preferences.
4. It connects to `/ws/devices/{server-device-id}` and sends an `enrollment` message.

On a device WebSocket connection, the server can also auto-create or reconcile a device row from the agent's enrollment message. **This is a release blocker:** the WebSocket path must not auto-create or reconcile devices from unauthenticated metadata before identity verification.

Canonical identity wording:

> The URL and `X-Device-UUID` identify a connection candidate. The device is not authenticated until the enrolled identity and signed handshake transcript are verified. Commands are rejected until `session_ready`.

Do not describe the initial upgrade as an “authenticated WebSocket connection.” A peer can open a socket; it cannot issue commands before `session_ready`; TLS client certificates are requested, not verified as mTLS.

## Command lifecycle

```text
Administrator action
  → authenticated REST command request
  → CommandDispatcher worker
  → audit row INSERT (pending)
  → Is device connected and session_ready?
       ├─ no: INSERT pending_commands row (24 h expiry), return queued
       └─ yes: send AES-GCM sealed command over device WebSocket
                    → Android CommandHandler
                    → sealed response over WebSocket
                    → Hub correlates transaction ID
                    → dispatcher returns result and attempts audit/media update
```

### Timeouts, queueing, and results

- The command dispatcher has a buffered task queue of 1,000 and starts 10 workers.
- A device WebSocket send waits for a reply for the command's `timeout_seconds`; it returns `command timeout` when no matching response arrives.
- Offline commands are persisted in PostgreSQL for 24 hours and scanned every five seconds for currently online devices, ordered by priority descending then creation time ascending.
- Command status lookup checks the dispatcher's in-memory map first, then Redis response helpers, then PostgreSQL audit rows.
- Cancellation API exists but currently returns a placeholder; it does not cancel a dispatched or queued task.

### Important audit limitation

`InitSchema` installs a PostgreSQL trigger that forbids *all* updates to `audit_logs`, while the dispatcher updates `response_status` and `response_data` on the same table. As written, those updates are rejected after schema initialization. The project therefore does **not** currently provide a correct immutable append-only audit ledger. This is a P0 release blocker, not an assurance.

Choose one design before sensitive deployment (do **not** simply remove the trigger and allow unrestricted updates):

1. **Append-only event model** — separate rows such as `audit_command_created`, `audit_command_dispatched`, `audit_command_completed`, `audit_command_failed`, `audit_command_timed_out`.
2. **Mutable command-status table + immutable audit events** — current status separately; append-only events for the ledger.

Required tests once redesigned: every command creates an initial event; every terminal result creates a completion event; failed DB writes are surfaced; normal application code cannot modify/delete audit events; sensitive event fields remain encrypted.

## WebSocket protocol

### Device connection

- Endpoint: `/ws/devices/{server_device_id}`
- Header used by the agent: `X-Device-UUID` (connection candidate identity; not alone an authentication proof)
- WebSocket max inbound message size: 512 KiB
- Hub send buffer: 256 messages
- Ping interval: 54 seconds; pong/read deadline: 60 seconds
- Device `WritePump` sends one logical message per text frame (no newline coalescing)

#### Session encryption handshake (CKX1)

Device WebSocket application messages are plaintext only during the authenticated **CKX1** bootstrap. The server offers X25519/Ed25519 public keys; the device verifies the pinned fingerprint and signs the length-prefixed transcript with Ed25519. Both sides derive directional ChaCha20-Poly1305 keys via X25519 + HKDF-SHA256. After `session_ready`, all application messages use `type=enc` frames. Commands are rejected before session readiness. TLS remains mandatory.

Full contract: [Session encryption](SESSION_ENCRYPTION.md). Coverage summary: [Encryption coverage](ENCRYPTION.md).

| `type` | Direction | Notes |
| --- | --- | --- |
| `key_offer` | server → device | CKX1; server X25519/Ed25519 public keys, fingerprint, nonce |
| `key_exchange` | device → server | Device public keys + Ed25519 signature over handshake transcript |
| `session_ready` | server → device | Emitted only after signature verify + X25519/HKDF commit |
| `enc` | both | ChaCha20-Poly1305; AAD = length-prefixed `CKX1-FRAME-V1` fields |

**Key derivation:**

```text
shared = X25519(local, peer)
c2s/s2c = HKDF-SHA256(shared, salt=SHA256(transcript), info=CKX1/client-to-server|server-to-client)
```

After `session_ready`, the server sends a sealed `DeviceCommand` JSON object (fields include `transaction_id`, `command_type`, `payload`, `timeout_seconds`, `issued_at`).

The agent sends these **inner** message categories (always inside `enc` after handshake):

| Inner `type` | Required/useful fields | Server behavior |
| --- | --- | --- |
| `enrollment` | Agent UUID, friendly name, OS/hardware metadata, certificate fingerprint | Reconciles or creates a device record and updates the hub identity. |
| `heartbeat` | `battery_level`, timestamp | Updates device status/last check-in. |
| `response` | Transaction ID, command type, status, base64 `data` or `error` | Correlates the response with the waiting dispatcher call. |
| `status_update` | Device-specific status payload | Broadcasts update to connected admin sessions. |

### Admin connection

- Endpoint: `/ws/admin?token=<JWT>`
- Authentication: the access token may be supplied as the `token` query parameter.
- Supported inbound subscription message: `{ "type": "subscribe", "device_ids": ["uuid", "…"] }`.
- Broadcasts currently include `device_online` (emitted after device session crypto is ready), `device_session_ready`, `device_offline`, and agent-originated `status_update` messages.
- Admin WebSocket traffic is **not** wrapped in the device session AEAD.

The WebSocket upgrader currently accepts every origin. Treat these connections as development-only until origin validation and stronger session handling are implemented.

## Persistence model

### PostgreSQL

`PostgresDB.InitSchema` creates these tables and indexes when the server starts with a reachable database:

| Table | Current use |
| --- | --- |
| `administrators` | Login identity, bcrypt password hash, role, stored permissions, active flag. |
| `devices` | Registry metadata, last check-in, reported battery level, agent UUID, certificate fingerprint. |
| `sessions` | Refresh-token sessions and client metadata. |
| `audit_logs` | Command audit payload/status/data. Trigger behavior currently conflicts with response updates. |
| `pending_commands` | Offline command persistence, priority, status, and expiry. |
| `media_files` | AES-GCM-encrypted bytes from selected media command responses plus checksum and metadata. |

`media_files.encrypted_data` is written with crypto-kit AES-GCM (`DataEncryptor`) using record-bound AAD. The media download handler decrypts before returning bytes to an authenticated admin. See [Encryption coverage](ENCRYPTION.md). Downloads still need production hardening (`Cache-Control: no-store`, safe disposition, download-time authorization, audit). File command paths still lack a canonical storage-root policy — see [Security and privacy](SECURITY_AND_PRIVACY.md).

### Redis

Redis failures are logged and do not stop the server. The adapter contains keys/helpers for:

- online device records and last-seen values (24-hour TTL),
- a sorted-set command queue,
- temporary command responses,
- session cache, and
- a rate-limit counter primitive.

Current limitations: the dispatcher does not call Redis `EnqueueCommand`, response storage is not wired after execution, and `rate_limit` YAML values are not registered as HTTP middleware.

### Browser IndexedDB

The web panel creates these object stores: `file_trees`, `contacts`, `sms_inbox`, `sms_sent`, `transfers`, `location_history`, `camera_captures`, `audio_recordings`, and `preferences`. These are local browser caches; they do not replace server-side retention controls.

## Configuration and transport

`server/config.yaml` contains server host/port, PostgreSQL URL, Redis address, JWT/encryption strings, and TLS file paths. It is a development configuration. Configuration is read from YAML; the values are not sourced from environment variables by `main.go`.

When `tls.enabled` is true, the Go server uses TLS 1.2+ and requests a client certificate (`tls.RequestClientCert`). Requesting a certificate is not equivalent to requiring and validating a trusted client certificate. In its current form, do not describe this deployment as mutually authenticated TLS.

The Android network security policy permits cleartext only for `10.0.2.2`, `localhost`, and `127.0.0.1`, which supports emulator development. Production transport hardening is not implemented by configuration alone.

## Boundaries and non-goals of the current source

- No MDM policy enforcement, work profile, remote wipe, device lock, or app allow/block policy is implemented.
- No device command authorization scoped to a particular user/device assignment is implemented.
- No server-side browser-history handler or continuous camera/video stream is implemented.
- Mirror snapshots are held in Go-process memory, not PostgreSQL or Redis; a process restart loses them.
- The agent's raw file handlers use Java `File` paths. The server does not currently enforce an allowlisted storage root or canonical-path policy.

For concrete endpoint status and compatibility, see [API reference](API_REFERENCE.md) and [Android agent](../android-agent/README.md).
