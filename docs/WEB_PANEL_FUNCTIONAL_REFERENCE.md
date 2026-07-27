# Web panel — deep functional reference (every file & function)

Behavior-only documentation for first-party sources under `web-panel/` (excluding `node_modules/` and `dist/`). See also [PROJECT_FILE_CATALOG.md](PROJECT_FILE_CATALOG.md).

---

## Development role of the control panel

The panel is the **operator cockpit**: authentication, device presence, command issuance, file browsing/downloads, communications review, location mapping, live media actions, and audit search. It never talks to devices directly—all remote work goes through the Go API (REST) and admin WebSocket (presence).

---

## Build / shell files

### `package.json` / `package-lock.json`

**Role.** Declare React 18, Vite 5, React Router, Leaflet, Lucide, and npm scripts (`dev`, `build`, `preview`). Lockfile pins transitive versions for reproducible installs.

### `vite.config.js`

**Role.** Dev server on port 3000; proxies `/api` → Go HTTP and `/ws` → Go WebSocket (`VITE_DEV_API_TARGET`, default `http://localhost:8443`). This is why the panel can use relative `/api/v1` and `/ws/admin` in development without CORS pain.

### `index.html`

**Role.** SPA host document; mounts the React root for `main.jsx`.

---

## `src/main.jsx` / `src/main.css`

| Symbol / file | Role |
| --- | --- |
| `main.jsx` | Creates React root and renders `<App />`; imports CSS entry |
| `main.css` | Pulls global stylesheet chain into the bundle |

---

## `src/App.jsx`

**File role.** Application shell: authentication context, route table, protected layout.

| Symbol | Role in the project |
| --- | --- |
| `useAuth` | Public hook for pages needing the signed-in operator profile and login/logout actions |
| `AuthProvider` | Holds `user`/`loading`; on mount restores `user` from `localStorage` if an access token exists; `login` calls API, stores tokens via client, persists user JSON; `logout` clears server session best-effort and local state |
| `AppLayout` | Chrome around authenticated pages: Sidebar + main content; shows online device count from DeviceProvider |
| `ProtectedRoute` | Blocks unauthenticated navigation; shows LoadingScreen while auth bootstraps; redirects to `/login` preserving intended location |
| `App` | Declares all routes (login public; nested authenticated routes for every operator page). Note `/orders` and `/console` both render `OrdersPage`; `CommandConsole.jsx` exists as an alternate UI module not currently wired in this router |

---

## `src/api/` — HTTP client (split by domain)

**File role.** Sole HTTP/IndexedDB gateway for the panel. `client.js` holds core auth/IndexedDB/commands and attaches domain methods; pages still `import { api } from '../api/client'`.

| File | Role |
| --- | --- |
| `client.js` | `ApiClient` core, tokens, IndexedDB, `executeCommand`, singleton `api` |
| `errors.js` | `ApiError` |
| `devices.js` | Device registry methods |
| `files.js` | Path normalize + file list/read/stream |
| `contacts.js` | `getContacts` |
| `records.js` | `getCallLogs` |
| `camera.js` | `cameraSnapshot` |
| `mic.js` | `micStart` / `micStop` |
| `location.js` | `getLocation` |
| `comms.js` | Save/list/export device communications |
| `artifacts.js` | Save/list/export artifacts |
| `media.js` | Stored media list/download |
| `mirrors.js` | Mirror get/update |
| `transfers.js` | Transfer stubs |
| `audit.js` | Audit search + dashboard stats |

### Module constants (`client.js`)

| Symbol | Role |
| --- | --- |
| `API_BASE_URL` | Resolves API prefix from env / DEV proxy / production default |
| `DB_NAME` / `DB_VERSION` | IndexedDB identity for local caches |
| `COMMAND_ALIASES` | Maps dotted UI command names to server snake_case types so Orders/Console can use product language |

### Core `ApiClient` methods

| Method | Role |
| --- | --- |
| `constructor` | Loads tokens; kicks off IndexedDB open |
| `initDB` | Creates object stores for trees, contacts, SMS, transfers, location/camera/audio history, preferences |
| `dbGet` / `dbPut` / `dbGetAll` | Low-level IndexedDB accessors used by mirror/transfer helpers and cached UI state |
| `request` | Authenticated fetch with timeout abort, automatic one-shot refresh on 401, JSON or blob decoding, normalized `ApiError` |
| `refreshAccessToken` | Quietly renews tokens; forces logout if refresh fails |
| `setTokens` / `clearTokens` / `isAuthenticated` | Token lifecycle in memory + `localStorage` |
| `login` / `logout` | Auth endpoints |
| `executeCommand` | Posts a command (alias-mapped) with extended timeout so long agent work does not abort early |
| `getCommandStatus` / `pollCommandResult` | Status polling until terminal states |

