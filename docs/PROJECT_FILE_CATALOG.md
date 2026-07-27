# Complete first-party file catalog

Inventory of **project-owned** files only. Excludes `node_modules/`, `.gradle/` caches, `build/` / `dist/` outputs, IDE metadata, and third-party lock artifacts beyond naming them as config roots.

Use this catalog to verify coverage, then open the matching `*_FUNCTIONAL_REFERENCE.md` for per-function depth.

---

## Root

| File | Role in the project |
| --- | --- |
| `README.md` | Top-level status, docs map, prerequisites, quick start, privacy boundary |
| `scripts/emulator-port-forward.ps1` | ADB reverse of host `8443` into emulator so agent URL `http://127.0.0.1:8443` reaches the Go server |

---

## `docs/` (documentation set)

| File | Role |
| --- | --- |
| `docs/README.md` | Documentation index and reading order |
| `docs/ARCHITECTURE.md` | Runtime topology and data-flow narrative |
| `docs/SESSION_ENCRYPTION.md` | Device WebSocket session crypto contract |
| `docs/ENCRYPTION.md` | Session + at-rest encryption coverage |
| `docs/API_REFERENCE.md` | HTTP/WebSocket contract and endpoint maturity |
| `docs/WEB_PANEL.md` | Operator UI overview |
| `docs/DEVELOPMENT.md` | Contributor build/validation workflow |
| `docs/LOCAL_TESTING.md` | Emulator-only test procedure |
| `docs/SECURITY_AND_PRIVACY.md` | Controls, gaps, release blockers |
| `docs/COMPONENT_FUNCTIONAL_INDEX.md` | Entry to deep functional refs |
| `docs/PROJECT_FILE_CATALOG.md` | This catalog |
| `docs/SERVER_FUNCTIONAL_REFERENCE.md` | Server file/function depth |
| `docs/WEB_PANEL_FUNCTIONAL_REFERENCE.md` | Panel file/function depth |
| `docs/ANDROID_AGENT_FUNCTIONAL_REFERENCE.md` | Agent file/function depth |

---

## `server/` — Go service

