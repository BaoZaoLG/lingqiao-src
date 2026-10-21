# Lingqiao Configuration

The Go production server reads configuration from environment variables. Relative paths are resolved from the process working directory, so production launchers should set an explicit working directory and absolute `DATA_DIR`.

| Variable | Default / fallback | Code owner | Runtime effect |
| --- | --- | --- | --- |
| `PORT` | `48901` | `server/internal/config/config.go` | Admin UI and client API HTTPS listener. |
| `AGENT_PORT` | `38472` | `server/internal/config/config.go` | Agent UI/API HTTPS listener. |
| `DATA_DIR` | `data` | `server/internal/config/config.go` | Main SQLite, release SQLite, JSON compatibility data, payloads, scripts, uploads, and generated application secrets. |
| `SESSION_TTL` | `4h` | `server/internal/config/config.go` | Intended shared TTL for admin, agent, and client sessions. See the known wiring gap below. |
| `ADMIN_PASSWORD` | generated when no stored hash exists | `server/internal/features/admin/service.go` | Seeds the initial owner account. The derived hash is compatible with `DATA_DIR/admin_password.hash`. |
| `HMAC_SECRET` | loaded or generated | `server/internal/auth/auth.go` | Client API root secret. Fallback file: `DATA_DIR/hmac_secret.key`. |
| `ADMIN_ORIGIN` | empty, same-origin only | `server/middleware.go` | Optional credentialed admin CORS origin. |
| `AGENT_ORIGIN` | empty, same-origin only | `server/middleware.go` | Optional credentialed agent CORS origin. |
| `CERT_HOST` | `127.0.0.1` | `server/config.go` | Subject/SAN input for generated self-signed TLS certificates. |
| `UPLOAD_KEY` | loaded or generated | `server/internal/features/payloads/handler.go` | Payload administration upload key. Fallback file: `DATA_DIR/upload_key`. |
| `UPDATE_SIGNING_SEED_HEX` | loaded or generated | `server/internal/features/releases/service.go` | Ed25519 release manifest seed. Fallback file: `DATA_DIR/update_manifest_seed.key`. |

## Runtime Files

| Path | Purpose | Backup / security rule |
| --- | --- | --- |
| `DATA_DIR/lingqiao.db` | Cards, sessions, blacklist, audit, agents, invites, exam, and admin state. | Back up consistently with the release database. |
| `DATA_DIR/releases.db` | Releases, packages, and release events. | Back up with `lingqiao.db`. |
| `DATA_DIR/data.json` | Legacy import source. | Retain only as required for rollback/import evidence. |
| `DATA_DIR/chat.json` | File-backed chat state. | Treat as runtime data until storage convergence. |
| `DATA_DIR/payloads.json`, `DATA_DIR/payloads/` | Payload metadata and encrypted chunks. | Sensitive runtime data; preserve permissions and consistency. |
| `DATA_DIR/scripts/` | Script versions and active pointer compatibility data. | Back up with releases when releases reference scripts. |
| `DATA_DIR/hmac_secret.key` | Generated client HMAC root secret. | Secret; never commit. |
| `DATA_DIR/upload_key` | Generated payload upload key. | Secret; never commit. |
| `DATA_DIR/update_manifest_seed.key` | Generated release signing seed. | Critical signing secret; never commit. |
| `certs/server.crt`, `certs/server.key` | Generated TLS certificate and private key. | Currently relative to the process working directory, not `DATA_DIR`; track as a deployment boundary issue. |

## Known Wiring Gap

`ConfigureRuntime` stores `SESSION_TTL`, and admin/agent construction uses it, but `server/main.go` currently constructs the client `sessions.Service` with a fixed 30-minute TTL. This is recorded in `../project-management/CROSS_MODULE_REPORT.md` for the business/configuration owners; documentation must not imply that the client session TTL is already controlled by `SESSION_TTL`.

Do not hardcode production secrets in source files. Java sidecar processes must remain read-only for these files until an explicit cutover decision defines a single writer.

## Go-to-Java routing

The Go production entrypoint reads the following variables. Every flag defaults to
`false`; production deployment should still write them explicitly so configuration
drift is visible.

| Variable | Shadow value | Purpose |
| --- | --- | --- |
| `JAVA_SIDECAR_BASE_URL` | `http://127.0.0.1:49001` | Loopback Java destination. |
| `JAVA_CUTOVER_ENABLED` | `false` | Global forwarding gate. |
| `JAVA_CUTOVER_<MODULE>` | `false` | Read-route ownership for one module. |
| `JAVA_CUTOVER_<MODULE>_WRITE` | `false` | State-changing route ownership for one module. |

Supported module names are `ADMIN`, `EXAM`, `AUDIT`, `SCRIPTS`, `CARDS`, `SESSIONS`,
`AGENTS`, `RELEASES`, `PAYLOADS`, `CHAT`, `BLACKLIST`, `ANNOUNCEMENT`, and
`AGENT_PORTAL`. Audit has no HTTP write route.

Use `/opt/lingqiao-java/bin/configure-go-shadow.sh` to add the complete disabled
matrix. The script backs up the existing environment file and refuses to overwrite
any non-shadow value.
