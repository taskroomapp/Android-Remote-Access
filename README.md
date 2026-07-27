# Android Remote Access — Current Implementation Documentation

> **Status: development / prototype — not production-ready.**
>
> Encryption uses **CKX1** (X25519 + HKDF-SHA256 + ChaCha20-Poly1305 + Ed25519) for device sessions and `AT1:` ChaCha20-Poly1305 at rest; see [Encryption coverage](docs/ENCRYPTION.md). Authorization, audit persistence, path policy, TLS enforcement, origin validation, admin WS tickets, and browser storage controls remain release blockers — see [Security and privacy](docs/SECURITY_AND_PRIVACY.md).
>
> This repository contains a Go service, a React administrative panel, and an Android companion app. It handles highly sensitive device data and peripheral permissions. It may be used only for organization-owned devices or devices for which the person using the device has given explicit, informed, revocable consent. Do not expose it to the public internet or use it for covert monitoring.

This README documents **what the source currently does**, including incomplete routes and security limitations. It intentionally does not represent the project as an MDM, an endpoint-security product, or a production-ready remote-support platform.

## Documentation map

| Document | Purpose |
| --- | --- |
| [Architecture](docs/ARCHITECTURE.md) | Runtime components, data flow, persistence, WebSockets, and implementation boundaries. |
| [Full system lifecycle](docs/FULL_SYSTEM_LIFECYCLE.md) | End-to-end path for agent, server, and control panel: every phase, branch, and crypto/math possibility space. |
| [Session encryption](docs/SESSION_ENCRYPTION.md) | CKX1 device/admin handshake, directional ChaCha20-Poly1305 frames, AAD, verification. |
| [Encryption coverage](docs/ENCRYPTION.md) | Session + admin channel + `AT1:` at-rest, config, and verification. |
| [API reference](docs/API_REFERENCE.md) | HTTP/WebSocket surface, authentication model, response conventions, and endpoint status. |
| [Web panel guide](docs/WEB_PANEL.md) | Routes, local browser storage, user-interface behavior, and icon system. |
| [Component functional index](docs/COMPONENT_FUNCTIONAL_INDEX.md) | Entry point to deep file/function docs for panel, server, and agent. |
| [Project file catalog](docs/PROJECT_FILE_CATALOG.md) | Complete checklist of first-party project files (no dependencies/build outputs). |
| [Server functional reference](docs/SERVER_FUNCTIONAL_REFERENCE.md) | Every Go file/function with detailed role-in-project descriptions. |
| [Web panel functional reference](docs/WEB_PANEL_FUNCTIONAL_REFERENCE.md) | Every panel file/function/component with detailed role descriptions. |
| [Android agent functional reference](docs/ANDROID_AGENT_FUNCTIONAL_REFERENCE.md) | Every agent class/method, resource, and build file with detailed roles. |
| [Android agent](android-agent/README.md) | Build, enrollment model, lifecycle, permissions, and command-compatibility matrix. |
| [Local testing](docs/LOCAL_TESTING.md) | Isolated emulator-only development verification. |
| [Development guide](docs/DEVELOPMENT.md) | Prerequisites, builds, validation, code layout, and contributor workflow. |
| [Security and privacy](docs/SECURITY_AND_PRIVACY.md) | Current controls, material gaps, go/no-go criteria, and remediation checklist. |

## What is in this repository

```text
.
├── server/              Go HTTP/WebSocket service, PostgreSQL and optional Redis adapters
├── android-agent/       Android application (Kotlin, Gradle/AGP)
├── web-panel/           React + Vite operator interface
├── crypto-kit/          CKX1 interop docs (implementations live in server, agent, web-panel)
├── docs/                Architecture, API, session encryption, safety, test, and development documentation
└── scripts/             Windows Android-emulator port-forward helper
```

### Components at a glance

| Component | Stack | Current responsibility |
| --- | --- | --- |
| `server` | Go 1.21, Gorilla Mux/WebSocket, PostgreSQL, optional Redis | REST API, JWT + CKX1 admin channel, device WebSocket hub with CKX1 AEAD, command dispatch, `AT1:` at-rest, audit/pending-command persistence. |
| `android-agent` | Kotlin, Android SDK 34, OkHttp | Visible foreground service, enrollment UI, CKX1 WebSocket sessions, heartbeats, permission-gated command handlers. |
| `web-panel` | React 18, Vite 5, React Router, Leaflet, Lucide, @noble crypto | Login, CKX1-sealed REST/WS, dashboard, device pages, IndexedDB cache. |