Domain methods (devices, files, contacts, records, camera, mic, location, comms, artifacts, media, mirrors, transfers, audit) are attached via `attach*Api(ApiClient.prototype)` from the matching modules above.

### Free functions / types

| Symbol | Role |
| --- | --- |
| `payloadToBlob` / `base64ToBlob` | In `files.js` — turn payloads into downloadable Blobs |
| `ApiError` | Structured error with HTTP status + machine code |
| `getAdminWebSocketUrl` | Builds authenticated admin WS URL from current origin + access token |
| `api` | Singleton instance imported everywhere |

---

## Context & hooks

### `src/context/DeviceContext.jsx`

| Symbol | Role |
| --- | --- |
| `DeviceProvider` | Loads device list when authenticated; refreshes every 15s; also refreshes on admin WS online/offline events; exposes `devices`, `onlineDevices`, `loading`, `loadDevices` |
| `useDevices` (context) | Consumes the provider; required by AppLayout and many pages |

### `src/hooks/useDevices.js`

| Symbol | Role |
| --- | --- |
| `useDevices({ onlineOnly, storageAccessOnly })` | Lightweight filtered list helper for pickers (separate from context hook name collision—prefer context inside authenticated tree) |
| `deviceLabel` | Consistent display naming |
| `isDeviceOnline` | Normalizes status fields |

### `src/hooks/useAdminWebSocket.js`

**Role.** React binding to `subscribeAdminWebSocket` so components can show connection state.

### `src/hooks/useDebouncedValue.js`

**Role.** Delays propagating rapidly changing inputs (search boxes) to reduce re-render/API spam.

### `src/hooks/usePanelLayout.js`

| Symbol | Role |
| --- | --- |
| `storageKey` / `clamp` / `loadWidths` | Internal helpers for persisted split widths |
| `usePanelLayout` | Returns resizable pane width state for Files/Contacts hybrid layouts |

---

## Libraries (`src/lib/`)

### `adminWebSocket.js`

Shared singleton so React Strict Mode double-mount does not thrash sockets.

| Function | Role |
| --- | --- |
| `isConnected` | ReadyState check |
| `notify` | Fan-out connection boolean to subscribers |
| `clearDisconnectTimer` | Cancels delayed teardown |
| `connect` | Opens/reuses WS; parses device_online/offline into event listeners; reconnects after close if still subscribed |
| `subscribeAdminWebSocket` | Ref-counted connection subscription for UI status |
| `subscribeAdminDeviceEvents` | Presence event subscription used by DeviceProvider |

### `commandRunner.js`

| Function | Role |
| --- | --- |
| `pollCommandResult` | Generic poller for transaction IDs |
| `runCommand` | Execute + poll convenience |
| `parseCommandData` | Normalize agent payloads for UI |
| `listFilesLive` / `readFileLive` | Live filesystem helpers used by Files page |
| `normalizeFileList` | Convert agent list JSON into UI file entries |
| `joinPath` / `sleep` | Path + delay utilities |

### `mirror.js`

| Function | Role |
| --- | --- |
| `getBrowsePrefs` / `saveBrowsePrefs` | Remember path/sort preferences per device |
| `loadLocalMirror` / `saveLocalMirror` | IndexedDB snapshot I/O |
| `fetchServerMirror` | Pull server cache |
| `pickNewerSnapshot` | Conflict resolution by freshness |
| `isMirrorStale` | Age threshold for “refresh recommended” banners |
| `mirrorChildren` / `mirrorRoots` | Navigate cached trees without hitting the device |
| `buildFileTreeFromDevice` | Recursive live walk with progress—expensive but builds offline-friendly trees |
| `syncFileTreeMirror` | Build + local save + server update pipeline |
| `emptyTreeSnapshot` | Blank tree template |

### `transfers.js`

Client-side download manager compensating for stub server transfer APIs.

| Function | Role |
| --- | --- |
| `openDb` / `withStore` | Transfers store access |
| `transferKey` | Stable identity for device+path |
| `getLocalTransfers` / `upsertLocalTransfer` / `removeLocalTransfer` / `clearCompletedLocal` | Queue CRUD |
| `mergeTransfers` | Combine server (empty) + local rows for Downloads UI |
| `fetchServerTransfers` | Calls stub API |
| `onTransferProgress` / `emitProgress` | Progress pub/sub for rows |
| `runQueued` | Serializes concurrent downloads per device |
| `startDownload` / `executeDownload` | Create transfer row and stream bytes to completion/failure |
| `cancelDownload` / `retryDownload` / `processWaitingDownloads` / `appealAndRetry` | Operator controls; appeal hits stub then retries |

