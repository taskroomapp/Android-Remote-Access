# Security and privacy assessment

> **Release decision: do not deploy this repository to production or expose it beyond an isolated, authorized development environment.**
>
> The code handles privileged Android permissions and potentially sensitive data, while several authentication, authorization, audit, storage, and transport controls are incomplete or internally inconsistent. This document records current behavior so that the project is not represented as a production-ready control plane.

## Assessment verdict

| Area | Status |
| --- | --- |
| Encryption design (session + at-rest) | **Strong and substantially hardened** — see [Encryption coverage](ENCRYPTION.md) and [Session encryption](SESSION_ENCRYPTION.md) |
| Production readiness | **Not ready** — authorization, transport, audit, path policy, browser storage, and operational secret management blockers remain |

### Device WebSocket identity (canonical wording)

Use this wording everywhere; do not call the initial upgrade an “authenticated WebSocket connection”:

> The URL and `X-Device-UUID` identify a connection candidate. The device is not authenticated until the enrolled identity and signed handshake transcript are verified. Commands are rejected until `session_ready`.

Current boundary (accurate, not contradictory):

- A peer can open a socket (upgrade is not credential-gated).
- It cannot issue or receive commands before `session_ready`.
- The signed key exchange authenticates the device for the application session.
- TLS client certificates are requested, not verified as mutual TLS.
- Unauthenticated socket open remains a DoS/probing risk until connection-time credentials or mTLS exist.
- The WebSocket path must **not** auto-create or reconcile devices from unauthenticated metadata before identity verification (current auto-create/reconcile behavior is a release blocker).

## Required use boundary

The only acceptable current use is isolated engineering evaluation involving:

- a disposable emulator, or a device owned by the evaluator;
- explicit, informed, revocable consent for each sensitive category used in the test;
- a loopback/private development network, never a public listener;
- synthetic or minimal test data; and
- prompt deletion of local browser/server test data afterward.

It must not be used for covert monitoring, employee monitoring without a reviewed lawful basis and notice, monitoring of another person's private device, or collection beyond a documented support/investigation scope.

## Current controls present in source

| Area | Present behavior | Limitation |
| --- | --- | --- |
| Admin password storage | bcrypt via `golang.org/x/crypto/bcrypt` | Default development administrator is created with a known password. |
| Admin sessions | HS256 JWT access/refresh pair plus PostgreSQL refresh-session row | Tokens in browser `localStorage`; admin WS accepts JWT in query string; logout does not delete the PostgreSQL session. |
| Device / admin session AEAD | CKX1: X25519 + HKDF directional keys + ChaCha20-Poly1305 `type=enc` + Ed25519 handshake signatures. Panel uses the same construction after `/auth/ckx1/*`. See [Session encryption](SESSION_ENCRYPTION.md). | Connection candidate ≠ authenticated session until `session_ready`. Identity keys are long-term (not ephemeral DH); compromise of long-term X25519 private key affects future agreements. No connection-time mTLS. |
| Data at rest | `DataEncryptor` seals sensitive fields with `AT1:` ChaCha20-Poly1305 and record-bound AAD. Fail-closed on write. See [Encryption coverage](ENCRYPTION.md). | Admin APIs return decrypted plaintext inside the CKX1/TLS channel. Browser IndexedDB caches remain outside this layer. At-rest key-ID rotation strategy not yet implemented. |
| Audit tables | PostgreSQL trigger rejects UPDATE/DELETE on `audit_logs` | Trigger conflicts with dispatcher response updates — incomplete command audit trail. |
| TLS | Server supports TLS 1.2+ | Development permits cleartext loopback; production-mode startup enforcement of HTTPS/WSS is not implemented. `RequestClientCert` is not mTLS. |
| Runtime Android permissions | Agent checks many specific Android permissions before command execution | Main UI asks for a broad group at once instead of contextual, feature-level consent. |
| Foreground service | Android uses a visible `dataSync` foreground service and notification | Lifecycle/reconnect/boot design still needs user-control and revocation review. |
| Roles | Fixed role-to-permission map | Not device-scoped; stored per-administrator `permissions` are ignored. |
| Offline commands | PostgreSQL `pending_commands` with 24-hour expiry | Sensitive actions can be queued without a fresh approval model. |
| File access | Agent uses platform `File` paths; server forwards paths | No canonical storage-root / traversal policy on server or agent. |

