# Android agent — deep functional reference (every file & function)

Behavior-only documentation for first-party sources under `android-agent/` (excluding `.gradle/` caches and `app/build/` outputs). See also [PROJECT_FILE_CATALOG.md](PROJECT_FILE_CATALOG.md) and [android-agent/README.md](../android-agent/README.md).

---

## Development role of the agent

The agent is the **device-side executor**: a visible foreground service that enrolls with the Go server, maintains a WebSocket, answers heartbeats, and runs permission-gated platform operations when commanded. It is not stealthy; notification text discloses that the management connection is active.

---

## Build & tooling files

| File | Role in the project |
| --- | --- |
| `settings.gradle.kts` | Includes the `:app` module |
| `build.gradle.kts` (root) | Declares Android/Gradle plugin versions for the project |
| `gradle.properties` | JVM args and AndroidX flags for builds |
| `gradle/wrapper/gradle-wrapper.properties` | Pins Gradle wrapper distribution (8.7) |
| `app/build.gradle.kts` | Application ID, SDK 26/34, Java 17, OkHttp + BouncyCastle + AppCompat dependencies, packaging excludes |
| `local.properties` | Machine-local Android SDK path (developer machine) |
| `README.md` | Human agent guide + command matrix |
| `emulator-forward.ps1` | ADB reverse helper colocated with the agent |
| `set-emulator-location.ps1` | Injects a test GPS fix into the emulator for location command verification |

---

## Manifests & resources

### `app/src/main/AndroidManifest.xml`

**Role.** Declares `AgentApplication`, launcher `MainActivity`, foreground `AgentService`, boot/network receivers, and the full permission set (network, FGS, camera, mic, contacts, call log, SMS, location, storage/media, notifications, package query, usage stats, boot). References network security config.

### `app/src/debug/AndroidManifest.xml`

**Role.** Debug-variant merge point for development-only overrides.

### Resources

| File | Role |
| --- | --- |
| `res/layout/activity_main.xml` | Enrollment form layout: server URL, credentials, IDs, connect/disconnect, status |
| `res/values/strings.xml` | UI + foreground notification copy |
| `res/values/themes.xml` | App theme |
| `res/values/colors.xml` | Color tokens |
| `res/xml/network_security_config.xml` | Allows cleartext only to emulator/localhost targets for local testing |
| `res/drawable/ic_notification.xml` | Foreground notification icon |
| `res/drawable/ic_launcher_foreground.xml` | Launcher foreground vector |
| `res/mipmap-anydpi-v26/ic_launcher.xml` / `ic_launcher_round.xml` | Adaptive icons |

---

## `AgentApplication.java`

**File role.** Process-wide application object.

| Method | Role |
| --- | --- |
| `onCreate` | Captures singleton; creates the notification channel required before starting a foreground service on modern Android |
| `getInstance` | Lets services/activities reach application-scoped helpers |
| `createNotificationChannel` | Registers the channel used by the ongoing “agent active” notification |
| `getForegroundNotification` | Builds the persistent notification that makes the agent’s connection **visible** to the device user |

---

## `config/AgentPreferences.java`

**File role.** Durable local settings bridging UI and service.

| Method | Role |
| --- | --- |
| constructor | Opens SharedPreferences for the agent |
| `getServerBaseUrl` | Returns configured Go server base URL |
| `getServerDeviceId` / `setServerDeviceId` | Persists the **server-assigned** UUID required in the WS path after enrollment |
| `isAutoStartEnabled` / `setAutoStartEnabled` | Controls whether Boot/Network receivers should relaunch the service |
| `normalizeBaseUrl` | Trims and normalizes URLs entered by operators |
| `buildWebSocketUrl` | Converts HTTP(S) base + server device ID into `ws(s)://…/ws/devices/{id}` for `AgentService` |

---

## `util/DeviceEnv.java`

**File role.** Environment heuristics for smoother emulator onboarding.

| Method | Role |
| --- | --- |
| private constructor | Utility class guard |
| `isEmulator` | Detects emulator Build properties so defaults can differ from physical devices |
| `defaultLocalServerUrl` | Suggests a localhost/emulator-friendly server URL in MainActivity |

---

## `ui/MainActivity.java`

**File role.** Human-facing enrollment and permission UX; starts/stops the service.

### Lifecycle