### `largeFileDownload.js`

| Function | Role |
| --- | --- |
| `decodeBase64ToUint8Array` | Chunk decode |
| `readFileChunk` | Issues one chunk command |
| `downloadDeviceFileAsBlob` | Assembles full file via chunks with abort/progress—fallback when stream endpoint unavailable |
| `statRemoteFileSize` | Infers size from first chunk metadata |

### `paths.js`

| Function | Role |
| --- | --- |
| `normalizePath` | Canonicalize Android paths |
| `pathKey` | Stable React key for paths |
| `parentPath` / `joinPath` | Navigation helpers |
| `rewriteEmulatedPath` | Map legacy emulated paths to `/storage/emulated/0` |
| `breadcrumbSegments` | Split path for breadcrumb UI |

### `fileBrowserUi.js`

| Symbol | Role |
| --- | --- |
| `FILE_BOOKMARKS` | Quick-jump locations (DCIM, Download, etc.) |
| `formatFileSize` / `formatFileModified` / `getFileTypeLabel` | Display formatters |
| `sortAndFilterFiles` | Client-side search/sort |
| `cycleFileSort` / `sortFieldLabel` | Sort UX |
| `pathKey` | Path key helper (local to file UI) |

### `media.js`

| Function | Role |
| --- | --- |
| `extractBase64Payload` | Dig base64 out of nested command results |
| `toDataUrl` / `toImageDataUrl` / `toAudioDataUrl` | Build browser-playable/viewable URLs |
| `cleanBase64` / `isLikelyBase64` / `binaryStringToBase64` / `bytesToBase64Payload` / `sniffMimeFromBase64` | Internal decode/MIME helpers |
| `looksLikeImagePayload` | Heuristic for preview decisions |

### `deviceInterfaces.js`

| Symbol | Role |
| --- | --- |
| `DEVICE_INTERFACES` | Canonical list of operator interfaces (detail, files, contacts, live, …) with routes/icons |
| `DEVICE_INTERFACE_CHIPS` | Subset shown as chips on device cards |

---

## Components

### `components/Sidebar.jsx`

**Role.** Primary navigation for all authenticated pages; shows username; logout; online device badge.

### `components/Layout.jsx`

**Role.** Alternate layout wrapper composing sidebar + content when needed outside App’s default chrome.

### `components/DevicePicker.jsx`

**Role.** Dedicated device select control with online/offline filtering for operational pages.

### `components/LoadingScreen.jsx`

| Export | Role |
| --- | --- |
| `LoadingScreen` | Full-page boot/loading |
| `LoadingOverlay` | Modal-style busy state |
| `ActionButton` | Shared button with loading/disabled variants |
| `StatusBadge` | Status chip |
| `EmptyState` | Empty dataset placeholder |
| `Toast` | Transient notifications |
| `DevicePicker` | Inline picker variant |
| `Breadcrumbs` | Path crumbs for file browser |
| `FileIcon` | File/folder glyph |

### `components/ui/Icon.jsx`

| Symbol | Role |
| --- | --- |
| `Icon` | Resolve Lucide icon by project name |
| `fileTypeIcon` | Map filename/dir → icon |
| `statusIcon` | Map status → icon |

### `components/hybrid/HybridControls.jsx`

| Export | Role |
| --- | --- |
| `HybridPrimaryButton` / `HybridSecondaryButton` / `HybridAccentButton` | Styled action buttons |
| `HybridEmptyState` / `HybridLoadingState` | Consistent empty/loading blocks |

### `components/hybrid/FileEntryViews.jsx`

| Export | Role |
| --- | --- |
| `FileBrowserBanners` | Online/stale/truncated warnings |
| `FileSelectionBar` | Multi-select + batch download controls |
| `FileListTable` | Tabular file listing |
| `FileGridView` | Grid listing |
| `FileDetailsPanel` | Selected entry metadata + preview/download actions |

### `components/hybrid/TruncatedText.jsx`

**Role.** Ellipsis overflow with full text available via title/tooltip for dense tables.

---

## Pages

### `pages/Login.jsx`

**Role.** Collects credentials, calls `useAuth().login`, navigates to intended route or home; surfaces auth errors.

### `pages/Dashboard.jsx`

