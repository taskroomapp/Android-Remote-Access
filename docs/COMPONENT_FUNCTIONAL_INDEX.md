# Component functional documentation index

Deep, behavior-only documentation for every first-party file and function in the control panel, Go server, and Android agent.

| Document | Contents |
| --- | --- |
| [Full system lifecycle](FULL_SYSTEM_LIFECYCLE.md) | Download → teardown for agent, server, and panel with all branches and crypto math |
| [Session encryption](SESSION_ENCRYPTION.md) | CKX1 device/admin handshake and ChaCha20-Poly1305 framing |
| [PROJECT_FILE_CATALOG.md](PROJECT_FILE_CATALOG.md) | Complete checklist of project-owned files (excludes `node_modules`, build caches, etc.) |
| [SERVER_FUNCTIONAL_REFERENCE.md](SERVER_FUNCTIONAL_REFERENCE.md) | Every server file and function with role-in-project descriptions |
| [WEB_PANEL_FUNCTIONAL_REFERENCE.md](WEB_PANEL_FUNCTIONAL_REFERENCE.md) | Every panel source file, component, hook, and helper with role descriptions |
| [ANDROID_AGENT_FUNCTIONAL_REFERENCE.md](ANDROID_AGENT_FUNCTIONAL_REFERENCE.md) | Every agent class/method, manifest/resource, and build/tooling file |

## How the three parts collaborate

```text
Operator browser (web-panel)
  REST /api/v1/*  +  WebSocket /ws/admin
           │
           ▼
Go server (server/)
  auth · command dispatch · mirror/comms/artifacts persistence
  WebSocket /ws/devices/{server-device-id}
           │
           ▼
Android agent (android-agent/)
  enroll via REST → WS connect → session handshake → enc traffic → execute commands → enc responses
```

## Suggested reading order for deep dives

1. [FULL_SYSTEM_LIFECYCLE.md](FULL_SYSTEM_LIFECYCLE.md) — whole-system path and possibility space
2. [PROJECT_FILE_CATALOG.md](PROJECT_FILE_CATALOG.md) — confirm which files are in scope
3. Matching `*_FUNCTIONAL_REFERENCE.md` for the component you are changing
4. [ARCHITECTURE.md](ARCHITECTURE.md) / [API_REFERENCE.md](API_REFERENCE.md) for cross-component contracts
5. [Security and privacy](SECURITY_AND_PRIVACY.md) before any deployment evaluation

## Conventions

- **Implemented**: handler exists and performs the described work end-to-end (subject to permissions/platform limits).
- **Partial / stub**: route or UI exists but returns placeholders or incomplete persistence.
- **Declared only**: type/constant exists; runtime path is not registered or returns “unknown”.
- Documentation describes **roles and development behavior**, not source code.