| File | Role |
| --- | --- |
| `server/go.mod` | Module path, Go 1.21, direct dependencies |
| `server/go.sum` | Dependency checksum lock |
| `server/config.yaml` | Local development YAML (secrets are placeholders; not for production) |
| `server/cmd/main.go` | Process entry: config, deps, HTTP serve, shutdown, default admin |
| `server/cmd/config.go` | Config structs + YAML loader with defaults |
| `server/internal/api/handlers.go` | Server/Config/NewServer/Router/setupRoutes (core wiring) |
| `server/internal/api/middleware.go` | JWT auth middleware + admin context helpers |
| `server/internal/api/response.go` | Health, writeJSON/writeError, media content-type helpers |
| `server/internal/api/auth.go` | Login / refresh / logout |
| `server/internal/api/devices.go` | Device list/get/enroll/delete/status |
| `server/internal/api/commands.go` | Execute / status / cancel commands |
| `server/internal/api/files.go` | File list/read/delete/download |
| `server/internal/api/contacts.go` | Live contacts pull |
| `server/internal/api/call_logs.go` | Live call-log pull |
| `server/internal/api/media.go` | Stored media list/download |
| `server/internal/api/actions_camera.go` | Camera action API |
| `server/internal/api/actions_mic.go` | Microphone action API |
| `server/internal/api/actions_location.go` | Location + foreground-app actions |
| `server/internal/api/transfers.go` | Transfer list stubs |
| `server/internal/api/audit.go` | Audit search/get |
| `server/internal/api/dashboard.go` | Dashboard stats |
| `server/internal/api/admins.go` | Admin CRUD stubs |
| `server/internal/api/websocket.go` | Device + admin WebSocket upgrades |
| `server/internal/api/file_stream.go` | Chunked remote file HTTP stream aggregator |
| `server/internal/api/mirrors.go` | In-memory mirror snapshots + optional DB persist |
| `server/internal/api/comms.go` | Save/list/Excel-export contacts, SMS, calls |
| `server/internal/api/artifacts.go` | Save/list/Excel-export locations, files, media |
| `server/internal/dispatcher/dispatcher.go` | Command worker pool, offline queue, media store, builders |
| `server/internal/dispatcher/encode_test.go` | Unit tests for command payload encoding |
| `server/internal/cryptokit/ckx1_*.go` | CKX1 algorithm, KDF, AEAD, envelopes, vectors |
| `server/internal/cryptokit/x25519_keys.go` | X25519 load/generate/agree |
| `server/internal/cryptokit/ed25519_identity.go` | Ed25519 sign/verify |
| `server/internal/api/ckx1_admin.go` | Admin CKX1 offer/exchange |
| `server/internal/api/ckx1_transport.go` | Admin REST enc middleware |
| `server/internal/security/admin_ckx1.go` | Admin CKX1 session store |
| `server/internal/websocket/hub.go` | Device/admin WS hub, presence, command correlation, enc demux |
| `server/internal/websocket/ckx1_handshake.go` | Device CKX1 handshake |
| `server/internal/websocket/ckx1_session.go` | Device CKX1 seal/open |
| `server/internal/websocket/session_test.go` | Session handshake and reject-path unit tests |
| `server/.gitignore` | Ignores `data/` and private key material |
| `server/internal/database/postgres.go` | Schema + core CRUD |
| `server/internal/database/comms.go` | Comms upsert/list helpers |
| `server/internal/database/artifacts.go` | Artifact upsert/list helpers |
| `server/internal/database/redis.go` | Optional Redis cache/queue/session helpers |
| `server/internal/security/security.go` | JWT, bcrypt, AT1 at-rest, permissions, validation, TLS helpers |
| `server/internal/security/tls.go` | Self-signed cert generation and PEM helpers |
| `server/internal/models/types.go` | Domain entity structs |
| `server/internal/models/protocol.go` | Device command/response protocol types |
| `server/internal/models/api.go` | REST DTO structs |
| `server/internal/models/comms.go` | Stored comms + save DTOs |
| `server/internal/models/artifacts.go` | Stored artifacts + save DTOs |
| `server/scripts/init-database.ps1` | Creates Postgres role/DB for local setup |
| `server/scripts/init-database.sql` | SQL companion for DB init |
| `server/scripts/grant-and-extension.sql` | Grants/extensions for app role |

---

## `web-panel/` — React control panel

