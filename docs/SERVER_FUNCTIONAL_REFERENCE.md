# Server — deep functional reference (every file & function)

Behavior-only documentation for first-party Go sources under `server/`. No source listings. For the file checklist see [PROJECT_FILE_CATALOG.md](PROJECT_FILE_CATALOG.md).

---

## Development role of the server

The Go process is the **control plane**: it authenticates operators, registers devices, correlates WebSocket sessions, dispatches commands, persists audit/comms/artifacts/media, and fans out presence events to the control panel. Nothing in the panel or agent can complete an operator action without passing through this layer (except pure local IndexedDB UI state in the browser).

---

## `go.mod` / `go.sum`

**Role.** Declare module `github.com/enterprise/android-remote-access/server`, Go 1.21, and lock direct libraries (Gorilla Mux/WebSocket, JWT, pq, Redis, excelize, yaml, uuid, x/crypto). Not runtime logic; required for reproducible builds.

---

## `config.yaml`

**Role.** Development runtime configuration consumed by `loadConfig`: bind address, Postgres URL, Redis address, JWT/encryption secrets, TLS flags, rate-limit knobs. Values are placeholders; changing them alters which datastores the process attaches to and whether HTTPS is used. Rate-limit keys are currently declarative only.

---

## `cmd/config.go`

**File role.** Defines the typed configuration surface and the YAML load path used before any network listener starts.

| Symbol | Role in the project |
| --- | --- |
| `Config` | Root document binding all subsystems’ settings into one object passed through startup |
| `ServerConfig` | Host/port that become `http.Server.Addr` |
| `DatabaseConfig` | Postgres DSN; empty means “no DB” (degraded mode) |
| `RedisConfig` | Optional cache endpoint |
| `SecurityConfig` | JWT signing secret, media encryption key material, token issuer string |
| `TLSConfig` | Whether ListenAndServeTLS is used and which cert/key paths |
| `RateLimitConfig` | Intended request budget; not enforced by middleware today |
| `loadConfig` | Reads YAML from disk, unmarshals into `Config`, fills insecure development defaults when fields are blank so a missing key does not prevent local bring-up |

---

## `cmd/main.go`

**File role.** Process lifecycle owner: wires every major subsystem and owns graceful shutdown.

| Function | Role in the project |
| --- | --- |
| `main` | Parses CLI flags; optionally emits sample config; loads YAML; opens Postgres and runs schema init; opens Redis if configured; constructs AES encryptor (fatal if key invalid); constructs WebSocket hub and command dispatcher; starts hub event loop; constructs API server + `http.Server` with long timeouts for large downloads; optionally attaches TLS; ensures a default admin exists; serves until SIGINT/SIGTERM; shuts down HTTP then closes DB/Redis |
| `createDefaultAdmin` | Bootstraps first-login capability for local development by inserting `admin` / `admin123!` as `super_admin` with a full permission set when that username is absent; logs a production warning |
| `generateSampleConfig` | Prints a documented YAML template to stdout so operators can create a new config without reading source |

---

## `internal/api/` — HTTP surface (split by domain)

Handlers remain methods on `*Server` in `package api`. Routes stay in `handlers.go` (`setupRoutes`); implementations live in focused files.

| File | Role |
| --- | --- |
| `handlers.go` | `Server`, `Config`, `NewServer`, `Router`, `setupRoutes` |
| `middleware.go` | `authMiddleware`, `authenticateAdmin`, `withAdmin`, `getAdmin` |
| `response.go` | `handleHealth`, `writeJSON`, `writeError`, `getClientIP`, `mediaContentType` |
| `auth.go` | `handleLogin`, `handleRefresh`, `handleLogout` |
| `devices.go` | Device list/get/enroll/delete/status |
| `commands.go` | Execute / status / cancel |
| `files.go` | File list/read/delete/download |
| `contacts.go` | Live contacts pull |
| `call_logs.go` | Live call-log pull |
| `media.go` | Stored media list/file |
| `actions_camera.go` | Camera action API |
| `actions_mic.go` | Microphone action API |
| `actions_location.go` | Location + foreground-app actions |
| `transfers.go` | Transfer stubs |
| `audit.go` | Audit search/get |
| `dashboard.go` | Dashboard stats |
| `admins.go` | Admin CRUD stubs |
| `websocket.go` | Device + admin WebSocket upgrades |
| `file_stream.go` | Chunked remote file stream (unchanged) |
| `comms.go` / `artifacts.go` / `mirrors.go` | Persistence APIs (unchanged) |