## Implemented behavior vs. product claims

The source has useful development functionality, but several features often described as enterprise controls are **not complete**:

- The agent is a visible Android foreground service and asks for runtime permissions; it is not a hidden background component.
- The server can use TLS, but the current code does **not** enforce verified mutual TLS for device WebSockets.
- PostgreSQL persistence is required for a usable authenticated deployment. The server may start when PostgreSQL is unavailable, but login and most APIs still depend on the database.
- Redis is optional. The current offline-command processor reads PostgreSQL pending commands; the Redis queue helpers are not wired into that processor.
- The audit-table trigger blocks all updates, including the dispatcher's attempt to attach a final response. This makes the present audit implementation internally inconsistent; see [Security and privacy](docs/SECURITY_AND_PRIVACY.md).
- Transfer listing, transfer appeal/clear, audit-log detail, most administrator management, and camera streaming are stubs or partial implementations.
- The dashboard and panel may expose an action before the server/agent has a complete implementation for it. UI availability is not a capability guarantee.

See the endpoint status tables in the [API reference](docs/API_REFERENCE.md) and the agent matrix in [Android agent](android-agent/README.md).

## Development prerequisites

| Tool/service | Version / note |
| --- | --- |
| Go | 1.21 (declared in `server/go.mod`) |
| Node.js | Current supported LTS recommended; lockfile uses npm |
| PostgreSQL | Required for normal auth, registry, audit, and command operation |
| Redis | Optional in the current codebase; health reports `degraded` without it |
| Android Studio / SDK | Compile SDK 34, JDK 17, Android Gradle Plugin 8.5.2 |
| Android emulator | Recommended for local development; do not test on another person's device |

## Local development quick start

Use a dedicated local PostgreSQL instance and a disposable Android emulator. The detailed, verified sequence is in [Local testing](docs/LOCAL_TESTING.md).

```bash
# Server
cd server
go run ./cmd/ -config config.yaml

# Separate terminal: web panel
cd web-panel
npm ci
npm run dev
```

The development panel runs on `http://localhost:3000` and proxies API/WebSocket requests to the Go service on port `8443` by default.

> `server/config.yaml` is a development configuration currently committed to the repository. It contains placeholder/insecure values and must never be copied to a shared or production environment. Create an environment-specific configuration with unique secrets and database credentials.

## Build and validation

```bash
# Compile/test Go packages
cd server
go test ./...

# Produce a panel bundle
cd ../web-panel
npm ci
npm run build

# Build an Android debug APK
cd ../android-agent
./gradlew assembleDebug
```

The repository does not currently contain a CI workflow or automated integration-test suite. A successful build only confirms compilation/bundling; it does not certify security, Android permission behavior, or production readiness.

## High-level runtime flow

```text
Browser (React/Vite)
  ├─ HTTPS/REST ────────────────► Go API
  └─ WebSocket /ws/admin ──────► Go WebSocket hub
                                      │
                                      ├─ PostgreSQL: users, devices, sessions,
                                      │  pending commands, audit rows, media blobs
                                      ├─ Redis (optional): online-device cache,
                                      │  response/cache helpers
                                      └─ Device WebSocket /ws/devices/{id}
                                           │  CKX1 key_offer → key_exchange → session_ready
                                           │  then ChaCha20-Poly1305 type=enc frames
                                           ▼
                                      Android foreground service
```

## Privacy and operational requirements

Before any authorized evaluation or deployment, establish all of the following:

1. Written scope, named device inventory, data-retention period, and a lawful basis for every data category.
2. Per-device, in-app disclosure of active connection and requested permission scopes.
3. Least-privilege roles, dual approval for sensitive operations, and an audit-review process.
4. A supported way for a device user to revoke consent, disconnect, and remove the app.
5. A security review that closes every **Critical** and **High** item in [Security and privacy](docs/SECURITY_AND_PRIVACY.md).

## License

No license file is present in the repository. Unless a license is added by the copyright holder, do not assume that the source may be redistributed or used outside the authorized project context.
