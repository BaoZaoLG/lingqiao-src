# Java API Compatibility Map

## Compatibility Rules

- Existing Go paths, methods, JSON field names, status codes, and response top-level fields stay compatible.
- Java endpoints are sidecar-only unless a feature flag explicitly routes traffic.
- New Java-only routes use `/java/*` until cutover.
- OpenAPI currently covers only part of the live API; Java migration must treat Go handlers as the source of truth.

## Client API

| Existing API | Go owner | Java target | Auth | Compatibility notes | Rollback |
| --- | --- | --- | --- | --- | --- |
| `POST /api/v1/activate` | `sessions.Handler` via `APIHandler` | `SessionController.activate` | HMAC body | Preserve `session_token`, `expires_at`, `card_expires_at`, `message`, optional `update` | Route to Go |
| `POST /api/v1/heartbeat` | `sessions.Handler` | `SessionController.heartbeat` | HMAC body | Preserve Unix seconds and update block | Route to Go |
| `POST /api/v1/deactivate` | `sessions.Handler` | `SessionController.deactivate` | HMAC body | Go returns OK even for missing/expired session | Route to Go |
| `GET /api/v1/announcement` | root `APIHandler` | `AnnouncementController.current` | none | Preserve `announcement: null` when empty | Route to Go |
| `GET /api/v1/dll` | root `APIHandler` | `DllDownloadController` | HMAC path + session headers | Binary response and headers must be byte-compatible before cutover | Route to Go |
| `GET /api/v1/script` | `scripts.Handler` | `ScriptController.download` | HMAC path + session | Preserve JavaScript response and optional JSON format | Route to Go |
| `POST /api/v1/update/check` | `releases.Handler` | `ReleaseController.check` | HMAC body + session | Preserve signed manifest payload and HMAC | Route to Go |
| `POST /api/v1/update/events` | `releases.Handler` | `ReleaseController.event` | HMAC body + session | Preserve required `release_id` and `type` validation | Route to Go |
| `GET /api/v1/update/package/{id}` | `releases.Handler` | `ReleasePackageController.download` | HMAC path + session | Preserve path guard and octet-stream headers | Route to Go |
| `POST /api/v1/chat/*` | `chat.Handler` | `ChatController` | HMAC body + session | Preserve blacklist check, mute/rate-limit messages, reaction names | Route to Go |
| `POST /api/v1/exam/report` | `exam.Handler` | `ExamController.report` | HMAC body + session | Low-risk candidate after tests | Disable Java exam flag |
| `POST /api/v1/exam/collect` | `exam.Handler` | `ExamController.collect` | session token only | No HMAC by design | Disable Java exam flag |
| `POST /api/v1/key-exchange` | `payloads.Handler` | `PayloadKeyController.exchange` | HMAC body + session | High-risk cryptographic compatibility | Route to Go |
| `GET /api/v1/payload/{id}/info` | `payloads.Handler` | `PayloadController.info` | HMAC path | Preserve `payload_id`, counts, hash | Route to Go |
| `GET /api/v1/payload/{id}/chunk/{n}` | `payloads.Handler` | `PayloadController.chunk` | HMAC path | Binary chunk compatibility required | Route to Go |

## Admin API