### Types (in `handlers.go` / `middleware.go`)

| Type | Role |
| --- | --- |
| `Server` | Aggregate of router + DB + Redis + hub + dispatcher + security helpers + mirror store; every handler is a method on this type |
| `Config` | Subset of process config the API layer needs (host/port/secrets/issuer) |
| `contextKey` | Private typed key so the authenticated administrator can travel with the request without colliding with other context values |

### Construction / routing / auth

| Function | Role |
| --- | --- |
| `NewServer` | Assembles the API facade: creates Gorilla router, security helpers, encryptor (best-effort), empty mirror store, then registers all routes. This is the object `main` hands to `http.Server` |
| `Router` | Exposes the mux as `http.Handler` so the stdlib server can serve it |
| `setupRoutes` | Declares the entire public HTTP/WS contract: health, auth (public), protected `/api/v1` resource groups, device WS, admin WS. Middleware is attached per subrouter |
| `authMiddleware` | Gate for protected REST routes: reject unauthenticated callers with 401; otherwise inject administrator into context for handlers |
| `authenticateAdmin` | Resolves Bearer header or `token` query (needed by browser WebSockets); validates access JWT; loads active admin row from Postgres; refuses inactive/missing admins and DB-down conditions |
| `withAdmin` | Stores admin on context after successful auth |
| `getAdmin` | Retrieves admin in handlers for permission checks and audit attribution |

### Health & session

| Function | Role |
| --- | --- |
| `handleHealth` | Liveness/readiness style probe used by agents before enrollment and by operators; reports per-dependency ok/error and overall healthy/degraded |
| `handleLogin` | Credential verification → JWT pair → durable session row → optional Redis session cache → returns tokens + admin profile used by the panel login page and by the Android enrollment client |
| `handleRefresh` | Rotates refresh sessions: validate refresh JWT, ensure DB session matches, delete old session, issue new pair. Enables long-lived panel sessions without keeping a single access token forever |
| `handleLogout` | Best-effort cache invalidation and success response; panel clears local tokens regardless |

### Devices

| Function | Role |
| --- | --- |
| `handleListDevices` | Builds the operator device registry: merges Postgres inventory with live hub presence, includes hub-only connected devices not yet fully reflected in DB, returns online/offline counts for dashboard/sidebar |
| `handleGetDevice` | Single-device detail for Device Detail page; overlays live online/offline |
| `handleEnrollDevice` | Called by Android `EnrollmentClient`: creates a new server UUID for a new agent UUID, or re-enrolls an existing agent UUID and returns the canonical server device ID required for the WebSocket path |
| `handleDeleteDevice` | Revokes a device from the registry and forcibly closes its live socket if connected |
| `handleGetDeviceStatus` | Compact status payload (online, battery, last check-in, optional Redis pending count) for polling UIs |

### Commands

| Function | Role |
| --- | --- |
| `handleExecuteCommand` | Central operator action entry: permission `device:command`, validate allowlisted type, build `CommandTask` with audit metadata (IP/UA), block until dispatcher returns, encode binary-safe data for JSON, return 202 when queued offline |
| `handleGetCommandStatus` | Polling endpoint used by panel `pollCommandResult` / Orders / Console while a command is in flight or to recover a recent result |
| `handleCancelCommand` | Placeholder acknowledging cancel is not implemented; exists so clients do not 404 |

### Files / contacts / media / actions

