# Java Backend Architecture

## Scope

This document defines a low-risk Java backend migration path for the current Lingqiao Go backend. The Java backend is a sidecar and compatibility target first. It must not replace `server/main.go`, `buildAdminServer`, `buildAgentServer`, or any existing production route until contract tests and rollback checks pass.

## Current Backend Audit

The current backend is a Go modular monolith under `server/`.

- Runtime: Go 1.26.5, `net/http`, embedded web assets, two HTTPS servers.
- Persistence: SQLite via `modernc.org/sqlite`, plus JSON/file stores for scripts, payloads, chat, announcements, agent token maps, and legacy migration data.
- Entrypoints: admin/client API on `PORT` default `48901`, agent API on `AGENT_PORT` default `38472`.
- Shared API layer: `server/internal/httpapi` provides JSON responses, RFC 9457 Problem Details, security headers, rate limiting, body limits, and request helpers.
- Security: client HMAC with PBKDF2-derived keys and nonce replay guard; admin and agent sessions use secure cookies or bearer tokens; admin authorization uses role permissions.
- Main database: `data/lingqiao.db` with `cards`, `sessions`, `blacklist`, `audit_log`, `agents`, `invites`, `exam_records`, `admin_users`, and `admin_sessions`.
- Release database: `data/releases.db` with `releases`, `release_packages`, and `release_events`.

## Modules That Should Not Be Replaced First

These modules should stay in Go until Java has byte-for-byte or contract-level compatibility:

- `payloads`: encrypted chunks, key exchange, DLL validation, filesystem layout, upload key handling.
- `dll-download`: AES-GCM encrypted binary response and active payload fallback logic.
- `releases`: signed manifest, rollout selection, package file path guard, separate SQLite database.
- `auth`: HMAC timestamp/nonce replay behavior and generated secret files.

These modules are better Java candidates for early sidecar work:

- `system-health`: public health endpoint and module overview DTOs.
- `exam`: simple SQLite repository and four endpoints, with clear session/auth boundaries.
- `scripts`: file-backed repository and explicit legacy compatibility endpoint.
- `audit` read path: admin list/query can be mirrored before writes.

## Target Package Structure

```text
server-java/
  pom.xml
  src/main/java/com/lingqiao/server/
    LingqiaoJavaApplication.java
    common/
      api/
      errors/
      security/
      web/
    config/
    system/
    exam/
    scripts/
    audit/
    cards/
    sessions/
    agents/
    releases/
    payloads/
    migration/
  src/main/resources/
    application.yml
    db/migration/
  src/test/java/com/lingqiao/server/
```

## Layering

- Controller: parses HTTP requests, preserves path/method/body/query compatibility, delegates to service.
- Service: owns business rules and migration state. Services must not depend on servlet APIs.
- Repository: owns SQLite or file access. Repositories must not return HTTP response DTOs.
- DTO/Mapper: preserves existing snake_case JSON names and old top-level response fields.
- Security: Spring filters mirror Go HMAC, admin token, agent token, and permission checks.
- Migration: Flyway migrations are additive and disabled unless explicitly enabled.

## Unified Response Structure

Java must support two response modes:

- Compatibility success: existing top-level `status: "ok"` plus legacy fields.
- New internal success: `ApiResponse(status, data, meta)`, only for Java-only or future `/java/*` routes.

Errors use RFC 9457 Problem Details with stable `code`. Legacy Chinese `detail` text remains compatible where clients already depend on it.

## Unified Error Codes

Java `ErrorCode` starts with existing Go/API codes:

`INVALID_JSON`, `METHOD_NOT_ALLOWED`, `HMAC_MISSING`, `HMAC_INVALID`, `BAD_REQUEST`, `UNAUTHORIZED`, `FORBIDDEN`, `NOT_FOUND`, `INTERNAL_ERROR`, `CARD_NOT_FOUND`, `CARD_BLACKLISTED`, `CARD_DISABLED`, `CARD_EXPIRED`, `CARD_BOUND_TO_OTHER_MACHINE`, `MAX_SESSIONS_REACHED`, `SESSION_NOT_FOUND`, `SESSION_EXPIRED`, `MACHINE_MISMATCH`, `SESSION_CARD_MISMATCH`, `RELEASE_SERVICE_UNAVAILABLE`, `PACKAGE_NOT_FOUND`.

## Authentication Integration

- Client API: `HmacAuthenticationFilter` verifies `X-HMAC-Signature`, `X-Client-ID`, `X-Timestamp`, `X-Nonce` over either request body or URL path exactly like Go.
- Admin API: `AdminSessionAuthenticationFilter` extracts `admin_token` cookie or bearer token, hashes token, reads `admin_sessions`, and attaches permissions.
- Agent API: `AgentSessionAuthenticationFilter` reads `agent_token`, validates token map compatibility, and attaches agent ID.
- Permissions: `@RequiresPermission("cards.read")` mirrors `admin.HasPermission`.

## Database Migration

- Use Flyway under `server-java/src/main/resources/db/migration`.
- Migrations are additive only.
- Existing Go startup schema remains authoritative until cutover.
- Java can run in read-only DB mode by default.
- JSON importers run dry-run first and write reports before any mutation.

## DTO Mapping

- Use explicit Jackson annotations for every snake_case field.
- Keep Unix seconds for client session responses where Go uses Unix timestamps.
- Keep RFC3339 strings for admin list views where Go formats time strings.
- Avoid global naming assumptions for compatibility DTOs.

## Service And Repository Design

Each migrated feature follows:

```text
Controller -> Service -> Repository -> SQLite/File
              |          ^
              v          |
            Mapper/DTO ----
```

Service methods return domain objects or result types, not servlet responses. Repositories expose narrow interfaces per module, not a shared generic DAO.

## Gradual Replacement Plan

1. Sidecar only: Java starts on a separate port with `/java/api/health`.
2. Shadow read: Java reads the same SQLite/file data and compares JSON fixtures.
3. Contract mirror: Java controller tests replay Go-compatible requests.
4. Optional proxy: selected admin-only read endpoints can be routed to Java behind a flag.
5. Controlled cutover: only one low-risk module at a time.
6. Rollback: disable the Java module flag and continue serving Go routes.

## Architecture Governance Gates

- Repository and runtime boundaries are defined in `docs/architecture/repository-boundaries.md`.
- Current Go module ownership, persistence, dependency direction, and migration wave are defined in `docs/architecture/server-module-map.md`.
- Java must not become a second writer for SQLite, JSON, payload files, scripts, generated HMAC secrets, upload keys, release signing seeds, or TLS keys before a module-specific ownership decision.
- `system-health`, read-only audit, and read-only exam projections are lower-risk traffic candidates than cards, sessions, releases, scripts, payloads, or authentication.
- Cards and sessions compatibility code may be implemented early to establish DTO and repository contracts, but production routing waits for HMAC, session, time-format, transaction, and rollback contract tests.
- HMAC, card-code generation, release manifests, payload key exchange, and encrypted DLL delivery require cross-language golden vectors before any cutover.