| File | Role |
| --- | --- |
| `web-panel/package.json` | npm scripts/deps for Vite React app |
| `web-panel/package-lock.json` | Locked dependency tree |
| `web-panel/vite.config.js` | Dev server port 3000; proxies `/api` and `/ws` to Go |
| `web-panel/index.html` | SPA HTML shell |
| `web-panel/src/main.jsx` | React mount |
| `web-panel/src/main.css` | CSS entry import |
| `web-panel/src/App.jsx` | Auth, routes, protected layout |
| `web-panel/src/api/client.js` | Core HTTP client, IndexedDB, auth, commands; attaches domain APIs |
| `web-panel/src/api/errors.js` | `ApiError` type |
| `web-panel/src/api/devices.js` | Device registry API methods |
| `web-panel/src/api/files.js` | File list/read/stream download APIs |
| `web-panel/src/api/contacts.js` | Live contacts API |
| `web-panel/src/api/records.js` | Live call-logs API |
| `web-panel/src/api/camera.js` | Camera snapshot API |
| `web-panel/src/api/mic.js` | Mic start/stop API |
| `web-panel/src/api/location.js` | Location command API |
| `web-panel/src/api/comms.js` | Persist/list/export communications |
| `web-panel/src/api/artifacts.js` | Persist/list/export artifacts |
| `web-panel/src/api/media.js` | Stored media download API |
| `web-panel/src/api/mirrors.js` | Mirror snapshot APIs |
| `web-panel/src/api/transfers.js` | Transfer stub APIs |
| `web-panel/src/api/audit.js` | Audit + dashboard APIs |
| `web-panel/src/context/DeviceContext.jsx` | Shared device list + online filter |
| `web-panel/src/hooks/useDevices.js` | Filtered device helper hook |
| `web-panel/src/hooks/useAdminWebSocket.js` | Admin WS connection state hook |
| `web-panel/src/hooks/useDebouncedValue.js` | Debounce hook for search inputs |
| `web-panel/src/hooks/usePanelLayout.js` | Persist resizable pane widths |
| `web-panel/src/hooks/useDeviceComms.js` | Contacts/SMS/calls bootstrap, sync, DB persist |
| `web-panel/src/hooks/useMediaSession.js` | Live View capture gallery session |
| `web-panel/src/hooks/useCameraCapture.js` | Camera capture busy/state hook |
| `web-panel/src/hooks/useMicRecording.js` | Mic recording start/stop hook |
| `web-panel/src/lib/adminWebSocket.js` | Shared admin WS singleton |
| `web-panel/src/lib/commandRunner.js` | Run/poll/parse device commands |
| `web-panel/src/lib/mirror.js` | Local/server file-tree mirrors |
| `web-panel/src/lib/transfers.js` | Client-side download queue |
| `web-panel/src/lib/largeFileDownload.js` | Chunked file assembly |
| `web-panel/src/lib/paths.js` | Android path normalization |
| `web-panel/src/lib/fileBrowserUi.js` | File UI sort/filter/labels |
| `web-panel/src/lib/media.js` | Base64/media data-URL helpers |
| `web-panel/src/lib/download.js` | Shared `downloadBlob` helper |
| `web-panel/src/lib/deviceInterfaces.js` | Nav chips for device interfaces |
| `web-panel/src/features/comms/*.js` | Contacts, SMS, call-log, phone helpers |
| `web-panel/src/features/live/*.js` | Camera/mic capture helpers + formatters |
| `web-panel/src/features/location/*.js` | Geocode + map sync helpers |
| `web-panel/src/components/Sidebar.jsx` | Main navigation |
| `web-panel/src/components/Layout.jsx` | Layout shell helper |
| `web-panel/src/components/DevicePicker.jsx` | Device select control |
| `web-panel/src/components/LoadingScreen.jsx` | Loading/empty/toast/button primitives |
| `web-panel/src/components/ui/Icon.jsx` | Lucide icon name map |
| `web-panel/src/components/hybrid/HybridControls.jsx` | Hybrid button/empty/loading controls |
| `web-panel/src/components/hybrid/FileEntryViews.jsx` | File list/grid/details UI |
| `web-panel/src/components/hybrid/TruncatedText.jsx` | Ellipsis text component |
| `web-panel/src/components/comms/*.jsx` | Contacts suite sidebar/list/detail/thread |
| `web-panel/src/components/live/*.jsx` | Live View camera/mic/stage/strip/lightbox |
| `web-panel/src/components/files/FileTree.jsx` | File browser tree navigation |
| `web-panel/src/components/location/MapPanel.jsx` | Leaflet map panel |
| `web-panel/src/pages/Login.jsx` | Login form |
| `web-panel/src/pages/Dashboard.jsx` | Stats dashboard |
| `web-panel/src/pages/Devices.jsx` | Device registry table |
| `web-panel/src/pages/DeviceDetail.jsx` | Single-device overview |
| `web-panel/src/pages/Files.jsx` | File browser page shell |
| `web-panel/src/pages/Downloads.jsx` | Transfer manager |
| `web-panel/src/pages/Orders.jsx` | Structured command orders UI |
| `web-panel/src/pages/CommandConsole.jsx` | Alternate command console UI (module catalog) |
| `web-panel/src/pages/Location.jsx` | Map + location history page shell |
| `web-panel/src/pages/ContactsSms.jsx` | Contacts / SMS / calls page orchestrator |
| `web-panel/src/pages/LiveView.jsx` | Camera/mic page orchestrator |
| `web-panel/src/pages/AuditLogs.jsx` | Audit search UI |
| `web-panel/src/pages/Settings.jsx` | Account/session/about |
| `web-panel/src/styles/*.css` | Visual stylesheets (no runtime logic) |