| Function | Role |
| --- | --- |
| `handleFileList` | REST convenience for directory listing via `CommandBuilder.FileList` (permission `file:list`) |
| `handleFileRead` | Full-file read via agent (permission `file:read`); also used as download fallback |
| `handleFileDelete` | Destructive delete on device (requires `file:*`) |
| `handleDownloadFile` | Serves a **stored** encrypted media blob (camera/mic artifacts) after decrypt; not a live device path |
| `handleGetContacts` | Live contacts pull from device |
| `handleGetCallLogs` | Live call-log pull from device |
| `handleGetMedia` | Lists recent persisted media metadata for a device |
| `handleGetMediaFile` | Alias that reuses `handleDownloadFile` for `/media/file/{id}` |
| `handleCameraAction` | Action-oriented camera API: `snapshot` dispatches; `stream`/`stop` are stub responses today |
| `handleMicAction` | Action-oriented mic API: `start` dispatches timed record; `stop` currently returns a stub message rather than a full `mic_stop` dispatch |
| `handleLocationAction` | One-shot location command for Location page / actions API |
| `handleForegroundAppAction` | Foreground/recent-app command for Live View |

### Audit / dashboard / admins / transfers

| Function | Role |
| --- | --- |
| `handleSearchAuditLogs` | Paginated filtered audit search powering Audit Logs page |
| `handleGetAuditLog` | Stub single-record detail |
| `handleDashboardStats` | Aggregates DB stats then overlays live online count from hub for Dashboard |
| `handleListAdmins` | Stub empty list (permission-gated) |
| `handleCreateAdmin` | Super-admin-only creation of new administrator accounts with hashed passwords |
| `handleGetAdmin` / `handleUpdateAdmin` / `handleDeleteAdmin` | Stub placeholders for future admin CRUD |
| `handleListTransfers` | Returns empty transfer collections so Downloads page can merge with local IndexedDB |
| `handleClearCompletedTransfers` / `handleTransferAppeal` | No-op success stubs matching panel API expectations |

### WebSockets

| Function | Role |
| --- | --- |
| `handleDeviceWebSocket` | Upgrades Android connections; resolves canonical device ID from path and/or `X-Device-UUID` (connection candidate); registers hub client; starts read/write pumps. Authenticated key exchange is required before session readiness / commands |
| `handleAdminWebSocket` | Authenticates operator; upgrades browser socket; registers admin session for presence events used by DeviceContext |

### Response helpers

| Function | Role |
| --- | --- |
| `writeJSON` | Uniform JSON responses |
| `writeError` | Uniform error envelope (`code` + message) consumed by panel `ApiError` |
| `getClientIP` | Extracts client IP (honors `X-Forwarded-For`) for audit rows |
| `mediaContentType` | Chooses Content-Type when streaming stored media by filename/type |

---

## `internal/api/file_stream.go`

**File role.** Makes large device files downloadable through HTTP by repeatedly issuing agent chunk commands and writing bytes to the response body—used by the panel’s preferred download path.

| Symbol | Role |
| --- | --- |
| `fileStreamChunkSize` | Fixed 32 KiB chunk size balancing latency vs. round-trips |
| `fileChunkPayload` | JSON body shape sent to the agent for each chunk |
| `fileChunkResponse` | Expected agent JSON: base64 content, bytes read, file size, offset |
| `handleFileStream` | AuthZ `file:read`; parses optional Range start; loops chunk commands via `CommandBuilder.FileReadChunk`; sets streaming headers; flushes progressively; ends when bytes exhausted or device fails mid-stream |

---

## `internal/api/mirrors.go`

**File role.** Speeds the control panel by caching last-known snapshots (file trees, contacts, etc.) in process memory and optionally projecting them into Postgres.

| Symbol | Role |
| --- | --- |
| `mirrorStore` | Thread-safe in-memory map of snapshots keyed by device + mirror type |
| `newMirrorStore` | Constructs empty store at server start |
| `mirrorStorageKey` | Canonical key formatter |
| `get` / `set` | Read/write snapshot JSON |
| `handleGetMirror` | Panel fetch of cached snapshot (`?type=`); 404 when absent |
| `handleMirrorUpdate` | Panel push after a sync; updates memory and may persist |
| `persistMirrorSnapshot` | Type-specific bridge into comms/artifact upserts so mirrors become durable |
| `extractMirrorItems` | Normalizes heterogeneous snapshot JSON into item arrays for persistence |

