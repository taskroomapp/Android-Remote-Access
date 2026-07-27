# Documentation index

The documentation describes the repository as it exists today. It distinguishes implemented behavior from planned or incomplete functionality.

| Document | Contents |
| --- | --- |
| [../README.md](../README.md) | Project overview, repository map, prerequisites, status, and safe-use boundary. |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Component boundaries, command lifecycle, WebSocket messages, persistence, and data flow. |
| [Session encryption](SESSION_ENCRYPTION.md) | Device WebSocket session crypto: authenticated handshake, AES-GCM frames, AAD layers, config, and limits. |
| [Encryption coverage](ENCRYPTION.md) | Full encryption coverage: session + at-rest fail-closed stores and verification. |
| [API reference](API_REFERENCE.md) | Versioned HTTP and WebSocket interfaces, authentication, endpoint status, and error shapes. |
| [WEB_PANEL.md](WEB_PANEL.md) | React routes, operator pages, IndexedDB caches, API client behavior, and the SVG icon system. |
| [COMPONENT_FUNCTIONAL_INDEX.md](COMPONENT_FUNCTIONAL_INDEX.md) | Index to file/function-level functional references for panel, server, and agent. |
| [PROJECT_FILE_CATALOG.md](PROJECT_FILE_CATALOG.md) | Complete checklist of first-party project files (excludes dependencies/build outputs). |
| [SERVER_FUNCTIONAL_REFERENCE.md](SERVER_FUNCTIONAL_REFERENCE.md) | Deep per-function roles for every Go server source file. |
| [WEB_PANEL_FUNCTIONAL_REFERENCE.md](WEB_PANEL_FUNCTIONAL_REFERENCE.md) | Deep per-function roles for every panel source file, hook, and helper. |
| [ANDROID_AGENT_FUNCTIONAL_REFERENCE.md](ANDROID_AGENT_FUNCTIONAL_REFERENCE.md) | Deep per-method roles for every agent class, manifest, resource, and build file. |
| [LOCAL_TESTING.md](LOCAL_TESTING.md) | Emulator-only local test procedure and troubleshooting. |
| [DEVELOPMENT.md](DEVELOPMENT.md) | Build commands, repository conventions, validation steps, and contribution guidance. |
| [Security and privacy](SECURITY_AND_PRIVACY.md) | Existing controls, known security/privacy gaps, release blockers, and remediation priorities. |
| [../android-agent/README.md](../android-agent/README.md) | Android app architecture, lifecycle, permissions, enrollment, and supported command matrix. |

## Reading order

1. Start with the root [README](../README.md) and [Security and privacy](SECURITY_AND_PRIVACY.md).
2. For local engineering work, follow [Development](DEVELOPMENT.md), then [Local testing](LOCAL_TESTING.md).
3. Use [Architecture](ARCHITECTURE.md) and [API reference](API_REFERENCE.md) when changing an interface.
4. For exhaustive file/function behavior, open [Component functional index](COMPONENT_FUNCTIONAL_INDEX.md) and the three `*_FUNCTIONAL_REFERENCE.md` documents.
5. Read the agent and panel overview documents before altering Android permissions, local browser storage, or WebSocket behavior.

## Documentation rules

- A route or command is called **implemented** only when the current server and agent code both handle it.
- A server route may be marked **partial** when it exists but returns a placeholder response or does not persist the expected state.
- Documentation must not call the system production-ready while the release blockers in `SECURITY_AND_PRIVACY.md` remain unresolved.
- Any change to a public API, Android permission, data category, persistence behavior, or browser cache must update the relevant document in the same change.