---

## `android-agent/` — Android companion

| File | Role |
| --- | --- |
| `android-agent/README.md` | Agent build, lifecycle, command matrix |
| `android-agent/settings.gradle.kts` | Gradle multi-project settings (`:app`) |
| `android-agent/build.gradle.kts` | Root Gradle plugins |
| `android-agent/gradle.properties` | JVM/Android Gradle properties |
| `android-agent/gradle/wrapper/gradle-wrapper.properties` | Wrapper Gradle version |
| `android-agent/app/build.gradle.kts` | App SDK versions, deps (OkHttp, BouncyCastle, AppCompat) |
| `android-agent/local.properties` | Local SDK path (machine-specific; not for commit of secrets) |
| `android-agent/emulator-forward.ps1` | Agent-local ADB reverse helper |
| `android-agent/set-emulator-location.ps1` | Sets emulator GPS for location testing |
| `android-agent/app/src/main/AndroidManifest.xml` | Components + permissions |
| `android-agent/app/src/debug/AndroidManifest.xml` | Debug manifest merge |
| `android-agent/app/src/main/kotlin/.../AgentApplication.kt` | App singleton + notification channel |
| `.../ui/MainActivity.kt` | Enrollment UI + permission requests |
| `.../service/AgentService.kt` | Foreground WS service + session handshake |
| `.../cryptokit/Ckx1*.kt` | CKX1 algorithm, KDF, AEAD, handshake, session |
| `.../cryptokit/X25519Keys.kt` / `Ed25519Identity.kt` | Agreement + signatures |
| `.../cryptokit/KeyStoreIdentity.kt` | Long-term device keys |
| `.../config/AgentPreferences.kt` | Server URL / device ID prefs |
| `.../util/DeviceEnv.kt` | Emulator detection + default URL |
| `.../network/EnrollmentClient.kt` | REST login + enroll |
| `.../network/ServerConnection.kt` | OkHttp WebSocket |
| `.../protocol/DeviceCommand.kt` | Inbound command model |
| `.../protocol/CommandResult.kt` | Outbound result model |
| `.../protocol/CommandHandler.kt` | Command executors |
| `.../media/CameraCaptureHelper.kt` | JPEG capture |
| `.../media/AudioCaptureHelper.kt` | Audio record helpers |
| `.../security/TLSConnectionFactory.kt` | SSLContext for wss |
| `.../receiver/BootReceiver.kt` | Boot auto-start |
| `.../receiver/NetworkReceiver.kt` | Network-restore reconnect |
| `.../res/layout/activity_main.xml` | Main UI layout |
| `.../res/values/strings.xml` | Strings / notification text |
| `.../res/values/themes.xml` | Theme |
| `.../res/values/colors.xml` | Colors |
| `.../res/xml/network_security_config.xml` | Dev cleartext allowlist |
| `.../res/drawable/*` | Icons |
| `.../res/mipmap-anydpi-v26/*` | Adaptive launcher icons |

---

## `crypto-kit/` — CKX1 specification

| File | Role |
| --- | --- |
| `crypto-kit/README.md` | Design overview and pointers to Go/Kotlin/JS implementations |
| `crypto-kit/FILE-ORGANIZATION.md` | File map |
| `crypto-kit/docs/interop.md` | CKX1 wire contract |

Also: `web-panel/src/crypto/ckx1.js` (admin channel). Implementations live under server, agent, and panel.

---

## Explicitly out of scope for “project source” docs

- `web-panel/node_modules/**`
- `web-panel/dist/**`
- `android-agent/.gradle/**`, `android-agent/app/build/**`
- Any generated R class / intermediate DEX artifacts
- `server/data/**` generated session RSA private keys