## Release blockers

### P0 — resolve before sensitive deployment

| Finding | Evidence in source | Risk | Required remediation |
| --- | --- | --- | --- |
| Device WebSocket upgrade is not credential-gated | `/ws/devices/{id}` upgrades without connection-time credential; TLS uses `RequestClientCert` only | Socket probing/DoS; URL identity confused with auth | Require and validate client certs, or enrollment-bound short-lived connection ticket/credential; stop auto-create/reconcile before verified identity |
| Authorization is not device-scoped | Role maps only; no administrator→device→command checks | Any capable role can act on all registry devices | Server-side checks on every command, file, contacts/SMS/calls, location, camera/mic, media download, artifact export, mirror, enroll/revoke path |
| Audit implementation is internally broken | Trigger forbids all updates; dispatcher updates response fields | Incomplete/missing terminal audit events | Append-only event model **or** mutable status table + immutable audit events; do not simply drop the trigger for unrestricted updates |
| File access lacks canonical storage policy | Raw paths on agent; no server storage root | Traversal / out-of-policy read/write/delete | Canonicalize; allowed roots; reject traversal; symlink policy; size/duration limits; blocked paths; per-device policy; authorize before dispatch; audit requested and resolved paths |

### P1 — strongly recommended before release

| Finding | Evidence in source | Risk | Required remediation |
| --- | --- | --- | --- |
| Limited forward secrecy (static identity DH) | Long-term server/device X25519 identity keys | Compromise of identity private keys can affect past recorded handshakes that used those keys | Consider ephemeral X25519 per connection in a future CKX revision; document as per-connection key separation today |
| Admin JWT in WebSocket query string | `/ws/admin?token=` | Leak via proxies, logs, history, tooling | Short-lived single-use WS ticket over HTTPS, or Secure cookie / `Sec-WebSocket-Protocol`; never log query strings |
| Origin validation accepts every origin | `CheckOrigin` always `true` | Cross-origin WS abuse | Allow only configured origins; pair with CSRF protections for REST |
| Browser stores tokens and sensitive caches | `localStorage` + IndexedDB | XSS/profile sharing exposure | In-memory access token; HttpOnly refresh cookie; no default PII/media caches; clear on logout/switch; CSP |
| TLS not enforced at startup for production | Docs say mandatory; defaults allow `http`/`ws` | Accidental cleartext deployment | Reject non-loopback cleartext and disabled TLS in production mode; validate cert paths; TLS 1.2+ |
| Secrets are YAML-only | `config.yaml` encryption/JWT values | Committed or weakly managed secrets | Env/secret-manager injection; reject placeholders; never log secret material |

### P2 — correctness and operations

| Finding | Required remediation |
| --- | --- |
| At-rest key rotation | Key-ID versioning; re-encrypt plan; refuse plaintext fallback; audit completion |
| At-rest key rotation | Document/implement key IDs on envelopes; re-encrypt vs dual-key window strategy |
| Static AAD must not return | Record-bound AAD only; mismatch fails closed (no purpose-only fallback except gated legacy migration) |
| Admin plaintext responses | Acceptable only with mandatory TLS, device-scoped auth, audited sensitive routes, `Cache-Control: no-store`, no body logging |
| Media download hardening | Accurate content type; safe `Content-Disposition`; auth at download; audit; no decrypted temp files; size/rate limits |

### Other critical / high (privacy and operations)

| Finding | Evidence in source | Risk | Required remediation |
| --- | --- | --- | --- |
| Broad sensitive capability without feature-specific consent | Manifest + `MainActivity` bulk permission flow | Over-collection / coercive use | Granular disclosure, activation, indication, revocation; legal/privacy review |
| Default development credentials | Known `admin` password; committed placeholders | Immediate compromise on accidental deploy | Remove default admin outside tests; secret management; rotate |
| API rate limit config unused | YAML/Redis primitive unused | Brute-force / exhaustion | Per-IP/account/device limits and body-size limits |
| Sensitive offline command replay | Auto-dispatch of queued commands | Actions outside approval window | Fresh approval; never queue high-risk by default |
| Android transport trust incomplete | No demonstrated pinning / verified client identity at connect | MITM depending on deployment | Pinning/attestation where appropriate; enrollment-bound client credentials |

