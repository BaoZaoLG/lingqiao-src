# Lingqiao Platform Upgrade Design

Date: 2026-06-06
Status: Approved design draft

## Goal

Upgrade Lingqiao from a single-package Go server with embedded static pages into a maintainable platform for license/card operations, agent self-service, auditing, update management, and operational administration.

The upgrade must preserve existing business data and current client API compatibility where practical. It must improve maintainability, observability, security posture, and frontend usability without expanding injection, evasion, stealth delivery, or bypass capability.

## Current State

The current server lives under `_src_tmp/injector-server` and is a single Go package. It embeds two static frontends:

- `admin/` for administrator operations.
- `agent/` for agent registration, login, and card generation.

The backend stores business data in JSON files under `data/`, including cards, sessions, agents, invites, payload metadata, announcements, update metadata, and password/session files.

Existing tests pass when `GOCACHE` is redirected into the workspace:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./...
```

The workspace root is not currently a git repository, so design and implementation changes cannot be committed from this directory unless a repository is initialized or a git-backed target directory is provided.

## Scope

This platform upgrade includes:

- Backend package restructuring.
- Business service boundaries for cards, agents, sessions, audit, updates, and storage.
- Safer authentication, authorization, session, upload, CORS, and audit behavior.
- A modern frontend architecture for administrator and agent panels.
- Compatibility loading for existing JSON data.
- Tests for core business logic and HTTP behavior.
- Build and operation documentation.

This platform upgrade excludes:

- Enhancing DLL injection behavior.
- Adding stealth, persistence, bypass, unpacking, or anti-detection features.
- Making payload delivery more covert.
- Weakening authorization, logging, or operator visibility.

## Architecture

### Backend Layout

The Go backend will move from one package into focused packages:

```text
cmd/server
internal/config
internal/httpapi
internal/auth
internal/cards
internal/agents
internal/audit
internal/storage
internal/updates
internal/health
web/admin
web/agent
web/shared
```

`cmd/server` owns process startup, configuration loading, TLS setup, server lifecycle, graceful shutdown, and dependency wiring.

`internal/config` owns environment parsing, defaults, validation, and config structs.

`internal/httpapi` owns route registration, middleware, request decoding, response encoding, API errors, and frontend static serving.

`internal/auth` owns administrator and agent authentication, password hashing, session management, cookie policy, rate limiting, and authorization helpers.

`internal/cards` owns card generation, activation, heartbeat, deactivation, binding, status transitions, bulk operations, and machine/card lookup rules.

`internal/agents` owns agent accounts, invite codes, agent card ownership, and agent-level card generation limits.

`internal/audit` owns audit event models, append/query APIs, retention, and archive behavior.

`internal/storage` owns persistence interfaces and implementations. The first implementation remains JSON-backed with atomic writes and backups. The public interface should allow a future SQLite backend.

`internal/updates` owns client update metadata, update package validation, checksums, and update download metadata.

`internal/health` owns health status, runtime stats, dependency checks, and admin-facing diagnostics.

### Dependency Direction

Handlers call service interfaces. Services call repositories and audit interfaces. Storage does not import business packages.

HTTP-specific concepts such as cookies, headers, remote IP, and request bodies must stay in `internal/httpapi` or auth middleware. Business packages receive typed inputs and return typed results.

### Server Compatibility

The existing external HTTP routes should remain available during the first migration:

- Client API routes under `/api/v1/...`.
- Admin API routes under `/admin/api/...`.
- Agent API routes under `/api/...` on the agent server.
- Admin frontend under `/admin/`.
- Agent frontend under `/`.

Any route rename must be additive first, then deprecated later.

## Data Model and Migration

The first platform release keeps JSON persistence to protect existing deployments.

`internal/storage` will load existing files:

- `data/data.json`
- `data/admin_sessions.json`
- `data/agent_sessions.json`
- `data/agent_token_map.json`
- `data/invites.json`
- `data/announcement.json`
- `data/payloads.json`
- `data/updates/info.json`
- `data/admin_password.hash`
- `data/hmac_secret.key`
- `data/upload_key`

Migration rules:

- Missing files are treated as empty state.
- Unknown JSON fields are preserved where possible or ignored safely.
- Legacy password hashes continue to auto-migrate to bcrypt after successful login.
- Existing card codes, machine bindings, agent IDs, and session semantics remain compatible.
- Backup files are written before destructive replacement.
- Corrupt files return explicit startup or health warnings rather than silent partial state when the file is critical.

SQLite is a future storage backend. It should be enabled by the storage interface design, not forced into the first implementation batch.

## Business Behavior

### Card Management

Card services will expose typed operations:

- Generate one card.
- Generate card batch.
- List cards with status, agent, search, and date filters.
- Update card status.
- Update note, expiration, max session count, and ownership fields.
- Bulk enable, disable, expire, extend, and unbind.
- Activate with version policy checks.
- Heartbeat with session renewal and version policy checks.
- Deactivate by session token.

Bulk operations return per-item results, including success, skipped, and failed counts.

### Agent Management

Agent services will expose:

- Invite creation, listing, deletion, and usage tracking.
- Registration with invite validation.
- Login and session creation.
- Agent disable/enable.
- Agent card listing scoped to ownership.
- Agent card generation with configurable limits.
- Password change with session invalidation.

### Audit

Audit events will cover high-risk and business-critical actions:

- Admin login success/failure.
- Agent login success/failure.
- Password changes.
- Card generation, update, disable, enable, expire, extend, unbind.
- Batch operations.
- Invite creation/deletion/use.
- Agent disable/update.
- Version metadata changes.
- Upload attempts and results.
- Blacklist changes.

Audit query supports action, actor, card, agent, machine, IP, and time-range filters.

### Updates

Update management will remain focused on authorized client version policy:

- Latest version.
- Minimum allowed version.
- Force update flag.
- Package metadata.
- SHA-256 checksum.
- Upload size/type validation.
- Download route with explicit metadata.

It will not add covert delivery, evasion, or unauthorized execution features.

## Security Design

Authentication and session behavior will be centralized:

- Administrator and agent sessions are stored as token hashes.
- Cookies are `HttpOnly`, `Secure`, `SameSite=Strict` by default.
- Session TTL, cleanup, and invalidation are implemented in one package.
- Password minimum length and max length are configurable with safe defaults.
- Bcrypt remains the password hashing default.
- Login, registration, password change, activation, and upload routes have rate limits.
- CORS defaults to same-origin for credentialed admin and agent pages.
- Request body limits are centralized by route class.
- Upload targets are constrained to known directories.
- Security headers and CSP are applied consistently.
- API errors are consistent and do not leak sensitive internals.

CSRF protection should be added for cookie-authenticated admin and agent mutation routes. The preferred first implementation is a same-site cookie plus per-session CSRF token returned by an authenticated bootstrap endpoint and required on unsafe methods.

## Frontend Design

The frontend will move to Vite + TypeScript with two application entries:

```text
web/admin
web/agent
web/shared
```

The built static assets will be embedded by the Go server.

Shared frontend code includes:

- API client.
- Error normalization.
- Toasts and notifications.
- Modal primitives.
- Form validation helpers.
- Table state helpers.
- Date/time formatting.
- Status badge formatting.
- Auth bootstrap and logout flow.

Admin frontend views:

- Login.
- Dashboard.
- Card management.
- Sessions.
- Machines.
- Blacklist.
- Announcements.
- Version updates.
- Agents.
- Invites.
- Audit.
- System health/settings.

Agent frontend views:

- Login/register.
- Dashboard.
- My cards.
- Generate card.
- Batch generate.
- Password change.
- Invite/account status.

The UI style should be an operational console: dense but readable, predictable navigation, restrained color, clear table controls, and strong loading/empty/error states.

## API Design

The first platform release keeps current routes compatible. New internal handlers should return a consistent response shape:

```json
{
  "status": "ok",
  "data": {},
  "meta": {}
}
```

Errors should return:

```json
{
  "status": "error",
  "message": "human readable message",
  "code": "MACHINE_READABLE_CODE"
}
```

Existing clients that depend on current response fields should continue to receive them on `/api/v1/...` routes.

## Testing

Backend tests:

- Storage load/save and backup behavior.
- Password verify and legacy migration.
- Session creation, lookup, expiry, and invalidation.
- Card lifecycle and bulk operations.
- Agent registration, invite use, login, disable behavior.
- Audit append/query behavior.
- Handler tests for admin, agent, and client API routes.
- CORS, security headers, body limits, and method checks.

Frontend tests:

- API client response/error normalization.
- Core form validation.
- Table filtering/sorting state.
- Auth bootstrap behavior.

End-to-end verification:

- `go test ./...`
- Frontend build.
- Server startup.
- Admin login flow.
- Agent register/login flow.
- Card generation.
- Client activate/heartbeat compatibility test using signed requests.

## Implementation Phases

### Phase 1: Platform Foundation

- Add config package.
- Add storage interfaces and JSON implementation.
- Add typed API response helpers.
- Add auth session service.
- Add audit service shell.
- Wire existing handlers through new dependencies with minimal behavior change.
- Keep existing static frontend operational.

### Phase 2: Business Services

- Move card logic into `internal/cards`.
- Move agent and invite logic into `internal/agents`.
- Move update metadata logic into `internal/updates`.
- Add per-operation audit events.
- Add bulk operation result models.
- Expand tests around business services.

### Phase 3: Frontend Modernization

- Add Vite + TypeScript workspace.
- Build shared API and UI helpers.
- Rebuild admin panel views.
- Rebuild agent panel views.
- Embed built assets in Go.
- Preserve existing routes.

### Phase 4: Operations and Hardening

- Add health diagnostics.
- Add config example and README.
- Add build scripts.
- Add startup validation.
- Add CSRF token flow.
- Add upload validation hardening.
- Add data migration checks and warnings.

## Acceptance Criteria

- Existing tests pass.
- New backend tests cover auth, cards, agents, storage, audit, and handlers.
- Existing client activation and heartbeat routes stay compatible.
- Existing JSON data loads successfully.
- Admin and agent frontends build and are served by Go.
- Admin can manage cards, agents, invites, sessions, audit, announcements, and updates.
- Agent can register, log in, generate/list cards, batch generate, and change password.
- Security headers, CORS, body limits, rate limits, and session cookies are consistently applied.
- High-risk actions emit audit events.
- Documentation explains configuration, build, run, migration, and test commands.

## Risks

- Frontend modernization introduces Node tooling and build complexity.
- Package restructuring may create a large diff if done in one pass.
- JSON compatibility requires careful migration tests.
- Existing data files include secrets and binary artifacts; repository hygiene must be handled separately.
- Because the workspace root is not a git repository, changes cannot be committed until git state is resolved.

## Open Decisions

- Whether to initialize a git repository in `C:\Users\Li\Downloads\Lingqiao_src` or move work into an existing git-backed directory.
- Whether to keep JSON only for the first full release or add SQLite behind the storage interface immediately after JSON migration tests pass.
- Whether to keep admin and agent on separate ports or eventually serve both from one server with path-based routing.