**Role.** Calls `getDashboardStats`, shows counts and recent activity summaries; helpers `formatCommandName`, `getBatteryIconName` adapt raw data for display; deep-links into device interfaces.

### `pages/Devices.jsx`

**Role.** Registry table with battery coloring (`getBatteryColor`), last-seen formatting (`formatLastSeen`), interface chips, navigation to detail, refresh/delete flows.

### `pages/DeviceDetail.jsx`

**Role.** Single-device dossier: status, hardware fields, quick actions into Files/Contacts/Live/etc.; helpers `formatDate`, `formatDateTime`, `formatDuration`, `formatBytes`.

### `pages/Files.jsx`

**Role.** File browser page shell: device selection, mirror vs live mode, bookmarks, sort/filter, preview, batch download. Tree UI lives in `components/files/FileTree.jsx`; shared download via `lib/download.js`.

### `pages/Downloads.jsx`

**Role.** Operator view of the local transfer queue; `formatSize`, `xferBadgeClass`, `TransferRow` render each job with retry/cancel/appeal/remove.

### `pages/Orders.jsx`

**Role.** Spec-driven command runner (parameterized orders). Helpers: `defaultParams`, `paramHint`, `buildParams`, `formatResult`, `redactLargeFields` (prevents dumping huge base64 into the UI). Mounted at `/orders` and `/console`.

### `pages/CommandConsole.jsx`

**Role.** Alternate module-catalog console (file/comms/camera/mic/info/apps). Internals: `loadDevices`, `addResult`, `executeCommand`, `pollCommandResult`, `getPayloadFields`. **Not currently mounted in `App.jsx` routes**—kept as a first-party UI alternative to Orders.

### `pages/Location.jsx`

**Role.** Location page orchestrator. Map UI in `components/location/MapPanel.jsx`; geocode/`accuracyLabel`/`isValidPoint` in `features/location/geocode.js`; map sync helpers in `features/location/mapHelpers.js`.

### `pages/ContactsSms.jsx`

**Role.** Communications page orchestrator (contacts / SMS / call logs). Domain helpers live under `features/comms/`; UI under `components/comms/`; data bootstrap/sync/persist in `hooks/useDeviceComms.js`.

| Module | Role |
| --- | --- |
| `features/comms/phones.js` | Phone normalize/index/resolve |
| `features/comms/contacts.js` | Contact UI mapping + prefs |
| `features/comms/messages.js` | SMS mapping, conversations, enrich |
| `features/comms/records.js` | Call-log mapping/enrich |
| `features/comms/nav.js` | Sidebar nav + search text helpers |
| `components/comms/*` | Sidebar, list, contact detail, SMS thread |
| `hooks/useDeviceComms.js` | Sync from device, mirror, DB save/load/export |

### `pages/LiveView.jsx`

**Role.** Capture Studio orchestrator. Camera/mic logic in `features/live/` + hooks; UI in `components/live/`.

| Module | Role |
| --- | --- |
| `features/live/camera.js` | Still capture + optional artifact save |
| `features/live/mic.js` | Mic start/stop + audio entry |
| `features/live/formatters.js` | Session clock / bytes / storage labels |
| `hooks/useCameraCapture.js` / `useMicRecording.js` / `useMediaSession.js` | Capture busy state + gallery session |
| `components/live/*` | Camera/mic controls, stage, strip, lightbox |

### `pages/AuditLogs.jsx`

**Role.** Search form → `searchAuditLogs`; table rendering; helpers `formatTimestamp`, `formatCommandType`. Detail depends on stub server endpoint.

### `pages/Settings.jsx`

**Role.** Shows account fields from auth context, sign-out, version, and resolved API base URL. No remote settings mutation beyond logout.

---

## Styles (`src/styles/*.css`)

| File | Role |
| --- | --- |
| `global.css` | Base resets/typography |
| `hybrid-theme.css` | Hybrid visual tokens |
| `dashboard.css` | Dashboard layout |
| `devices.css` | Devices/detail |
| `commands.css` | Console/orders |
| `audit.css` | Audit page |
| `operations.css` | Operational pages shared |
| `hybrid-templates.css` | Hybrid template layouts |
| `suite-dashboard.css` | Suite dashboard variants |
| `layout.css` | App chrome / sidebar / main |

No functions; presentation only.

---

## Browser storage the panel owns

| Store | Role |
| --- | --- |
| `localStorage` tokens/user | Session continuity |
| `localStorage` layout/browse/contact prefs | UX persistence |
| IndexedDB caches | Offline-friendly mirrors, transfers, media/location history |

Development convenience only—not a hardened secret store.
