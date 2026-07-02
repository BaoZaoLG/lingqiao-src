# Authorization Control Plane Design

Date: 2026-07-02

## Decision Summary

The first V3 control-plane phase replaces the old admin "card management" page with an authorization control plane.

This is not another visual redesign of the same CRUD table. The admin-facing business object changes from `Card` to `Authorization`. Existing card storage remains compatible during migration, but the user experience, API shape, and domain language move to authorization lifecycle management.

The chosen direction is:

- Full backend and admin control-plane redesign, delivered in phases beside the existing service.
- Phase 1 starts with authorization because card validity, activation, binding, sessions, deletion, and audit are the core operational pain points.
- The old card page is replaced directly, not kept as a parallel primary workflow.
- Deletion defaults to archived deletion, preserving audit and historical context.

## Goals

1. Make the admin console answer operational questions quickly:
   - Can this authorization be used?
   - Why did activation fail?
   - Which machine and sessions are tied to it?
   - Which scripts, payloads, and release rules affect the client using it?
   - What happens if an admin freezes, unbinds, extends, or archives it?
2. Replace the old card table with a lifecycle-oriented authorization view.
3. Make status trustworthy by calculating effective status from domain facts instead of trusting one database field.
4. Preserve existing production data and existing client behavior during migration.
5. Keep all current card-management capabilities, but move them into safer flows.

## Non-Goals

1. Do not delete or rewrite production card data as part of Phase 1.
2. Do not require immediate client changes for existing activation and heartbeat endpoints.
3. Do not migrate every admin module in this phase.
4. Do not introduce microservices or split deployment units.
5. Do not remove legacy compatibility routes until the new control plane has test coverage and production verification.

## Domain Model

### Authorization

`Authorization` is the admin-facing domain object. It is initially backed by the existing `cards` table plus related records from sessions, agents, blacklist, scripts, payloads, releases, and audit.

Core fields:

- `id`: stable authorization identifier, initially the card code.
- `code`: card code shown to operators and accepted by clients.
- `origin`: `generated`, `imported`, or future source types.
- `agent_id`: optional issuing agent.
- `created_at`
- `activated_at`
- `expires_at`
- `max_sessions`
- `machine_id`
- `note`
- `lifecycle_status`: stored administrative state, such as active, frozen, archived.
- `effective_status`: computed state shown to operators.
- `risk_flags`: computed operational flags.
- `deleted_at`: set for archived deletion.
- `deleted_by`: admin identifier when available.
- `delete_reason`: optional operator reason.

### Lifecycle Stages

The UI uses lifecycle stages instead of raw database statuses:

- `issued`: generated or imported, not yet activated.
- `activated`: bound to a machine, but no active session right now.
- `in_use`: has active runtime sessions.
- `expiring`: active but close to `expires_at`.
- `expired`: `expires_at <= now`.
- `frozen`: manually disabled or blocked.
- `archived`: hidden from normal lists and unusable by clients.

### Effective Status Rule

Effective status is derived in one place:

1. If archived, show `archived`.
2. If blacklisted, show `blocked`.
3. If manually disabled or frozen, show `frozen`.
4. If `expires_at <= now`, show `expired`.
5. If active sessions exist, show `in_use`.
6. If machine bound or activation history exists, show `activated`.
7. Otherwise show `issued`.

The backend owns this computation. The frontend may do defensive display fallback, but it must not be the source of truth.

## Backend Design

### Package Boundary

Add a new control-plane layer without breaking existing feature services:

```text
server/internal/features/authorizations/
  model.go
  service.go
  impact.go
  repository.go
  handler_admin.go
  status.go
  service_test.go
  handler_admin_test.go
```

This package is a domain aggregation layer. It may depend on repository interfaces from cards, sessions, agents, audit, blacklist, releases, scripts, and payloads through narrow interfaces. It should not duplicate their persistence logic.

### Service Responsibilities

`authorizations.Service` owns:

- Effective status calculation.
- Authorization search and list projections.
- Authorization detail projection.
- Lifecycle timeline construction.
- Impact preview for dangerous actions.
- Archived deletion workflow.
- Freeze, restore, unbind, extend, and batch operation orchestration.

Existing `cards.Service` remains the lower-level compatibility service for card persistence and client activation behavior during Phase 1.

### Repository Strategy

Phase 1 does not require a destructive schema migration.

Preferred additive fields for SQLite:

- `cards.deleted_at TEXT NULL`
- `cards.deleted_by TEXT NULL`
- `cards.delete_reason TEXT NULL`
- `cards.lifecycle_status TEXT NULL`

If adding columns is too risky for the first deployment, archived deletion can be stored in a separate `authorization_archives` table keyed by card code:

```text
authorization_archives(
  code TEXT PRIMARY KEY,
  deleted_at TEXT NOT NULL,
  deleted_by TEXT,
  reason TEXT
)
```

The service should hide archived authorizations from normal lists, reject client usage, and include archived items only when explicitly requested.

### Admin API

Add V3 admin control-plane routes under `/admin/api/control/authorizations`.

Initial endpoints:

- `GET /admin/api/control/authorizations`
  - list authorization summaries
  - filters: query, effective_status, lifecycle_stage, agent_id, bound, expires_before, expires_after, include_archived
- `GET /admin/api/control/authorizations/{code}`
  - detail view projection
  - includes summary, sessions, machine, agent, lifecycle timeline, audit, risk flags, troubleshooting notes
- `POST /admin/api/control/authorizations`
  - generate one or more authorizations
- `POST /admin/api/control/authorizations/import/preview`
  - validate import before writing
- `POST /admin/api/control/authorizations/import`
  - import authorizations after preview
- `POST /admin/api/control/authorizations/{code}/impact`
  - preview action impact
