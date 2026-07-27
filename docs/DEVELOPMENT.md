# Development guide

## Before changing code

This repository handles capabilities that can involve private device data. Work only in an authorized local environment and read [Security and privacy](SECURITY_AND_PRIVACY.md) before adding a route, command, Android permission, browser cache, or telemetry field.

Every implementation change that affects any of the following must update the accompanying documentation in the same pull request/change set:

- HTTP/WebSocket request or response shape,
- command allowlist or Android command handler registration,
- Android manifest/runtime permission,
- persisted database field, browser IndexedDB store, or media handling,
- role/authorization behavior,
- third-party browser network call, or
- retention, deletion, or audit behavior.

## Toolchain

| Area | Required tooling |
| --- | --- |
| Server | Go 1.21, PostgreSQL; Redis optional for current local development |
| Web panel | Node.js current LTS, npm, a modern Chromium/Firefox browser for UI testing |
| Android agent | JDK 17, Android Studio/SDK 34, Gradle wrapper 8.7, emulator or authorized test device |
| Windows emulator networking | Android SDK platform tools (`adb`) for `adb reverse` |

## Repository layout

```text
server/
  cmd/                         Entrypoint and YAML config loading
  internal/api/                Routes and HTTP handlers
  internal/cryptokit/          CKX1 (X25519/HKDF/ChaCha20-Poly1305/Ed25519) + AT1 helpers
  internal/database/           PostgreSQL schema/queries and Redis adapter
  internal/dispatcher/         Queue workers and command orchestration
  internal/models/             Domain/API/protocol types
  internal/security/           JWT, password, AT1 at-rest, admin CKX1, TLS, permissions
  internal/websocket/          Device/admin hub + CKX1 handshake/session
  data/                        Generated server CKX1 private keys (gitignored)
  scripts/                     PostgreSQL bootstrap helpers

android-agent/
  app/src/main/kotlin/com/remoteagent/
    config/                    Local preference/URL support
    cryptokit/                 CKX1 Kotlin implementation
    network/                   REST enrollment and WebSocket client
    protocol/                  Wire model and command dispatch
    media/                     Camera/audio helpers
    security/                  TLS transport helpers
    service/                   Foreground service (CKX1 handshake + enc frames)
    ui/                        Visible setup UI

crypto-kit/
  docs/interop.md              CKX1 contract shared by Go, Kotlin, and panel

web-panel/src/
  api/                         Fetch client + CKX1 request wrapping
  crypto/                      Browser CKX1 (noble curves/ciphers/hashes)
  components/                  Shared layout, pickers, icon wrapper, hybrid controls
  context/, hooks/             Device and UI state
  lib/                         Files, mirrors, transfers, media, paths, commands
  pages/                       Routed page components
  styles/                      Global/theme/template/page CSS

docs/                          Documentation set (includes SESSION_ENCRYPTION.md)
scripts/                       Emulator port-forward helper
```

## Server workflow

### Format, compile, and test

```bash
cd server
gofmt -w ./cmd ./internal
go test ./internal/cryptokit/ ./internal/websocket/ ./...
go build ./cmd/
```

Crypto and device-session packages include unit tests for CKX1 vectors, handshake, AAD mismatch, reject paths, and at-rest `DataEncryptor` (`AT1:`). Add tests with every changed behavior, especially for route authorization, malformed device messages, command timeouts, path validation, and persistence behavior.

### Configuration

Run the binary with:

```bash
go run ./cmd/ -config config.yaml
```

Generate the built-in sample configuration with:

```bash
go run ./cmd/ -generate-config
```

The generated example still requires secure environment-specific values. The application currently reads YAML directly rather than mapping environment variables into its configuration struct. Do not commit real credentials, private keys, or signing/encryption secrets.

`security.encryption_key` is **required** (unique, non-placeholder). CKX1 server identity files (`security.ckx1.server_x25519_private_key_file` / `server_ed25519_private_key_file`) are created on first run if missing. Run the server from a stable working directory so relative paths resolve consistently. See [Session encryption](SESSION_ENCRYPTION.md) and [Encryption coverage](ENCRYPTION.md).

### Route changes

1. Add/modify a route in `internal/api/handlers.go`.
2. Require authentication and check an explicit permission.
3. Validate every path, UUID, JSON field, limit, and command type before dispatch.
4. Update `docs/API_REFERENCE.md` with method, route, request/response shape, authorization, and status.
5. Add unit/integration tests for both success and denied/error paths.

Avoid treating a client-side alias or hidden UI state as an authorization control.

### Command changes

A command change spans **three** interfaces:

1. server command type/validator (`models/protocol.go`, `security.InputValidator`),
2. dispatcher/builders and API routes, and
3. Android `DeviceCommand.CommandType` plus a registered `CommandHandler` executor.

Update all three atomically, document the compatibility matrix, define structured output/error schema, and test an authorized failure case when the relevant Android permission is absent. Do not add a command that collects sensitive data without a feature-specific consent and review design.