---

## `internal/api/comms.go`

**File role.** Durable communications warehouse API used after the panel pulls live contacts/SMS/calls.

| Function | Role |
| --- | --- |
| `handleSaveDeviceComms` | Permission-scoped upsert of contacts, messages, and/or calls arrays into Postgres |
| `handleListDeviceComms` | Reads stored rows by `type` + `limit` for offline browsing in Contacts/SMS page |
| `handleExportDeviceComms` | Builds an Excel workbook from stored comms for operator export/download |

---

## `internal/api/artifacts.go`

**File role.** Durable locations, file inventory, and media upload API for Location/Files/Live captures the panel chooses to keep server-side.

| Function | Role |
| --- | --- |
| `canAccessArtifacts` | Shared permission predicate for artifact endpoints |
| `handleSaveDeviceArtifacts` | Accepts locations/files/media bundles; upserts structured data; stores media blobs |
| `saveMediaItems` | Decrypts/decodes each media item, encrypts at rest when encryptor present, inserts `MediaFile` rows |
| `decodeMediaPayload` | Accepts multiple client encodings (raw base64, data URLs, nested fields) into bytes + MIME |
| `handleListDeviceArtifacts` | Lists metadata for locations/files/media by type |
| `handleExportDeviceArtifacts` | Excel export of artifact indexes (metadata, not full binaries) |

---

## `internal/dispatcher/dispatcher.go`

**File role.** Asynchronous execution engine between REST handlers and device WebSockets. Owns worker pool, offline queue drain, audit side effects, media persistence, and fluent command builders used by specialized REST routes.

### Types

| Type | Role |
| --- | --- |
| `CommandDispatcher` | Queue + workers + result map + dependencies |
| `CommandTask` | One unit of work: command, issuing admin, client fingerprint, result channel |
| `CommandResult` | Normalized outcome for API/status polling |
| `CommandBuilder` | Ergonomic constructors for common command types without repeating boilerplate |

### Functions

| Function | Role |
| --- | --- |
| `ForJSON` | Shapes a result for HTTP JSON including encoded data |
| `EncodeCommandData` | Critical for panel media: keeps UTF-8/JSON readable; base64-encodes binary so React can decode images/audio safely |
| `isBinaryPayload` | Magic-byte classifier (JPEG/PNG/GIF/WEBP/Ogg/MP4) feeding `EncodeCommandData` |
| `storeResult` | Caches latest result in memory for fast status polls |
| `NewCommandDispatcher` | Starts ten workers and the 5-second pending-command scanner |
| `worker` | Per-goroutine loop consuming the command queue until cancel |
| `ExecuteCommand` | Public enqueue API returning a channel the HTTP handler waits on |
| `executeCommand` | Full lifecycle: create audit pending → offline queue **or** send via hub → update audit → maybe store media → deliver result |
| `shouldStoreMedia` | Decides whether camera/mic responses become durable `MediaFile` rows |
| `storeMediaFile` | Names files, extracts audio from JSON wrappers, encrypts, checksums, inserts media |
| `looksLikeJSONObject` | Detects JSON acknowledgements that must not be stored as audio as-is |
| `extractAudioBytes` | Pulls base64 audio fields out of mic JSON payloads |
| `processPendingCommands` | Periodic online-device scan that drains Postgres offline queues |
| `processDeviceQueue` | Sends each pending command for one device; updates pending status |
| `GetCommandStatus` | Fallback chain: memory → Redis response → audit log |
| `BroadcastCommand` | Fan-out helper returning which devices were offline-queued |
| `NewCommandBuilder` | Starts a fluent builder bound to this dispatcher |
| `WithAdmin` / `WithContext` | Attach issuer + IP/UA for audit |
| `FileList` / `FileRead` / `FileReadChunk` / `FileDelete` | File-family builders |
| `GetContacts` / `GetCallLogs` / `GetDeviceInfo` / `GetLocation` | Data-family builders |
| `CameraSnapshot` / `MicStart` / `GetForegroundApp` | Sensor-family builders |
| `execute` | Shared builder finale: allocate transaction, enqueue, return result channel |

