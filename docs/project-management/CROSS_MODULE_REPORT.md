# Cross Module Report

Session 03 owns business/service/domain semantics. Items here cross Controller, Repository, security, migration, or frontend/API boundaries and should be coordinated with the relevant parallel sessions before changing behavior.

- [ ] CM-0001 Duplicate announcement domain: `server/announcement.go` and `server/internal/features/system/announcement_service.go` contain nearly identical revision/publish/delete state machines and both write legacy `announcement.json`.
- [ ] CM-0002 Card status vocabulary split: `internal/cards`, `features/cards`, `features/sessions`, `features/authorizations`, admin overview, and repositories use overlapping but non-identical status strings including `archived` and legacy `disable`.
- [ ] CM-0003 Blacklist rule placement: signed API request handling checks machine blacklist, session activation checks card blacklist, repositories also support `ALL`; security and domain ownership must be clarified before consolidation.
- [ ] CM-0004 Audit-after-success coupling: several handlers return HTTP 500 when audit append fails after the business write succeeded, creating retry/idempotency risk across service and audit modules.
- [ ] CM-0005 Agent registration transaction boundary: `agents.Service.RegisterAgent` starts a SQL transaction through the repository then passes raw `*sql.Tx` into `InviteService`, crossing service/repository ownership.
- [ ] CM-0006 Release package path trust: release handler validates package paths while release store persists arbitrary paths; repository and controller responsibilities need alignment.
- [ ] CM-0007 Script release coupling: release creation/publish reads `scripts/active.json` directly from the release handler instead of going through the scripts service/repository API.
- [ ] CM-0008 Payload activation persistence: payload upload/activation updates in-memory state first and persists via storage afterward; controller responses may report success despite failed durable save.
- [ ] CM-0009 Session cleanup scheduler: root `APIHandler.SessionCleanupTask` runs background cleanup through sessions service; scheduler lifecycle and test ownership cross main/handler/service.
- [ ] CM-0010 Admin/agent password hashing: admin and agent services each implement bcrypt plus legacy SHA-256 migration with different length policies and error handling.
- [ ] CM-0011 Release update legacy path: `CheckVersion` is used as a legacy version gate from session activation and differs from `/api/v1/update/check` manifest/HMAC flow.
- [ ] CM-0012 Root handler migration boundary: root `AdminHandler`, `APIHandler`, and `agentCardManagerAdapter` still contain business rules while ADR-0002 says rules should live in service/usecase.
- [ ] CM-0013 Exam batch persistence atomicity: `exam.Handler.HandleReport` and `HandleCollect` write questions one row at a time through `exam.Repository`. Session 02 now returns HTTP 500 on the first insert failure instead of swallowing it, but earlier rows may already be committed. Service/repository ownership should add an atomic batch operation and define retry/idempotency semantics. No database schema change was made.
- [ ] CM-0014 Admin login token exposure vs compatibility: `AdminHandler.HandleLogin` sets an HttpOnly `admin_token` cookie and also returns the raw token in JSON. The bundled admin web client ignores the JSON token, so removing it would reduce XSS exposure, but external Bearer clients may depend on the field. Security and API versioning owners should decide whether to deprecate it or require an explicit token-response mode.
- [ ] CM-S04-0001 Invite deletion and multi-use attribution semantics (owner: Session 03): `InviteService.Delete` hard-deletes codes, while a multi-use invite keeps the first `used_by` but overwrites `used_at` on every use. Define retention/audit requirements and whether attribution should represent first use, last use, or a separate usage history before Session 04 changes durable behavior.
- [ ] CM-S04-0002 Invite list pagination contract (owner: Session 02): `/admin/api/invites` currently returns the full list with no filters or page parameters. Session 04 can provide indexed deterministic ordering, but API parameter names, limits, cursors, and response metadata require the API owner before repository pagination is exposed.

## Session 06 Frontend/API Boundary Notes

- [ ] CM-S06-0001 Frontend API contract review is in progress for `server/web/admin/src/main.ts`, `server/web/agent/src/main.ts`, and `server/web/shared/src/api.ts`. Any confirmed request/response mismatch will be recorded here instead of changing backend route contracts from this session.

## Session 05 Security Boundary Notes

- [ ] CM-S05-0001 Frontend/client bridge compatibility: page-context exam reporting previously received `window.__LQ_REPORT_CONFIG` with a raw session token and used direct XHR as a fallback. Session 05 is removing that credential exposure and retaining native bridge upload with bounded retry. Session 06 should verify that no frontend workflow depends on the removed global.
- [ ] CM-S05-0002 Frontend/client bridge compatibility: `runtime.getConfig` previously returned `baseUrl` and `apiKeyMasked`. Session 05 is removing both fields because page JavaScript must not receive credential fragments or potentially sensitive endpoint URLs. Session 06 should remove any display or diagnostics dependency on these fields.

## Session 01 Architecture Governance

- [x] CM-S01-0001 Session TTL configuration is only partially wired. Fixed in Session 03: `server/main.go` now wires `sessions.NewService(..., cfg.SessionTTL, ...)`; `sessions.NewService` and domain `cards.NewSessionService` fallbacks aligned to 4h (same as config default). Still recommend a Session 07/08 composition-root wiring test.
- [ ] CM-S01-0002 TLS certificate storage ignores `DATA_DIR`. `server/config.go` always reads or writes `certs/server.crt` and `certs/server.key` relative to the process working directory. Session 05 and Session 07 should define an explicit certificate directory/configuration and a rollback-compatible path migration before changing production behavior.
- [ ] CM-S01-0003 The unified C++ quality gate only runs CMake configure in the shared `build/` directory. It does not compile `ClientTests`, Hook regression targets, `CefHook`, or `Injector`, and it can mutate an existing developer cache. Session 07 and Session 08 should define an isolated check build directory and the minimum compile/test targets.
- [ ] CM-S01-0004 Shared infrastructure packages `internal/auth`, `internal/storage`, and `features/security/blacklist` currently report no package tests in the first Session 01 baseline run. Session 08 should add focused compatibility, atomic-write, path validation, replay, and global-blacklist coverage before Java migration relies on them.
- [ ] CM-S01-0005 The current Go module requires Go 1.26.5 while older architecture material and some progress notes referenced Go 1.25. Session 07 should align developer/CI toolchain declarations and confirm that release builders have the required Go version.