- `POST /admin/api/control/authorizations/{code}/actions`
  - execute action after preview
- `POST /admin/api/control/authorizations/bulk/impact`
  - preview batch operation
- `POST /admin/api/control/authorizations/bulk/actions`
  - execute batch operation

Supported actions:

- `extend`
- `unbind_machine`
- `freeze`
- `restore`
- `archive_delete`
- `purge_delete` is excluded from Phase 1 and requires a separate future design approval

### Impact Preview

Dangerous actions must return an impact preview before execution.

Preview includes:

- target authorization count
- active sessions that will be terminated
- machines that will lose authorization
- agent statistics affected
- audit entries that will be written
- whether the action is reversible
- warnings and required confirmation phrase for high-risk actions

Archived deletion is reversible only if implemented as archive metadata over the card row. Physical purge is irreversible and is not part of Phase 1 default workflow.

### Audit

Every state-changing action writes audit with:

- action name
- authorization code
- previous effective status
- next effective status
- reason when supplied
- impact summary
- admin identity when available

New audit actions:

- `authorization_generated`
- `authorization_imported`
- `authorization_extended`
- `authorization_unbound`
- `authorization_frozen`
- `authorization_restored`
- `authorization_archived`
- `authorization_bulk_updated`

## Admin UI Design

### Navigation

Replace the current card page label and content:

- Old: `卡密管理`
- New: `授权控制面`

Do not keep the old card page as a second primary entry.

### Page Structure

The first implementation screen uses three regions:

1. Header and global search
   - Search by code, machine ID, agent, IP, or note.
   - Fast status chips: issued, in use, expiring, expired, frozen, archived.
2. Authorization list
   - Dense rows, not oversized cards.
   - Each row shows code, effective status, lifecycle stage, expires at, remaining time, bound machine, active sessions, agent, last activity, and risk flags.
3. Detail drawer
   - Opens on row click without navigating away.
   - Shows summary, lifecycle timeline, client associations, troubleshooting, audit, and safe operation panel.

### Detail Drawer

Sections:

- Summary
  - Effective status, lifecycle stage, expires at, remaining time, max sessions, origin, agent.
- Client association
  - Machine ID, client version, current sessions, recent IP, last heartbeat.
- Delivery context
  - Script version/hash, payload ID/hash, release channel/version when available.
- Lifecycle timeline
  - Created/imported, activated, extended, unbound, frozen/restored, archived, recent heartbeats.
- Troubleshooting
  - Plain conclusion, such as "expired by time", "bound to another machine", "active session limit reached", "blacklisted", or "archived".
- Actions
  - Extend, unbind, freeze, restore, archive delete.
  - Every dangerous action opens impact preview before execution.

### Bulk Operations

Bulk operations are allowed, but always preview impact first.

Bulk actions:

- Extend
- Freeze
- Restore
- Archive delete
- Export selected

Bulk preview must show counts by effective status and warnings before execution.

## Migration Strategy

### Phase 1A: Read-Only Control Projection

- Add authorization service projection over existing data.
- Add list and detail V3 API.
- Add tests for effective status and archived filtering.
- UI can render the new authorization page from read-only endpoints.

### Phase 1B: Safe Actions

- Add impact preview endpoints.
- Add extend, unbind, freeze, restore through authorization service.
- Keep existing card endpoints working.
- Add audit coverage.

### Phase 1C: Archived Deletion

- Add archive storage.
- Replace physical delete in admin UI with archive delete.
- Hide archived records by default.
- Reject archived authorizations from client activation and session validation.
- Add explicit include-archived filter for audit and recovery.

### Phase 1D: Replace Old Card Page

- Swap the admin card page content to authorization control plane.
- Keep old admin card routes as compatibility adapters.
- Remove old row-level destructive actions from the visible UI.

## Data Safety

Production data must not be modified destructively during migration.

Rules:

- Additive schema changes only.
- Backup before deployment.
- No bulk rewrite of card rows unless a tested migration exists.
- Archive delete does not remove the card row by default.
- Purge delete is out of scope for normal Phase 1 UI.

## Testing Strategy

Backend tests:

- Effective status precedence.
- Archived authorization hidden by default.
- Archived authorization included only with explicit filter.
- Expired-by-time active card shows expired.
- Detail projection includes sessions, machine, agent, and audit.
- Impact preview for freeze, unbind, extend, and archive delete.
- Archive delete blocks client activation.
- Batch impact counts are correct.

Frontend tests or verification:

- Authorization list renders effective status from API.
- Clicking a row opens detail drawer.
- Dangerous actions cannot execute without preview confirmation.
- Archived items disappear from normal list after archive delete.
- Status chips and search filters do not conflict.
- Dense layout fits desktop and mobile without overlapping controls.

Verification commands:

```powershell
cd server
go test ./...

cd server/web
npm run typecheck
npm run build
```

## Rollout Plan

1. Implement and test V3 read-only authorization endpoints locally.
2. Build the new admin authorization page against those endpoints.
3. Deploy with the old card page still available behind compatibility routes, but not visible as the main entry.
4. Verify production data is readable and statuses match expectations.
5. Enable safe actions.
6. Enable archive delete only after impact preview and audit are verified.

## Acceptance Criteria

Phase 1 is done when:

1. Admin card page is replaced by authorization control plane.
2. Effective status is consistent across list, detail, filters, export, and action previews.
3. Archived deletion is available and does not physically delete card rows by default.
4. Every dangerous action requires impact preview.
5. Existing client activation and heartbeat behavior remains compatible.
6. Existing production data remains intact.
7. `go test ./...`, `npm run typecheck`, and `npm run build` pass.
8. The deployed admin UI can answer why an authorization cannot be used without jumping through unrelated pages.