---

## `internal/dispatcher/encode_test.go`

**File role.** Guards encoding behavior that the panel depends on for clear image/JSON display.

| Function | Role |
| --- | --- |
| `TestEncodeCommandData_JPEGBase64` | Ensures JPEG bytes become reversible base64 strings |
| `TestEncodeCommandData_JSONObject` | Ensures JSON objects stay structured maps |
| `TestEncodeCommandData_PlainText` | Ensures plain text stays a string |

---

## `internal/cryptokit/`

**Package role.** CKX1 primitives (X25519, HKDF-SHA256, ChaCha20-Poly1305, Ed25519) and at-rest `AT1:` helpers. See [SESSION_ENCRYPTION.md](SESSION_ENCRYPTION.md).

| File / API | Role |
| --- | --- |
| `ckx1_algorithm.go` | Protocol constants / directions |
| `ckx1_canonical.go` | Length-prefixed encoding, fingerprints |
| `ckx1_kdf.go` | HKDF + directional key derivation |
| `ckx1_aead.go` | ChaCha20-Poly1305 seal/open |
| `ckx1_envelope.go` | Handshake transcript, frame AAD, `AT1:` |
| `x25519_keys.go` / `ed25519_identity.go` | Key load/generate/agree/sign/verify |
| `ckx1_vectors_test.go` | Interop / round-trip vectors |

## `internal/security/admin_ckx1.go` + `internal/api/ckx1_*.go`

**Package role.** Operator CKX1 sessions for panel ↔ server REST and admin WebSocket.

| API | Role |
| --- | --- |
| `POST /auth/ckx1/offer` / `/exchange` | Establish admin directional keys |
| `ckx1BodyMiddleware` | Requires `X-CKX1-Session`; opens request `enc`; seals JSON responses |
| `AdminCKX1Session.SealAdmin` / `OpenAdmin` | Server↔panel frame crypto |

---

## `internal/websocket/hub.go`

**File role.** Real-time connection fabric. Tracks which devices are online, correlates command replies by transaction ID, and pushes presence to admin browsers. Device and admin application frames are CKX1-encrypted after handshake.

### Types

| Type | Role |
| --- | --- |
| `Hub` | Central registry; holds server CKX1 identity (`SetCKX1Identity`) |
| `DeviceUpdate` | Admin-facing event payload |
| `AdminSession` | Operator socket; optional `EncryptOutbound` / `DecryptInbound` |
| `Client` | Agent socket + per-connection `ckx1DeviceSession` |
| `OnlineDeviceSnapshot` | Minimal online device view for REST merge |

### Functions

| Function | Role |
| --- | --- |
| `NewHub` | Allocates maps/channels/context |
| `SetCKX1Identity` / `GetCKX1Identity` | Server X25519 + Ed25519 identity for `key_offer` |
| `Run` | Event loop for register/unregister/broadcast |
| `Register` / `registerClient` | Indexes client; sends CKX1 `key_offer` (admin presence after session ready) |
| `SendCommandToDevice` | Requires ready session; seals command as `type=enc` |
| `broadcastUpdate` | Fan-out; seals with admin CKX1 when configured |
| `IsDeviceSessionReady` | Dispatcher gate before commands |
| `Client.ReadPump` / `handleMessage` | Demux `key_exchange` / `enc` |
| `UpgradeDeviceConnection` / `UpgradeAdminConnection` | HTTP→WS upgrade |

## `internal/websocket/ckx1_*.go`

**File role.** Device CKX1 handshake and session state.

| File | Role |
| --- | --- |
| `ckx1_handshake.go` | `key_offer` / verify `key_exchange` / `session_ready` |
| `ckx1_session.go` | Directional seal/open + sequence |
| `replay_guard.go` | Reject duplicate/older seq |
| `session_state.go` | `DeviceSession` interface |

---

## `internal/database/postgres.go`

**File role.** System of record for admins, devices, sessions, audit, pending commands, media, dashboard aggregates.