| Method | Role |
| --- | --- |
| `onCreate` | Inflates UI, binds controls, loads agent UUID and prefs, registers connection broadcast receiver |
| `onStart` / `onStop` | Bind/unbind to `AgentService` for live status; manage polling |
| `onDestroy` | Unregister receivers; stop timers |

### Enrollment / service control

| Method | Role |
| --- | --- |
| `enrollAndConnect` | End-to-end onboarding: request permissions → REST login → REST enroll → store server device ID → start service |
| `startAgentService` | Launches foreground service with the server device ID needed to connect |
| `stopAgentService` | Stops connection and service when operator disconnects |
| `bindAgentService` | Local bind for `isConnected` queries |
| `isValidServerDeviceId` | Validates UUID format before connecting |
| `loadAgentUuid` | Loads or creates the long-lived agent UUID via `KeyStoreIdentity` / preferences |
| `refreshConnectionStatus` | Updates UI from binder/broadcast state |
| `startConnectionPolling` / `stopConnectionPolling` | Periodic UI refresh while activity visible |
| `setStatus` | Renders connected/disconnected messaging |

### Permissions

| Method | Role |
| --- | --- |
| `requestOperationalPermissions` | Batches runtime permissions required by command families (camera, mic, contacts, call log, SMS, location, notifications, media/storage) |
| `addIfMissing` | Avoids re-requesting already-granted permissions |
| `maybeRequestUsageAccess` | Guides user to Usage Access settings needed for foreground-app command |
| `onRequestPermissionsResult` | Continues or aborts enrollment after the system permission dialog |

### Nested collaborators

| Symbol | Role |
| --- | --- |
| `connectionReceiver` | Receives service broadcasts of connection state changes |
| `serviceConnection` | Obtains `LocalBinder` when bound |
| `uiHandler` | Main-thread scheduling for polling |

---

## `service/AgentService.kt`

**File role.** The long-running device endpoint of the product. Establishes session encryption before any application payload.

| Method / type | Role |
| --- | --- |
| `LocalBinder` / `getService` | Allows MainActivity to query connection state without IPC complexity |
| `onCreate` | Initializes preferences, command handler, serial `messageExecutor` + `commandExecutor` |
| `onStartCommand` | Enters foreground, starts connection; returns `START_STICKY` so Android may recreate after kill |
| `onBind` | Returns local binder |
| `startForegroundService` | Shows the ongoing disclosure notification |
| `startConnection` / `stopConnection` | Public connect/disconnect controls; clears session crypto on stop |
| `connectToServer` | Builds OkHttp client (TLS when `wss`), opens `ServerConnection`; waits for `key_offer` (does not enroll yet) |
| `scheduleReconnect` | 5-second backoff reconnect while service wants to stay online |
| `startHeartbeat` / `sendHeartbeat` | 30-second keepalive with battery level, sealed after `session_ready` |
| `sendEncrypted` | Seals inner JSON via `Ckx1Session` as `type=enc` |
| `getBatteryLevel` | Reads sticky battery intent for heartbeats |
| `broadcastConnectionState` | Notifies MainActivity UI |
| `handleKeyOffer` | CKX1: verify fingerprint, sign transcript, send `key_exchange` |
| `sendEnrollment` | Emits inner `type: enrollment` metadata after session confirm |
| `handleServerMessage` | Serial demux: `key_offer` / `session_ready` / `enc` / reject plaintext |
| `handleApplicationMessage` | Parses inner command JSON; dispatches to `commandExecutor` |
| `executeCommand` | Delegates to `CommandHandler`, then sends encrypted response |
| `sendCommandResponse` | Builds inner `type: response` then seals |
| `loadAgentDeviceUuid` | Supplies `X-Device-UUID` header (connection candidate identity) |
| `isConnected` / `getDeviceUUID` | Status accessors (`isConnected` requires `sessionReady`) |
| `onDestroy` | Tears down heartbeat, socket, executors |

## `cryptokit/`

**Package role.** CKX1 (X25519 + HKDF-SHA256 + ChaCha20-Poly1305 + Ed25519). Interop: [crypto-kit/docs/interop.md](../crypto-kit/docs/interop.md). Protocol: [SESSION_ENCRYPTION.md](SESSION_ENCRYPTION.md).

