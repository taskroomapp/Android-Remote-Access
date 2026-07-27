# Local testing — emulator, server, and web panel

> **Development-only procedure.** Use a disposable Android emulator or a device that you personally own and have explicitly configured for this test. Do not use this flow to enroll another person’s device or to collect sensitive data. The current source has release-blocking security and privacy gaps; see [Security and privacy](SECURITY_AND_PRIVACY.md).

This procedure validates basic local connectivity, registration, device presence, and a low-risk device-information command. It does not certify the system for production use.

## Prerequisites

| Service/tool | Required? | Notes |
| --- | --- | --- |
| PostgreSQL | Yes | Auth, enrollment, device registry, command persistence, and audit queries need a database. |
| Redis | No | The server starts without Redis; `/health` reports `degraded`. Some cache/queue helpers are unavailable. |
| Go 1.21 | Yes | Required by `server/go.mod`. |
| Node.js/npm | Yes | Used to start the Vite panel. |
| Android Studio, SDK, JDK 17 | Yes | Required to build the Android app. |
| Android emulator | Recommended | Use a disposable AVD and its Android permission prompts. |

## 0. Prepare a local database

Create a **local-only** PostgreSQL role/database and use a unique password. The repository’s `server/scripts/init-database.sql` is a development helper, but it contains a committed placeholder password and should be reviewed/changed before use.

Update a local copy of `server/config.yaml` so that it contains:

- the local PostgreSQL URL,
- a unique JWT signing secret,
- a unique encryption secret (at-rest media),
- `security.ckx1` key file paths pointing at writable locations (defaults under `data/`; created on first run),
- your Redis settings (or an empty Redis address if you intentionally run without it), and
- `tls.enabled: false` only for this loopback/emulator test.

Never expose an HTTP/no-TLS listener to a LAN or public network.

When the server starts, confirm CKX1 identity keys are ready (X25519/Ed25519 files under `data/`). Device commands require `key_offer` → `key_exchange` → `session_ready` before `type=enc` traffic; the panel must complete `/auth/ckx1/offer` + `/exchange` before protected REST. See [Session encryption](SESSION_ENCRYPTION.md) and [Encryption coverage](ENCRYPTION.md). Use a unique `security.encryption_key` (placeholder values are rejected).

## Windows emulator networking

For a Windows Android emulator, use ADB reverse after every emulator boot while the Go service is on port 8443:

```powershell
cd C:\path\to\Android-Remote-Access
.\scripts\emulator-port-forward.ps1
```

Equivalent agent-folder helper:

```powershell
cd C:\path\to\Android-Remote-Access\android-agent
.\emulator-forward.ps1
```

Equivalent direct command when `adb` is on the PATH:

```powershell
adb reverse tcp:8443 tcp:8443
```

Set the Android app’s server URL to:

```text
http://127.0.0.1:8443
```

Here `127.0.0.1` is the emulator’s loopback, which ADB reverse forwards to the host’s `localhost:8443`. Do not use this cleartext address outside the emulator/loopback context.

## 1. Start the Go service

```bash
cd server
go run ./cmd/ -config config.yaml
```

Expected startup line:

```text
Starting server on 0.0.0.0:8443
```

Check the service from the host:

```bash
curl http://localhost:8443/health
```

The response may be `degraded` if Redis is not available. A failed PostgreSQL connection is not acceptable for full end-to-end testing because login and most API handlers require the database.

On a new populated database, the server code creates a development administrator named `admin` with a known default password. Change it immediately in local configuration/testing and never use that identity outside an isolated test database.

## 2. Start the web panel

In another terminal:

```bash
cd web-panel
npm ci
npm run dev
```

Open `http://localhost:3000`. Vite proxies `/api` and `/ws` to port 8443 using `.env.development`.

## 3. Build, install, and enroll the emulator

1. Boot the emulator and confirm ADB sees it before installing:

   ```powershell
   & "$env:LOCALAPPDATA\Android\Sdk\platform-tools\adb.exe" devices -l
   ```

   You need a row ending in `device` (for example `emulator-5554   device`). An empty list, or states such as `offline` / `unauthorized`, means `./gradlew installDebug` will fail with `No connected devices!`. Cold-boot or restart the AVD, then re-check.

2. Run the ADB reverse command above (after every emulator boot while the Go service listens on 8443).

3. Build/install the debug app:

   ```bash
   cd android-agent
   ./gradlew installDebug
   ```

   On Windows use `gradlew.bat installDebug`. To open the app from the host:

   ```powershell
   adb shell am start -n com.remoteagent/.ui.MainActivity
   ```

4. Open **Remote Agent** on the emulator.
5. Enter the loopback server URL shown above.
6. Sign in with the local development administrator and select **Register device with server**.
7. Verify that the **Server device ID** field receives a server-generated UUID. Do not paste the displayed **Agent UUID** into that field.
8. Review Android permission dialogs. Decline anything not required for your test; the app should expose permission failure rather than bypass it.
9. Select **Connect** and verify the visible foreground-service notification and in-app status.
## 4. Verify a minimal end-to-end path

1. In the web panel, open **Devices** and confirm the emulator appears online.
2. Use an approved low-risk request such as device information.
3. Confirm a command result is returned or an explicit failure is shown.
4. Open **Audit Logs** and note that audit final-status persistence is currently limited by the trigger conflict documented in [Architecture](ARCHITECTURE.md).
5. Disconnect in the Android app and verify that the panel transitions to offline after the connection closes.

## 5. Stop and clean up

1. Disconnect the Android app and stop the Go/Vite processes.
2. Remove the app from the emulator or wipe/reset the AVD if sensitive test fixtures were used.
3. Delete browser-site data for `localhost:3000`, including localStorage and IndexedDB (`android_remote_access`).
4. Rotate/delete the local test credentials and database if they were used outside a disposable environment.

## Troubleshooting

| Symptom | Likely cause / check |
| --- | --- |
| `installDebug` → `No connected devices!` | No ADB target is online. Boot the AVD, wait until `adb devices` shows `device`, then retry. On Windows, `adb` may not be on PATH — use `%LOCALAPPDATA%\Android\Sdk\platform-tools\adb.exe`. If the emulator console warns `Failed to start Emulator console for 5554`, install can still succeed when the device row shows `device`. |
| App crashes immediately with `ClassNotFoundException: AgentApplication` | Sources are Kotlin; `org.jetbrains.kotlin.android` must be applied. Run `./gradlew clean installDebug`. |
| Cannot reach `10.0.2.2:8443` or `127.0.0.1:8443` | Verify the Go server is running, rerun `adb reverse tcp:8443 tcp:8443`, and use `http://127.0.0.1:8443` in the emulator. |
| Enrollment returns 401 | Confirm PostgreSQL is reachable, use the server-created test administrator exactly, and check the server URL. |
| Server device ID field contains the agent UUID | Register the device through the app so the server returns a second UUID; the two identifiers are intentionally different. |
| Panel is unauthenticated after reload | Check browser localStorage/site-data behavior; the panel uses stored tokens and user metadata. |
| `/health` says `degraded` | Redis is absent/unreachable. Basic database-backed testing can continue, but Redis helpers are unavailable. |
| Device appears offline | Confirm the foreground service notification, app connection state, server device ID, ADB reverse, and Android network access. |
| Command returns permission error | Grant only the relevant permission in Android settings, or select a test that does not need that data category. |
| Command is unknown | The command is declared by the protocol/server but is not registered in `CommandHandler`; consult the [agent compatibility matrix](../android-agent/README.md#current-command-compatibility). |