| Function | Role |
| --- | --- |
| `NewPostgresDB` | Opens and pings the pool |
| `Close` | Releases connections on shutdown |
| `InitSchema` | Creates tables/indexes/triggers on first boot (including audit immutability trigger that currently conflicts with response updates—documented in security notes) |
| `CreateAdministrator` / `GetAdministratorByUsername` / `GetAdministratorByID` | Admin identity store for login and middleware |
| `scanDevice` | Shared row mapper for device queries |
| `CreateDevice` / `GetDeviceByID` / `GetDeviceByUUID` / `GetAllDevices` / `UpdateDeviceStatus` / `DeleteDevice` | Device registry used by enrollment, hub, and panel lists |
| `CreateSession` / `GetSessionByRefreshToken` / `DeleteSession` / `DeleteSessionByRefreshToken` | Refresh-token session store |
| `CreateAuditLog` / `UpdateAuditLogResponse` / `GetAuditLogByTransactionID` / `SearchAuditLogs` | Command audit trail for compliance UI and status fallback |
| `CreatePendingCommand` / `GetPendingCommandsForDevice` / `UpdatePendingCommandStatus` | Offline command durability used by dispatcher scanner |
| `CreateMediaFile` / `GetMediaFileByID` / `GetMediaFilesByDevice` | Encrypted media blob store |
| `GetDashboardStats` | SQL aggregates for Dashboard |

---

## `internal/database/comms.go`

**File role.** Normalizes messy agent/panel JSON into relational contacts/SMS/call rows.

| Function | Role |
| --- | --- |
| `parseFlexibleTime` | Accepts ISO strings, epoch ms/s, and other shapes into `time.Time` |
| `millisOrSeconds` | Heuristic epoch unit conversion |
| `asString` / `asBool` / `asInt` | Loose JSON coercion |
| `smsTypeLabel` | Human labels for SMS type codes |
| `allPhonesFromContact` | Flattens varied phone field layouts |
| `UpsertDeviceContacts` / `UpsertDeviceSMS` / `UpsertDeviceCallLogs` | Idempotent persistence from save/mirror paths |
| `ListDeviceContacts` / `ListDeviceSMS` / `ListDeviceCallLogs` | Bounded reads for list/export APIs |
| `CountDeviceComms` | Counters for list responses |

---

## `internal/database/artifacts.go`

**File role.** Locations, file inventory, and media listing persistence.

| Function | Role |
| --- | --- |
| `asFloat64` / `asFloatPtr` | Numeric coercion for lat/lng/accuracy |
| `UpsertDeviceLocations` / `ListDeviceLocations` | Location history store |
| `UpsertDeviceFileEntries` / `ListDeviceFileEntries` | File inventory store |
| `ListDeviceMedia` | Media metadata by type |
| `CountDeviceArtifacts` | Counters for artifact list API |

---

## `internal/database/redis.go`

**File role.** Optional acceleration layer. Presence and session helpers are used; Redis command queues are **not** the path the offline processor currently drains.

| Function | Role |
| --- | --- |
| `NewRedisCache` / `Close` | Connect/ping/close |
| `SetDeviceOnline` / `SetDeviceOffline` / `GetOnlineDevice` / `GetOnlineDevices` / `IsDeviceOnline` | Presence cache |
| `UpdateDeviceHeartbeat` | Battery + last-seen updates from agent heartbeats |
| `EnqueueCommand` / `DequeueCommand` / `GetQueuedCommandCount` | Sorted-set queue helpers (available, lightly wired) |
| `StoreCommandResponse` / `GetCommandResponse` / `DeleteCommandResponse` | Short-TTL response cache for status polling |
| `CheckRateLimit` | Counter primitive not attached as HTTP middleware |
| `CacheSession` / `GetCachedSession` / `InvalidateSession` | Session cache for login/logout paths |

---

## `internal/security/security.go`

**File role.** Cryptography and authorization primitives used across API and dispatcher.

