# Administrative web panel

## Overview

`web-panel/` is a React 18 single-page application built with Vite. It is an authenticated operator UI for the current Go API; it does not implement its own server-side authorization. A visible control in the UI must therefore be treated as a request to the API, not proof that the action is available, authorized, or complete on the Android agent.

The panel is for authorized, user-notified development and support workflows only. See [Security and privacy](SECURITY_AND_PRIVACY.md).

## Run and build

```bash
cd web-panel
npm ci
npm run dev       # Vite on http://localhost:3000
npm run build     # production-style static bundle in dist/
npm run preview   # preview the built bundle
```

Development configuration is in `.env.development`:

```dotenv
VITE_API_URL=/api/v1
VITE_DEV_API_TARGET=http://localhost:8443
```

`vite.config.js` proxies `/api` to the HTTP target and `/ws` to a derived WebSocket target. For a deployed static build, set `VITE_API_URL` to an appropriate API base at build time. The default production fallback in `src/api/client.js` is `http://localhost:8443/api/v1`, which is not a deployment-safe default.

## Routes

All routes except `/login` are wrapped in `ProtectedRoute`. The protection is client-side; the Go API still needs to validate the JWT on every request.

| URL | Component | Current purpose |
| --- | --- | --- |
| `/login` | `Login` | Admin login and token storage. |
| `/` | `Dashboard` | Device/command statistics and recent activity. |
| `/devices` | `Devices` | Device Management hub: search/filter, per-device interface shortcuts, and a popup listing all operator interfaces. |
| `/devices/:id` | `DeviceDetail` | Device overview and on-demand tabs. |
| `/files` | `FilesPage` | Hybrid file browser with direct and cached-tree modes. Honors `?device=`. |
| `/downloads` | `DownloadsPage` | Browser-side transfer list, retry/resume UI, server transfer readout. Honors `?device=` as device filter. |
| `/orders` | `OrdersPage` | Generic synchronous command form for supported panel command aliases. Honors `?device=`. |
| `/console` | `OrdersPage` | Alias to the Orders page. |
| `/location` | `LocationPage` | One-device map, manual requests, browser-side tracking loop, reverse geocoding. Honors `?device=`. |
| `/contacts` | `ContactsSmsPage` | Cached and live contacts/SMS-oriented lists. Honors `?device=`. |
| `/live` | `LiveViewPage` | Session-only still captures and audio-recording UI. Honors `?device=`. |
| `/audit` | `AuditLogs` | Audit-search form and pagination. |
| `/settings` | `Settings` | Displays current panel/server details and user context. |

`src/pages/CommandConsole.jsx` remains in the source tree but is **not** registered in `App.jsx`; `/console` routes to `OrdersPage`. Do not treat the unregistered component as a supported route.

## Authentication and API client

`src/api/client.js` is the shared API client.

- On login, it saves `access_token`, `refresh_token`, and user metadata in `localStorage`.
- It adds `Authorization: Bearer <access_token>` to normal JSON requests.
- On a 401, it tries `POST /auth/refresh` once, updates the stored tokens, and retries the request.
- It uses `AbortController` with a default 30-second timeout, except generic commands use a timeout derived from their requested command duration.
- `logout()` calls the API opportunistically and clears browser tokens even if that request fails.
- The admin WebSocket URL is constructed as `/ws/admin?token=<access-token>` relative to the panel's browser origin.

Persisting bearer tokens in localStorage and exposing an access token in a WebSocket query string are current implementation details with security implications; they are not an approved production authentication design.

## Operator pages

### Dashboard and devices

`Dashboard`, `Devices`, `DeviceDetail`, and the `DeviceContext` use device-list/status/dashboard APIs. Online/offline status comes from the server hub, and the panel also listens to the admin WebSocket for live changes.

`Devices` is a navigation hub. Each row shows compact interface shortcuts (Files, Downloads, Location, Contacts, Live View, Orders). Clicking the device name or the list action opens a modal of the same interfaces (plus Device detail). Choosing an interface navigates to that page with `?device=<id>` so the DevicePicker is pre-selected; the operator still presses Fetch/Sync/capture on the target page. These shortcuts are UI routes only — they do not reflect Android runtime permission grants. The catalog lives in `src/lib/deviceInterfaces.js`.

Operator pages that accept a device (`/files`, `/downloads`, `/location`, `/contacts`, `/live`, `/orders`) read and keep `?device=` in sync with the picker.

The Device Detail tabs issue on-demand calls for information, contacts, and call logs. An empty table can mean no returned data, a missing runtime permission, an offline device, a command failure, or an incomplete backend response; it is not necessarily a statement about the device.

### Files

The Files page implements a hybrid UI model:

| Mode | Source | Current implementation |
| --- | --- | --- |
| **Cached folder tree** | IndexedDB + server mirror snapshot | Uses existing `file_tree` snapshot entries/roots. Server mirror snapshots are memory-only and are lost on restart. |
| **Direct browsing** | Synchronous generic command | Sends `file_list`; a failed live list falls back to a local sidebar cache for that path. |