## Web-panel workflow

```bash
cd web-panel
npm ci
npm run dev
npm run build
```

`npm ci` must be used for repeatable installation because a `package-lock.json` is committed. Do not commit `node_modules/` or `dist/`.

### API client and local state

- Reuse `api` from `src/api/client.js`; do not add ad hoc `fetch()` wrappers without matching token/error/timeout handling.
- Avoid adding browser persistence for sensitive response data. If needed, document retention/clearance and provide a clear/expiry behavior.
- Keep page state independent from authorization: request errors must be visible to the user and must not be converted to empty-data displays.
- The current app has no automated frontend tests; add tests before making complex state changes.

### Icons and accessibility

Use `src/components/ui/Icon.jsx` for all functional icons. The panel uses Lucide SVGs and centralized rules in `src/styles/global.css`.

```jsx
import Icon from '../components/ui/Icon';

<button type="button" title="Refresh devices" aria-label="Refresh devices">
  <Icon name="refresh" size={16} />
</button>
```

When adding a new symbol:

1. Add its Lucide import and semantic key to `Icon.jsx`.
2. Use the semantic key throughout the UI.
3. Do not introduce emoji/text glyphs as functional icons.
4. Preserve visible focus, `title`/accessible names for icon-only controls, color contrast, and reduced-motion behavior.

## Android workflow

```bash
cd android-agent
./gradlew assembleDebug
./gradlew installDebug   # requires adb devices to show a target in "device" state
```

On Windows, prefer `gradlew.bat`. The agent sources are Kotlin (`org.jetbrains.kotlin.android`). If `installDebug` reports `No connected devices!`, boot an AVD and verify with `adb devices -l` (SDK path: `%LOCALAPPDATA%\Android\Sdk\platform-tools`) before retrying. Launch with `adb shell am start -n com.remoteagent/.ui.MainActivity` when needed.

Use a disposable emulator whenever possible. Android behavior varies by API level, OEM restrictions, app role, scoped storage, permission state, and power management. A compilation success does not prove a command is available at runtime.

When touching Android code:

- test the API levels named in the manifest/build configuration,
- request only permissions required for the feature being activated,
- maintain the visible foreground-service disclosure,
- return structured permission errors rather than silently retrying,
- never add a bypass around Android permission, notification, or background-execution rules,
- update `android-agent/README.md` when command/permission behavior changes.

## Local end-to-end check

Follow [Local testing](LOCAL_TESTING.md). The minimum smoke check is:

1. PostgreSQL available; server health endpoint responds.
2. Vite panel loads and authenticates to the local server.
3. Emulator completes explicit enrollment and has a distinct server device ID.
4. Foreground notification is visible while connected.
5. Device appears online in the panel.
6. A permitted, low-risk device-information request returns an explicit success or explicit error.
7. Disconnecting the app updates presence appropriately.
8. Browser localStorage/IndexedDB is cleared after the test.

## Documentation quality check

Before finalizing documentation changes:

```bash
# From repository root: ensure Markdown relative links resolve locally.
python3 - <<'PY'
from pathlib import Path
import re

root = Path('.').resolve()
errors = []
for md in root.rglob('*.md'):
    text = md.read_text(encoding='utf-8')
    for target in re.findall(r'\[[^\]]+\]\(([^)#]+)(?:#[^)]+)?\)', text):
        if '://' in target or target.startswith('mailto:'):
            continue
        candidate = (md.parent / target).resolve()
        if not candidate.exists():
            errors.append(f'{md.relative_to(root)} -> {target}')
if errors:
    print('\n'.join(errors))
    raise SystemExit(1)
print('Markdown relative links: OK')
PY
```

Also run `git diff --check`, Go compilation/tests, and `npm run build` after code changes.

### Required automated checks (CI)

```bash
cd server
gofmt -l ./cmd ./internal
go test ./internal/security/...
go test ./internal/cryptokit/...
go test ./internal/websocket/...
go test ./internal/dispatcher/...
go test ./internal/api/...
go build ./cmd/...
```

From repository root: `git diff --check` and the Markdown relative-link checker above. A workflow at `.github/workflows/ci.yml` runs these on push/PR. Docs are `.md` under `docs/` (not `.txt`).

## Review checklist

- [ ] The change has no hard-coded secrets, test passwords, or private certificates.
- [ ] New route/command inputs have strict validation and server-side authorization.
- [ ] Sensitive actions require explicit, feature-specific consent and visible user notification.
- [ ] Data collection, encryption, retention, deletion, and browser caching are documented.
- [ ] Failure, timeout, offline, and permission-denied paths are tested.
- [ ] UI contains accessible real SVG icons, labels, keyboard focus, and no emoji-as-icon fallback.
- [ ] Public docs match actual source behavior and identify incomplete features.
- [ ] The security findings document is updated if a control or limitation changes.
- [ ] Docs still label the system development-only while P0 blockers in [Security and privacy](SECURITY_AND_PRIVACY.md) remain open.