## Next implementation order

Until the following are done, keep labeling the system **development-only**:

```text
1. Fix audit event persistence (append-only events or status + immutable events)
2. Add device-scoped authorization on every sensitive route/dispatcher path
3. Enforce canonical file roots (server + agent)
4. Replace admin WS query JWT with a short-lived ticket (or equivalent)
5. Implement strict WebSocket origin validation
6. Enforce HTTPS/WSS in production mode at startup
7. Remove or tightly control browser PII caching
8. Decide whether X25519 forward secrecy is required for the threat model
```

## Privacy engineering requirements

Before a production design can be considered, define and review:

1. **Data inventory:** exact fields, purpose, lawful basis, recipient, storage location, retention, deletion, and access logging for each category.
2. **Consent and notification:** in-app feature-specific notice, active-state indicator, separate controls, revoke/disconnect/uninstall instructions, and no dark patterns.
3. **Authorization:** device-to-organization binding; role, individual, device, data-category, and time-bound scope; dual authorization for sensitive actions.
4. **Minimization:** no bulk collection by default; pagination/limits; no raw media or full contact/SMS history unless explicitly necessary and authorized.
5. **Retention:** automatic TTL and secure deletion for server objects, browser cache, logs, backups, and failed/queued operations.
6. **Subject rights and operations:** export, correction, deletion, access review, incident handling, and a tested emergency disable/revocation process.
7. **Vendor review:** map tiles, reverse geocoding, hosting, certificate authority, and analytics vendors must be approved before any real data is sent.

## Secure redesign phases

### Phase 1 — stop unsafe release paths

- Remove known default credentials and committed placeholder passwords/secrets.
- Bind configuration to a secret-management mechanism and fail closed when required values are absent.
- Disable/omit sensitive command categories and unfinished routes from release builds.
- Restrict server listener/network access to a private environment; eliminate cleartext except emulator tests.
- Add explicit user-facing consent/disconnect/revocation controls to Android.

### Phase 2 — identity and authorization

- Establish a secure enrollment ceremony with verified device credentials and rotation/revocation.
- Keep authenticated key-exchange verification, and add verified device authentication at WebSocket connection time (mTLS or short-lived ticket).
- Stop device auto-create/reconcile from unauthenticated WebSocket metadata.
- Implement resource-scoped RBAC/ABAC using authoritative stored grants and device ownership.
- Replace query-token WebSocket authentication and permissive origin policy.
- Add MFA/SSO, session revocation, rate limits, and abuse monitoring for admins.

### Phase 3 — data protection and audit

- Replace the conflicting audit trigger with a true append-only event architecture (or mutable status + immutable events).
- Encrypt protected categories with managed keys, key IDs, rotation, and access controls.
- Implement retention/deletion jobs and test backup handling.
- Harden media retrieval (authorization, audit, cache headers, no temp plaintext).
- Add immutable evidence export only after the legal/privacy model is approved.

### Phase 4 — assurance

- Add automated tests for authorization, input/path validation, protocol parsing, concurrency, timeouts, queues, and retention.
- Add CI (see [Development](DEVELOPMENT.md)), dependency scanning, SAST, SBOM, secret scanning, and release signing.
- Perform independent application security, mobile security, privacy, and legal reviews.
- Run a documented threat model and penetration test before any limited pilot.

## Operational stop conditions

Stop a test immediately and rotate credentials/review logs if any of the following happens:

- a device is enrolled without the device owner’s documented consent;
- the service is accidentally reachable from an untrusted network;
- any default/committed secret is used outside a disposable test environment;
- sensitive production data is stored in the browser, emulator, local PostgreSQL, or test logs;
- an authentication, authorization, or certificate-validation anomaly occurs; or
- a device user asks to disconnect/revoke access.

## Documentation ownership

This file must be updated whenever a security control, data category, Android permission, auth/authorization rule, storage location, third-party network call, or deployment assumption changes. A finding may be downgraded only with linked code, tests, and independent review evidence.