| Existing API | Go owner | Java target | Permission | Compatibility notes | Rollback |
| --- | --- | --- | --- | --- | --- |
| `POST /admin/api/login` | `AdminHandler` | `AdminAuthController.login` | none | Preserve `admin_token` cookie and `token` body | Route to Go |
| `POST /admin/api/password` | `AdminHandler` | `AdminAuthController.changePassword` | session | Preserve password limits | Route to Go |
| `GET/POST /admin/api/admin-users` | `AdminHandler` | `AdminUserController` | `admins.write` | Do not expose password hash | Route to Go |
| `POST /admin/api/admin-user/update` | `AdminHandler` | `AdminUserController.update` | `admins.write` | Preserve last owner protection | Route to Go |
| `GET /admin/api/cards` | `cards.Handler` | `CardAdminController.list` | `cards.read` | Preserve filters and status calculation | Route to Go |
| `POST /admin/api/card/generate` | `cards.Handler` | `CardAdminController.generate` | `cards.write` | Preserve clamp ranges | Route to Go |
| `POST /admin/api/card/batch-generate` | `cards.Handler` | `CardAdminController.batchGenerate` | `cards.write` | Preserve max count 500 | Route to Go |
| `POST /admin/api/card/update` | `cards.Handler` | `CardAdminController.update` | `cards.write` | Preserve action names | Route to Go |
| `POST /admin/api/card/update-details` | `cards.Handler` | `CardAdminController.updateDetails` | `cards.write` | Preserve omitted field detection | Route to Go |
| `POST /admin/api/cards/bulk` | `cards.Handler` | `CardAdminController.bulk` | `cards.bulk` | Preserve dry-run result shape | Route to Go |
| `GET /admin/api/sessions` | `sessions.Handler` | `SessionAdminController.list` | `sessions.read` | Preserve `card` alias field | Route to Go |
| `POST /admin/api/force-logout` | `sessions.Handler` | `SessionAdminController.forceLogout` | `sessions.force_logout` | Preserve `session_token` request | Route to Go |
| `GET/POST /admin/api/blacklist` | `AdminHandler` | `BlacklistController` | `system.read` | Preserve `action=add/remove` | Route to Go |
| `GET /admin/api/audit` | `AdminHandler` | `AuditController.list` | `audit.read` | Read-only candidate | Disable Java audit |
| `GET/POST /admin/api/announcement` | `AdminHandler` | `AnnouncementAdminController` | announcement permissions | File format compatibility required | Route to Go |
| `GET/POST /admin/api/scripts*` | `scripts.Handler` | `ScriptAdminController` | `scripts.write` | Keep legacy `/admin/api/script` | Route to Go |
| `GET/POST /admin/api/releases*` | `releases.Handler` | `ReleaseAdminController` | `releases.write` | High-risk signed release flow | Route to Go |
| `GET/POST /admin/api/payload*` | `payloads.Handler` | `PayloadAdminController` | `payloads.write` | High-risk file/crypto flow | Route to Go |
| `GET /admin/api/exam/records` | `exam.Handler` | `ExamAdminController.list` | `exam.read` | Low-risk read candidate | Disable Java exam |
| `GET /admin/api/exam/stats` | `exam.Handler` | `ExamAdminController.stats` | `exam.read` | Low-risk read candidate | Disable Java exam |

## Agent API

| Existing API | Go owner | Java target | Auth | Compatibility notes | Rollback |
| --- | --- | --- | --- | --- | --- |
| `POST /api/register` | `agents.AgentHandler` | `AgentController.register` | invite | Preserve username/password validation and invite transaction | Route to Go agent server |
| `POST /api/login` | `agents.AgentHandler` | `AgentController.login` | none | Preserve `agent_token` cookie and token map compatibility | Route to Go agent server |
| `GET /api/dashboard` | `agents.AgentHandler` | `AgentController.dashboard` | agent token | Preserve count fields | Route to Go |
| `GET /api/cards` | `agents.AgentHandler` | `AgentController.cards` | agent token | Preserve card snapshot | Route to Go |
| `POST /api/card/generate` | `agents.AgentHandler` | `AgentController.generateCard` | agent token | Preserve tighter agent limits | Route to Go |
| `POST /api/card/batch-generate` | `agents.AgentHandler` | `AgentController.batchGenerate` | agent token | Preserve max count 50 | Route to Go |
| `GET /api/info` | `agents.AgentHandler` | `AgentController.info` | agent token | Preserve nested `agent` object | Route to Go |
| `POST /api/password` | `agents.AgentHandler` | `AgentController.changePassword` | agent token | Preserve 6-72 password limit | Route to Go |

