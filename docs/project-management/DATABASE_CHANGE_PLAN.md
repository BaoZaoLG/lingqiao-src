# Database Change Plan

This file is the only place where Session 04 records proposed or applied
database structure changes. Runtime migrations must remain additive,
idempotent, and covered by temporary-database upgrade tests.

## DCP-0001 Invites Compatibility And List Index

Status: test-first implementation in progress

Tasks: S04-0001 through S04-0048

Current risk:

- Legacy `invites` rows may contain `NULL` in `created_by`, `used_by`,
  `max_uses`, or `use_count`.
- `InviteService.List` scans some nullable columns into non-null Go values.
- Invite consumption performs arithmetic and limits against nullable counters.
- List ordering by `created_at` has no covering order index or deterministic
  tie-breaker.
- Legacy `invites.json` migration is embedded in `main.go` and is not directly
  covered by atomicity or repeated-startup tests.

Planned compatible change:

- New databases use non-null defaults for text and counter columns while
  retaining nullable `used_at`.
- Existing rows are normalized with `COALESCE` updates; the table is not
  rebuilt and no row is deleted.
- Add `idx_invites_created_at_code` on
  `(created_at DESC, code ASC)`.
- Move legacy invite import into a tested transaction that is safe to repeat.
- Store newly generated invite timestamps in UTC.

Upgrade verification:

- Open a temporary database containing the legacy nullable schema and rows.
- Apply the schema twice and verify normalized values and index presence.
- Import a temporary legacy `invites.json` twice and verify one durable row.
- Force an insert failure and verify the entire import transaction rolls back.
- Verify repository reads, UTC time round trips, and deterministic ordering.

Rollback:

- Stop the server before rollback.
- Drop only `idx_invites_created_at_code` if the index must be removed.
- Reverting code restores the previous query and migration path.
- Normalized `NULL` values are intentionally not restored because empty text
  and zero counters are the application-level legacy defaults.
- No table rebuild is part of this change, so rollback does not require copying
  or deleting invite rows.

## Session 04 invites hardening
- NOT NULL defaults on invites.created_by/used_by/max_uses/use_count
- idx_invites_created_at_code
- COALESCE normalize on applySchema
- migrateJSONInvitesToSQLite in db_invites_migrate.go