| Symbol | Role |
| --- | --- |
| `JWTManager` / `NewJWTManager` | Access/refresh token factory with issuer binding |
| `GenerateTokenPair` | Issues login tokens embedding admin identity/role |
| `ValidateAccessToken` | Middleware validation |
| `ValidateRefreshToken` | Refresh endpoint validation returning admin ID |
| `GetAdminIDFromClaims` | Extracts UUID from claims map |
| `TokenPair` | Transport struct for tokens + expiry |
| `PasswordHasher` / `NewPasswordHasher` / `HashPassword` / `VerifyPassword` | bcrypt for admin passwords |
| `DataEncryptor` / `NewDataEncryptor` / `Encrypt` / `Decrypt` | AES-GCM for media at rest |
| `GenerateChecksum` | SHA-256 integrity field on media rows |
| `CertificateManager` (server) / `GenerateCACertificate` / `GenerateServerCertificate` | TLS material generation utilities |
| `CreateServerTLSConfig` / `CreateClientTLSConfig` | Load TLS configs for HTTPS |
| `PermissionChecker` / `NewPermissionChecker` | Role→permission matrix (`super_admin` / `admin` / `operator`) |
| `HasPermission` / `HasAnyPermission` | Wildcard-aware authorization checks used by nearly every handler |
| `APIKeyAuth` + generate/validate/revoke/load/export | Alternate in-memory API key scheme (not primary device auth) |
| `InputValidator` / `NewInputValidator` / `ValidateDeviceUUID` / `SanitizeString` / `ValidateCommandType` | Input hygiene and command allowlist for `handleExecuteCommand` |
| `parseIPs` / `parseDNSNames` | SAN helpers for certificate generation |

---

## `internal/security/tls.go`

**File role.** Lower-level certificate PEM/RSA helpers for development TLS.

| Function | Role |
| --- | --- |
| `generateRSAKey` | Creates RSA private keys for cert tooling |
| `encodePrivateKey` | PEM-encodes private keys |
| `EncodeCertificatePEM` | PEM-encodes certificate DER |
| `GenerateSelfSignedCertificate` | Produces self-signed server cert/key for local HTTPS experiments |
| `osReadFile` | Thin file read wrapper for cert loading paths |

---

## `internal/models/*.go`

**File role.** Shared typed contracts between packages—no business logic.

### `types.go`

| Type | Role |
| --- | --- |
| `Device` | Enrolled device record (IDs, hardware, status, battery, cert hash) |
| `Administrator` | Operator account |
| `AuditLog` | Per-command audit row |
| `PendingCommand` | Offline queue row |
| `MediaFile` | Encrypted media blob metadata + ciphertext |
| `Session` | Refresh session |

### `protocol.go`

| Symbol | Role |
| --- | --- |
| `CommandType` constants | Canonical wire names shared conceptually with the agent |
| `ResponseStatus` constants | Terminal/non-terminal command states |
| `DeviceCommand` | Outbound command envelope to agents |
| `AgentResponse` | Correlated inbound result |
| `FileInfo`, `Contact`, `Phone`, `Email`, `CallLogEntry`, `ForegroundApp`, `DeviceInfo`, `Location` | Structured payload shapes |

### `api.go`

REST request/response DTOs: login, refresh, commands, device lists, search, enroll, dashboard, errors, health.

### `comms.go` / `artifacts.go`

Stored-row structs and save/list result DTOs for the persistence APIs.

---

## `server/scripts/*`

| File | Role |
| --- | --- |
| `init-database.ps1` | Creates `remote_access` role and `android_remote_access` database for local Postgres |
| `init-database.sql` | SQL statements companion for the same setup |
| `grant-and-extension.sql` | Grants privileges / enables extensions needed by the app schema |

These are **development tooling**, not part of the running HTTP process.

---

## Cross-cutting incompleteness (development truth)

- Transfer APIs are empty stubs; real download state lives in the panel IndexedDB.
- Several admin/audit-detail handlers are placeholders.
- Mic `stop` action REST path is stubbier than the agent’s `mic_stop` command.
- Camera stream is not implemented.
- Redis offline queues are not the scanner’s source of truth (Postgres is).
- Device WS upgrade is not credential-gated; authenticated key exchange is required before `session_ready`. TLS client certificates are requested but not enforced as mTLS.
