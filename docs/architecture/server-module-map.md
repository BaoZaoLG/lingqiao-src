# Server Module Dependency Map

Status: active migration baseline  
Last reviewed: 2026-07-15

## Production Composition Root

`server/main.go` currently performs configuration loading, runtime initialization, JSON-to-SQLite migration, repository/service/handler construction, route registration, TLS server construction, session cleanup scheduling, signal handling, and graceful shutdown.

The target composition split for both Go cleanup and Java parity is:

```text
config load
  -> runtime paths and secrets
  -> database and legacy migration
  -> repositories
  -> services
  -> handlers and middleware
  -> admin/client server + agent server
  -> schedulers
  -> lifecycle and shutdown
```

No Java cutover should copy the current oversized `main.go` into one application class. Spring configuration should separate storage, security, feature wiring, routes, background jobs, and lifecycle.

## Module Ownership

| Module | Direct project dependencies | Persistence | Java migration position |
| --- | --- | --- | --- |
| `internal/httpapi` | none | in-memory rate-limit state | Implement early as filters, advice, response writers, and request helpers. |
| `internal/auth` | none | generated HMAC file plus process-local nonce state | High compatibility risk; golden vectors and replay semantics required. |
| `internal/audit` | `database/sql` | `lingqiao.db.audit_log` | Read path is an early candidate; write ownership waits for event semantics. |
| `features/cards` | `internal/cards`, `httpapi` | `lingqiao.db.cards` | Foundational domain; keep Go authoritative until lifecycle and transaction contracts pass. |
| `features/sessions` | `internal/cards`, `cardcode`, `httpapi` | `lingqiao.db.cards/sessions/blacklist` | Central dependency for client modules; implement contracts early, route late. |
| `features/authorizations` | cards and sessions narrow interfaces | shared cards/session repositories | Management aggregate only; must not become a second persistence owner. |
| `features/agents` | `httpapi` | `lingqiao.db.agents/invites` plus file token compatibility | Separate agent port and auth boundary; migrate after token ownership is decided. |
| `features/chat` | auth, sessions, blacklist, `storage.Store` | `DATA_DIR/chat.json` | Storage convergence is a prerequisite for Java writes. |
| `features/releases` | auth, sessions, audit, `internal/releases` | `DATA_DIR/releases.db`, package files, signing seed | Keep in Go until signed manifest and path golden tests pass. |
| `features/payloads` | auth, sessions, cards, audit, storage, crypto | JSON metadata, encrypted chunks, upload key | Last migration wave; custom crypto and large-file semantics are high risk. |
| `features/scripts` | auth, sessions, audit, filesystem repository | `DATA_DIR/scripts/` | Preserve legacy endpoint and active pointer; remove release direct file coupling first. |
| `features/exam` | auth, sessions, httpapi | `lingqiao.db.exam_records` | Read path and isolated sidecar tests are early candidates; write path needs atomic batch semantics. |
| `features/security/blacklist` | `database/sql` | `lingqiao.db.blacklist` | Define rule precedence across machine, card, and global entries before migration. |
| `internal/storage` | none | atomic JSON files under `DATA_DIR` | Compatibility adapter only; no new domain should adopt it. |
| `internal/releases` | SQLite and Ed25519 | `releases.db` | Cross-language selector, canonical JSON, hash, and signature vectors required. |
| `internal/platform/cardcode` | none | none | Pure algorithm; add golden vectors before Java implementation. |

## Shared Dependency Hotspots

- `sessions.Service` is consumed by chat, releases, payloads, scripts, exam, root DLL download, admin session views, and the cleanup scheduler.
- Root `APIHandler` remains a compatibility adapter across HMAC request parsing, blacklist checks, sessions, announcements, DLL source selection, and DLL encryption.
- Root `AdminHandler` composes the feature admin service with cards, sessions, agents, blacklist, audit, import/export, and compatibility routes.
- Release publication reads script compatibility files directly; this is tracked as `CM-0007`.
- Chat and payload metadata still depend on the shared JSON store; payload durability is tracked as `CM-0008`.
- Audit callbacks cross handler/service boundaries, and failure semantics are tracked as `CM-0004`.

## Authentication Boundaries

| Actor | Current model | Migration rule |
| --- | --- | --- |
| Client | PBKDF2-derived HMAC, timestamp, nonce, client ID, session binding | Preserve signed bytes, salt, iteration count, headers, clock window, replay behavior, and error contract. |
| Admin | SQLite-backed hashed session token, secure cookie or bearer token, permissions | Preserve token hashing, cookie attributes, permission names, expiry, and last-owner rules. |
| Agent | Agent token/cookie compatibility with separate listener and CORS policy | Preserve independent listener and do not merge admin/agent authorization contexts. |
| Exam page context | Session-token-only collection endpoint | Keep explicitly separate from HMAC-authenticated report ingestion. |

## Migration Waves

1. Common HTTP/error/security scaffolding and `/java/api/health`.
2. Shadow read for system health, audit, and exam projections.
3. Script repository read compatibility and legacy fixtures.
4. Cards/session DTO, repository, and service contract implementation without traffic ownership.
5. Admin-only low-risk read routing behind a rollback flag.
6. Agent, cards, and sessions only after auth and transaction compatibility.
7. Releases after selector, canonical manifest, Ed25519, HMAC, package path, and rollback tests.
8. Payloads and encrypted DLL delivery last.

## Required Golden Vectors

- Client HMAC: PBKDF2 salt/iterations/key length, signed string, timestamp window, nonce replay, simple GET compatibility.
- Card code: valid formatting, normalization, checksum/alphabet behavior, and existing stored values.
- Release: selector ordering, rollout bucket, canonical JSON, SHA-256, Ed25519 signature, manifest HMAC, and package path guard.
- Payload: PBKDF2 inputs, AES mode/padding, HMAC bytes, key exchange response, chunk hashes, and path layout.
- DLL: source selection, AES-GCM nonce/ciphertext/tag layout, key derivation, and response headers.