| Type | Role |
| --- | --- |
| `Ckx1Algorithm` / `Ckx1Canonical` / `Ckx1Kdf` / `Ckx1Aead` | Constants, encoding, HKDF, AEAD |
| `X25519Keys` / `Ed25519Identity` | Agreement and transcript signatures |
| `Ckx1Handshake` / `Ckx1Session` / `ReplayGuard` | Handshake + directional frames |
| `KeyStoreIdentity` | Long-term device X25519 + Ed25519 keys |

Frame AAD: length-prefixed `CKX1-FRAME-V1` + session/device/dir/seq/txn.

The URL and `X-Device-UUID` identify a connection candidate. The device is not authenticated until enrolled public keys and the Ed25519 handshake signature are verified. Commands are rejected until `session_ready`.

---

## Networking

### `network/EnrollmentClient.java`

**File role.** REST client for the two enrollment steps before WebSocket is possible.

| Method / type | Role |
| --- | --- |
| constructor | Holds OkHttp client + base URL |
| `login` | `POST /api/v1/auth/login` with operator credentials; returns access token used only for the enroll call |
| `enrollDevice` | `POST /api/v1/devices` with agent UUID + hardware + cert fingerprint; returns **server device ID** that becomes the WS path segment |
| `verifyServerReachable` | `GET /health` preflight so users get a clear “server down” error before auth |
| `EnrollmentException` | Typed error for UI messaging |
| URL normalize helper | Ensures consistent base URL formatting |

### `network/ServerConnection.java`

**File role.** Thin OkHttp WebSocket adapter used by the service.

| Method | Role |
| --- | --- |
| constructor | Configures URL, headers (`X-Device-UUID`), listener, client |
| `connect` | Opens socket; maps OkHttp callbacks to listener |
| `postToListener` | Ensures listener callbacks run on an appropriate thread |
| `disconnect` | Clean close |
| `send` | Outbound text frames (enrollment, heartbeat, responses) |
| `isConnected` | Connection flag |

---

## Protocol

### `protocol/DeviceCommand.java`

**File role.** Deserializes server command envelopes and exposes payload accessors.

| Symbol | Role |
| --- | --- |
| `CommandType` enum + `getValue` / `fromString` | Canonical command names; includes declared-but-unimplemented types for forward compatibility |
| constructors | Empty / default construction |
| `fromJSON` / `toJSON` | Wire parse/serialize |
| getters/setters | transaction id, type, payload, timeout, issued_at |
| `getString` / `getInt` / `getLong` / `getBoolean` | Typed reads from payload JSON with defaults—used heavily by executors |

### `protocol/CommandResult.java`

**File role.** Uniform result object returned to `AgentService` for encoding onto the wire.

| Symbol | Role |
| --- | --- |
| `Status` enum | success/failed/timeout/pending/partial |
| constructors | Empty / status-initialized |
| `success` overloads | Build successful results from bytes or text |
| `failed` overloads | Build failures with optional error codes |
| `timeout` / `pending` / `partial` | Non-success factories |
| getters/setters | status, data, error fields |
| `getDataAsString` / `setDataAsString` | UTF-8 helpers |
| `isSuccess` / `isFailed` / `isTimeout` | Branching helpers |

### `protocol/CommandHandler.java`

**File role.** The device capability matrix in executable form. Only registered executors run; others fail as unknown.

| Method | Role |
| --- | --- |
| constructor | Stores Context; builds executor map |
| `registerDefaultExecutors` | Registers the implemented command set (files, contacts, calls, SMS, device info, foreground app, installed apps, location, camera snapshot, mic start/stop) |
| `handleCommand` | Looks up executor; runs; converts exceptions to failed results |

#### File executors

| Method | Role |
| --- | --- |
| `executeFileList` | Lists immediate children of a path for the Files UI / mirror walker |
| `getPermissionsString` | Human rwx-style permissions string in listing metadata |
| `executeFileRead` | Whole-file read with size/directory guards (10 MiB max) |
| `executeFileReadChunk` | Bounded range read powering server `/files/stream` and large downloads (max 512 KiB request) |
| `executeFileDelete` | Deletes a path when writable |
| `executeGetDirectory` | Directory listing alias |

#### Communications executors