The page includes bookmarks, breadcrumbs, tree expansion, list/grid views, local sort/search, details panel, simple preview, selection bar in direct mode, and a browser-side download pipeline. Its **Sync tree** process scans through the currently available agent file-list interface and uploads the resulting snapshot JSON to `/mirrors/{device}/update`; the server does not independently crawl the device.

The file page normalizes selected Android emulated-storage paths in the browser, but the server does not enforce a canonical storage-root allowlist. This is a security boundary that must be implemented server-side before any broader use.

### Downloads

`DownloadsPage` merges:

1. local IndexedDB `transfers` records, and
2. `GET /transfers` responses.

The server transfer endpoints are currently stubs. Progress, retry, resumption, and cleanup behavior is therefore principally browser-side / best-effort UI behavior, not a durable server transfer service. The file stream endpoint does support server-mediated chunk aggregation, but there is no fully persisted transfer-session engine.

### Orders

`OrdersPage` is the registered generic command page. It filters the device picker to online devices and uses `api.executeCommand` plus a result view. Panel “spec-style” aliases are converted by the API client to server command names; for example, `file.list` maps to `file_list` and `location.get` maps to `get_location`.

Aliases are client conveniences only. The backend validates its own allowlist and the agent may still return `Unknown command type` for an unimplemented command.

### Location

`LocationPage` stores a per-device browser history (capped in page logic), requests a current location through generic commands, and can make repeated synchronous `get_location` requests while the local tracking toggle is on. It is not a server-pushed tracking stream.

Map and geocoding data go directly from the browser to third parties:

- Carto basemap (street),
- Esri World Imagery (satellite),
- OpenTopoMap (terrain), and
- OpenStreetMap Nominatim reverse geocoding.

Coordinates are therefore disclosed to those services when the corresponding UI functions are used. Production use requires an approved mapping/geocoding provider, an applicable privacy assessment, and compliance with each provider's terms.

### Contacts and SMS

The Contacts/SMS page bootstraps from local IndexedDB records and then may retrieve mirror snapshots or use live generic commands. Current server-side mirrors only store what the panel posts; they do not collect contacts or SMS themselves. The agent implements contact and SMS retrieval only when the corresponding Android permission has been granted.

The panel has UI paths for inbox/sent/conversation/search views. Server/agent support is uneven, so these views must display and preserve explicit errors rather than infer absence of data.

### Live view

The Live View page is designed around manually initiated still capture and audio-session controls. It retains captures in the current browser session/IndexedDB-related state for presentation; it is not a video-streaming implementation. The Go camera stream route returns “not implemented.”

### Audit and settings

The audit page calls `POST /audit/logs`; individual audit-log lookup is not implemented by the server. Settings displays panel/environment values and the current authenticated user; it is not a server-side configuration editor.

## Local browser storage

The API client opens IndexedDB database `android_remote_access`, version 1. Its stores are:

| Store | UI use |
| --- | --- |
| `file_trees` | Cached file-tree snapshots by device. |
| `contacts` | Cached contact mirror by device. |
| `sms_inbox`, `sms_sent` | Cached SMS-oriented data by device. |
| `transfers` | Local download request/progress records; indexed by device/path. |
| `location_history` | Browser-side location history records. |
| `camera_captures`, `audio_recordings` | Browser-side media session records. |
| `preferences` | Per-panel/device UI choices. |

Browser storage can persist after logout and may be accessible to another user of the same browser profile. It must be cleared as part of sign-out/device handoff, and production designs should use encrypted, short-lived, least-data local caches.

## SVG icon system

All interface symbols should use the shared `src/components/ui/Icon.jsx` wrapper over `lucide-react`. Do not use emoji or text glyphs as functional icons.

The global rules in `src/styles/global.css` provide:

- semantic size tokens (`--icon-size-xs` through `--icon-size-xl`),
- consistent SVG baseline/flex behavior through `.ui-icon`,
- module, result-state, file, empty-state, and button icon containers,
- success/warning/error result colors,
- visible keyboard focus rings for icon-only controls, and
- a reduced-motion preference override.

When adding an icon:

1. Import it in `Icon.jsx` and give it a clear semantic key in `ICONS`.
2. Render `<Icon name="semanticKey" size={…} />`, rather than importing a raw icon in an arbitrary page.
3. Give icon-only buttons a `title` and, where the visible label is absent, an accessible name.
4. Keep state/meaning in visible text or `aria-label`; icons alone are not sufficient for accessibility.

## Styling layout

- `global.css`: tokens, base elements, common components, SVG icon system.
- `layout.css`: shell/sidebar/login/device-detail layouts.
- `hybrid-theme.css`, `hybrid-templates.css`, `suite-dashboard.css`: dense operator layouts and page templates.
- `operations.css`, `commands.css`, `devices.css`, `dashboard.css`, `audit.css`: page-focused rules.

`App.jsx` imports the active style sheets directly. `src/main.css` is a legacy import file and is not the main source of styling in the current app root.