| Method | Role |
| --- | --- |
| `executeGetContacts` | Requires `READ_CONTACTS`; returns contact graph for Contacts page |
| `executeGetCallLogs` | Requires `READ_CALL_LOG`; limited history for call tab |
| `getCallTypeString` | Maps Android call-type ints to labels |
| `executeGetSmsMessages` | Requires `READ_SMS`; respects payload filters for inbox/sent style pulls |

#### Device / sensors / media executors

| Method | Role |
| --- | --- |
| `executeGetDeviceInfo` | Build/model/battery/storage snapshot for Live View / console |
| `getBatteryStatus` | Charging status label |
| `executeGetForegroundApp` | Usage-stats based recent app (needs Usage Access) |
| `executeGetInstalledApps` | Package list (subject to package visibility) |
| `executeGetLocation` | Permission-gated location; prefers fresh fix |
| `requestFreshLocation` | Single-update listener with timeout |
| `pickBestLastKnown` | Fallback among last-known provider fixes |
| `executeCameraSnapshot` | Delegates to Camera2 JPEG helper |
| `executeMicStart` | Timed record or start-mode recording |
| `executeMicStop` | Stops recording; returns JSON-wrapped base64 audio for server media storage |
| `hasPermission` | Runtime permission gate used before sensitive APIs |

---

## Media helpers

### `media/CameraCaptureHelper.java`

| Method | Role |
| --- | --- |
| private constructor | Utility guard |
| `captureJpeg` | Full Camera2 open → session → still capture → JPEG bytes for `camera_snapshot` |
| `chooseCameraId` | Front/back lens selection |
| `chooseSize` | Picks a practical capture size from characteristics |

### `media/AudioCaptureHelper.java`

| Method | Role |
| --- | --- |
| private constructor | Utility guard |
| `isRecording` | Whether a start/stop session is active |
| `startRecording` | Begins MediaRecorder to a temp file |
| `stopRecording` | Finalizes and returns recorded bytes |
| `recordOgg` | Convenience timed capture used by duration-based `mic_start` |
| `readFile` | Reads temp recording into memory |

---

## Security helpers

### `cryptokit/KeyStoreIdentity.kt`

| Method | Role |
| --- | --- |
| load/generate | Long-term X25519 + Ed25519 identity; private material Keystore-wrapped |
| public key accessors | Enrollment + handshake wire encoding (raw 32-byte base64) |

### `security/TLSConnectionFactory.kt`

| Method | Role |
| --- | --- |
| `createSSLContext` / trust helpers | Supplies OkHttp HTTPS/WSS configuration (transport TLS; not RSA client certs) |

**Development note:** Mutual TLS client-certificate verification is **not** enforced by the current server WS handler; authentication is CKX1 after upgrade.

---

## Receivers

### `receiver/BootReceiver.java`

| Method | Role |
| --- | --- |
| `onReceive` | After boot, if auto-start enabled and a server device ID is stored, starts `AgentService` so enrolled devices reconnect without opening the UI |

### `receiver/NetworkReceiver.java`

| Method | Role |
| --- | --- |
| `onReceive` | On connectivity restore, restarts/reconnects the service when auto-start is enabled |
| `isNetworkAvailable` | Checks active network before attempting restart |

---

## End-to-end development flows

### Identity

| ID | Created by | Consumed by |
| --- | --- | --- |
| Agent UUID | Preferences / `KeyStoreIdentity` | Enrollment body + `X-Device-UUID` |
| Server device ID | `POST /api/v1/devices` | WS path + panel device list |

### Enrollment → online

```text
MainActivity.enrollAndConnect
  → EnrollmentClient.verifyServerReachable / login / enrollDevice
  → AgentPreferences.setServerDeviceId
  → AgentService.start → connectToServer
  → key_offer → key_exchange → session_ready
  → enc(enrollment) → enc(heartbeats)
```

### Command execution

```text
Server Hub.SendCommandToDevice (seal type=enc)
  → AgentService.handleServerMessage → open enc
  → CommandHandler.handleCommand → platform APIs
  → sendCommandResponse sealed as enc
  → Hub open enc → HandleDeviceResponse → dispatcher → panel
```

---

## Declared vs registered commands

`DeviceCommand.CommandType` may list commands the handler does **not** register (file write/rename/move/upload/download, browser history, camera stream/stop, mic stream). Those fail as unknown at runtime. The registered set in `registerDefaultExecutors` is the operational truth for development.
