# Lingqiao V2 Architecture Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the V2 architecture roadmap into a controlled, testable, standards-aligned refactor path for Lingqiao without breaking the current product.

**Architecture:** Keep the project as a modular monolith first. Introduce Clean/Hexagonal boundaries in small increments: HTTP handlers call feature services, feature services depend on repository interfaces, repositories wrap JSON or SQLite storage, and UI code delegates business work to controllers/services. Release, script, and API contracts become explicit artifacts rather than implicit behavior scattered across handlers.

**Tech Stack:** Go 1.25, Qt 5/C++17, Vite/TypeScript, CMake/MSBuild, SQLite via `modernc.org/sqlite`, JSON runtime persistence, OpenAPI, RFC 9457 Problem Details, SemVer, OWASP ASVS, ADR documents.

---

## 0. How To Use This Plan

This is the executable companion to `docs/ARCHITECTURE_V2_PROPOSAL.md`.

Rules for executing it:

1. Work one task at a time.
2. Create a branch before starting each phase.
3. Prefer small commits after every green test checkpoint.
4. Do not delete an old code path until a replacement path has tests and the old path is no longer used.
5. Run the listed verification commands before each commit.
6. If a step changes runtime behavior, update `docs/PROJECT_HANDOFF_REPORT.md` or add a focused doc note.
7. Keep runtime data under `injector-server/data/` out of commits.

Recommended branch names:

```powershell
git checkout -b codex/v2-phase-0-governance
git checkout -b codex/v2-phase-1-server-safety-net
git checkout -b codex/v2-phase-2-update-unification
git checkout -b codex/v2-phase-3-server-modules
git checkout -b codex/v2-phase-4-client-controllers
git checkout -b codex/v2-phase-5-autoexam-chain
```

Baseline commands before every phase:

```powershell
git status --short
git log -1 --oneline
```

Expected:

```text
git status --short
# no output

git log -1 --oneline
# shows the latest committed baseline
```

Known benign warning:

```text
warning: unable to access 'C:\Users\Li/.config/git/ignore': Permission denied
```

This warning has appeared before and does not block Git operations.

---

## 1. Final Target Structure

The long-term target is:

```text
Lingqiao/
├─ apps/
│  ├─ client-qt/
│  ├─ hook-runtime/
│  ├─ server/
│  ├─ admin-web/
│  └─ agent-web/
├─ packages/
│  ├─ autoexam/
│  ├─ contracts/
│  └─ release-tools/
├─ docs/
│  ├─ adr/
│  ├─ api/
│  ├─ security/
│  ├─ operations/
│  ├─ PROJECT_HANDOFF_REPORT.md
│  └─ ARCHITECTURE_V2_PROPOSAL.md
├─ installer/
├─ scripts/
└─ data/
```

Do not start by moving everything into this structure. The first implementation wave should keep existing source paths and add standards, tests, and adapter seams. Top-level directory migration is intentionally last.

Current-to-target mapping:

| Current | Target | Migration Timing |
| --- | --- | --- |
| `injector-server/` | `apps/server/` | Last, after server modules are stable. |
| `injector-server/web/admin` | `apps/admin-web/` | After API contracts stabilize. |
| `injector-server/web/agent` | `apps/agent-web/` | After API contracts stabilize. |
| `src/ui` | `apps/client-qt/ui` | Last, after controller split. |
| `src/hook` | `apps/hook-runtime` | After AutoExam build chain exists. |
| `AutoExam.js` | `packages/autoexam/src/autoexam.js` | Phase 5. |

---

## 2. Standards To Lock In

Create these files first. They do not change runtime behavior, but they define how future changes must be judged.

### Task 0.1: Add ADR Governance

**Files:**
- Create: `docs/adr/0001-use-modular-monolith.md`
- Create: `docs/adr/0002-use-clean-hexagonal-boundaries.md`
- Create: `docs/adr/0003-use-release-manifest-as-only-update-protocol.md`
- Create: `docs/adr/0004-use-sqlite-as-next-storage.md`

- [ ] **Step 1: Create ADR directory**

Run:

```powershell
New-Item -ItemType Directory -Force -Path docs\adr | Out-Null
```

Expected: command exits 0.

- [ ] **Step 2: Add `0001-use-modular-monolith.md`**

Content:

```markdown
# ADR-0001: Use Modular Monolith For V2 Server Architecture

## Status
Accepted

## Context
Lingqiao currently ships as a tightly coupled Windows client, Hook DLL, Go server, web admin, agent panel, AutoExam script, and release pipeline. The server has grown beyond a tool-style codebase, but the deployment and team size do not justify microservices.

## Decision
Keep the Go backend as a modular monolith. Split by business feature under `internal/features`, keep shared infrastructure under `internal/platform`, and keep one deployable server process.

## Consequences
Benefits:

- Deployment remains simple.
- Feature boundaries become clear.
- Tests can target services without booting the full server.
- A future service split remains possible after boundaries are proven.

Costs:

- Code review must enforce boundaries.
- Shared state must be moved behind repository interfaces gradually.
- Top-level directory migration must wait until feature boundaries are stable.
```

- [ ] **Step 3: Add `0002-use-clean-hexagonal-boundaries.md`**

Content:

```markdown
# ADR-0002: Use Clean And Hexagonal Boundaries

## Status
Accepted

## Context
Current handlers, services, storage, and response formatting are mixed in root-level Go files. This makes testing and future storage/API changes difficult.

## Decision
Use `handler -> service/usecase -> repository -> storage` as the default dependency direction. HTTP, JSON, SQLite, file storage, signing, and CLI are adapters around feature services.

## Consequences
Benefits:

- Business rules can be tested without HTTP.
- Storage can migrate from JSON to SQLite behind repository interfaces.
- API contract changes can be isolated to handlers and response mappers.

Costs:

- Initial refactors add adapter code.
- Some old root-level functions will temporarily call new services until callers are migrated.
```

- [ ] **Step 4: Add `0003-use-release-manifest-as-only-update-protocol.md`**

Content:

```markdown
# ADR-0003: Use Release Manifest As The Only Update Protocol

## Status
Accepted

## Context
Lingqiao currently has legacy announcement-driven update fields, legacy `updates/index.json`, and the newer release manifest flow. Maintaining all three creates ambiguous behavior.

## Decision
Make signed release manifest the only long-term update protocol. Announcements display messages only. Legacy update data may be imported or read during migration, but new releases must use release records, packages, hashes, signatures, channels, and rollout rules.

## Consequences
Benefits:

- One update path for client and admin.
- Stronger artifact verification.
- Clear version/channel/rollout semantics.

Costs:

- Client update code must be migrated carefully.
- Admin UI must stop treating announcement fields as update controls.
- Legacy data requires an import or read-only compatibility window.
```

- [ ] **Step 5: Add `0004-use-sqlite-as-next-storage.md`**

Content:

```markdown
# ADR-0004: Use SQLite As The Next Durable Storage

## Status
Accepted

## Context
Current JSON storage is easy to deploy but fragile for concurrent writes, auditing, pagination, and reporting. The release subsystem already uses SQLite through `modernc.org/sqlite`.

## Decision
Use repository interfaces first, then migrate durable server state from JSON to SQLite module by module. Keep JSON import/read compatibility until migration is complete.

## Consequences
Benefits:

- Single-file deployment remains simple.
- Transactions improve consistency.
- Query-heavy admin screens become easier to maintain.

Costs:

- Requires migration commands and backup/restore documentation.
- Requires repository tests for JSON and SQLite implementations.
```

- [ ] **Step 6: Verify ADR files exist**

Run:

```powershell
Get-ChildItem docs\adr | Select-Object Name
```

Expected includes:

```text
0001-use-modular-monolith.md
0002-use-clean-hexagonal-boundaries.md
0003-use-release-manifest-as-only-update-protocol.md
0004-use-sqlite-as-next-storage.md
```

- [ ] **Step 7: Commit**

Run:

```powershell
git add docs\adr
git commit -m "docs: add architecture decision records"
```

Expected: commit succeeds.

### Task 0.2: Add API And Error Contract Skeleton

**Files:**
- Create: `docs/api/openapi.yaml`
- Create: `docs/api/error-codes.md`
- Create: `docs/api/versioning.md`

- [ ] **Step 1: Create API docs directory**

Run:

```powershell
New-Item -ItemType Directory -Force -Path docs\api | Out-Null
```

- [ ] **Step 2: Create `docs/api/openapi.yaml`**

Content:

```yaml
openapi: 3.1.0
info:
  title: Lingqiao API
  version: 2.0.0
  description: >
    Contract-first API surface for Lingqiao client, admin, and agent APIs.
    OpenAPI 3.1 is used first for tooling compatibility; the architecture
    target allows OpenAPI 3.2 once the local toolchain supports it.
servers:
  - url: https://127.0.0.1:48901
    description: Admin and client API
  - url: https://127.0.0.1:38472
    description: Agent API
tags:
  - name: client-session
  - name: client-update
  - name: client-chat
  - name: admin-auth
  - name: admin-cards
  - name: admin-releases
  - name: agent
paths:
  /api/v1/activate:
    post:
      tags: [client-session]
      summary: Activate a card and create a client session.
      security:
        - hmacAuth: []
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/ActivateRequest'
      responses:
        '200':
          description: Card activated.
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/ActivateResponse'
        '400':
          $ref: '#/components/responses/Problem'
        '401':
          $ref: '#/components/responses/Problem'
        '403':
          $ref: '#/components/responses/Problem'
        '429':
          $ref: '#/components/responses/Problem'
  /api/v1/heartbeat:
    post:
      tags: [client-session]
      summary: Extend a client session.
      security:
        - hmacAuth: []
      responses:
        '200':
          description: Session extended.
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/HeartbeatResponse'
        '401':
          $ref: '#/components/responses/Problem'
  /api/v1/update/check:
    post:
      tags: [client-update]
      summary: Check for a signed release manifest.
      security:
        - hmacAuth: []
      responses:
        '200':
          description: Update availability result.
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/UpdateCheckResponse'
        '401':
          $ref: '#/components/responses/Problem'
components:
  securitySchemes:
    hmacAuth:
      type: apiKey
      in: header
      name: X-HMAC-Signature
  responses:
    Problem:
      description: RFC 9457 problem detail.
      content:
        application/problem+json:
          schema:
            $ref: '#/components/schemas/Problem'
  schemas:
    Problem:
      type: object
      required: [type, title, status]
      properties:
        type:
          type: string
          format: uri
        title:
          type: string
        status:
          type: integer
        detail:
          type: string
        instance:
          type: string
        code:
          type: string
          description: Stable Lingqiao business error code.
        request_id:
          type: string
    ActivateRequest:
      type: object
      required: [card, machine_id, fingerprint, client_version]
      properties:
        card:
          type: string
        machine_id:
          type: string
        fingerprint:
          type: string
        client_version:
          type: string
    ActivateResponse:
      type: object
      required: [status, session_token, expires_at, card_expires_at]
      properties:
        status:
          type: string
          enum: [ok]
        session_token:
          type: string
        expires_at:
          type: integer
          format: int64
        card_expires_at:
          type: integer
          format: int64
    HeartbeatResponse:
      type: object
      required: [status, card_expires_at]
      properties:
        status:
          type: string
          enum: [ok]
        card_expires_at:
          type: integer
          format: int64
    UpdateCheckResponse:
      type: object
      required: [status, update_available]
      properties:
        status:
          type: string
          enum: [ok]
        update_available:
          type: boolean
        manifest_payload:
          type: string
        manifest_hmac:
          type: string
        signature:
          type: string
        public_key:
          type: string
```

- [ ] **Step 3: Create `docs/api/error-codes.md`**

Content:

```markdown
# Lingqiao API Error Codes

All new API errors should use RFC 9457 Problem Details with `Content-Type: application/problem+json`.

Required fields:

- `type`: stable documentation URI for the problem type.
- `title`: short English title for logs and tooling.
- `status`: HTTP status code.
- `detail`: localized human-readable message when safe to expose.
- `instance`: request path or request-specific URI.
- `code`: stable Lingqiao business error code.
- `request_id`: correlation ID when available.

Initial error codes:

| Code | HTTP | Meaning |
| --- | --- | --- |
| `INVALID_JSON` | 400 | Request body is not valid JSON. |
| `METHOD_NOT_ALLOWED` | 405 | Endpoint does not allow the HTTP method. |
| `HMAC_MISSING` | 401 | Required HMAC headers are missing. |
| `HMAC_INVALID` | 401 | HMAC verification failed. |
| `CARD_NOT_FOUND` | 404 | Card code does not exist. |
| `CARD_BLACKLISTED` | 403 | Card is blacklisted. |
| `CARD_DISABLED` | 403 | Card is disabled. |
| `CARD_EXPIRED` | 403 | Card expired. |
| `CARD_BOUND_TO_OTHER_MACHINE` | 403 | Card is already bound to another machine. |
| `MAX_SESSIONS_REACHED` | 403 | Card reached active session limit. |
| `SESSION_NOT_FOUND` | 401 | Session token does not exist. |
| `SESSION_EXPIRED` | 401 | Session expired. |
| `MACHINE_MISMATCH` | 403 | Session machine does not match request machine. |
| `RELEASE_SERVICE_UNAVAILABLE` | 503 | Release subsystem is unavailable. |
| `PACKAGE_NOT_FOUND` | 404 | Release package metadata or file is missing. |
```

- [ ] **Step 4: Create `docs/api/versioning.md`**

Content:

```markdown
# Lingqiao API Versioning

## Version Families

- Client API: `/api/v1/*`
- Admin API: `/admin/api/*`
- Agent API: `/api/*` on the agent server

## Compatibility Policy

1. Existing `/api/v1/*` fields remain backward compatible during V2 migration.
2. New response fields may be added.
3. Existing fields must not change type.
4. Breaking client API changes require `/api/v2/*`.
5. New update protocol work must stay behind `/api/v1/update/check` until all current clients support the signed manifest response.

## Semantic Versioning

Lingqiao release artifacts use `MAJOR.MINOR.PATCH`.

- `MAJOR`: breaking client/server protocol changes or incompatible data migration.
- `MINOR`: new backward-compatible features.
- `PATCH`: bug fixes, UI fixes, security patches without protocol breakage.
```

- [ ] **Step 5: Verify**

Run:

```powershell
Get-ChildItem docs\api | Select-Object Name
```

Expected includes:

```text
error-codes.md
openapi.yaml
versioning.md
```

- [ ] **Step 6: Commit**

Run:

```powershell
git add docs\api
git commit -m "docs: add initial API contracts"
```

### Task 0.3: Add Security And Operations Skeleton

**Files:**
- Create: `docs/security/asvs-checklist.md`
- Create: `docs/security/secrets-management.md`
- Create: `docs/security/release-signing.md`
- Create: `docs/operations/config.md`
- Create: `docs/operations/backup-restore.md`
- Create: `docs/operations/release-runbook.md`

- [ ] **Step 1: Create directories**

Run:

```powershell
New-Item -ItemType Directory -Force -Path docs\security,docs\operations | Out-Null
```

- [ ] **Step 2: Create `docs/security/asvs-checklist.md`**

Content:

```markdown
# Lingqiao OWASP ASVS Checklist

Target level: ASVS L1 now, L2 for admin/release/security-sensitive flows.

## Authentication

- [ ] Admin password stored with bcrypt.
- [ ] Legacy SHA-256 admin password migration remains one-way.
- [ ] Agent password stored with bcrypt.
- [ ] Session tokens are random and not stored in plaintext where avoidable.

## API Protection

- [ ] Client API requires HMAC headers.
- [ ] HMAC timestamp and nonce are checked.
- [ ] Rate limiting protects activation and login.
- [ ] Problem responses do not leak secrets.

## Uploads And Releases

- [ ] Package upload validates file type, size, hash, and safe path.
- [ ] Release manifest is signed.
- [ ] Client verifies package SHA256.
- [ ] Admin actions write audit entries.

## Web Security

- [ ] CSP is set for admin and agent panels.
- [ ] CORS is same-origin by default.
- [ ] Cookies use secure settings.
- [ ] Sensitive logs are masked.
```

- [ ] **Step 3: Create `docs/security/secrets-management.md`**

Content:

```markdown
# Lingqiao Secrets Management

Never commit runtime secrets.

Ignored local runtime files include:

- `injector-server/data/admin_password.hash`
- `injector-server/data/hmac_secret.key`
- `injector-server/certs/`
- generated client secret headers

Transfer rule:

1. Transfer source code through Git.
2. Transfer production runtime data separately through encrypted backup.
3. Rotate `ADMIN_PASSWORD`, `HMAC_SECRET`, TLS certificates, release signing key, and client embedded configuration when ownership changes.
4. Rebuild the client after rotating embedded client credentials.
```

- [ ] **Step 4: Create `docs/security/release-signing.md`**

Content:

```markdown
# Lingqiao Release Signing

Every production release package must have:

- semantic version
- channel
- package kind
- file size
- SHA256
- signed manifest
- release notes
- rollout percentage

Client acceptance rules:

1. Manifest signature must verify.
2. Manifest HMAC must verify for the active client session when used.
3. Package SHA256 must match the manifest.
4. Package kind must be supported by the client.
5. Force update must lock risky client actions until completed.
```

- [ ] **Step 5: Create `docs/operations/config.md`**

Content:

```markdown
# Lingqiao Configuration

Server configuration should come from environment variables.

| Variable | Purpose |
| --- | --- |
| `PORT` | Admin and client API port. |
| `AGENT_PORT` | Agent panel port. |
| `DATA_DIR` | Runtime JSON/SQLite data directory. |
| `SESSION_TTL` | Client/admin/agent session lifetime. |
| `ADMIN_PASSWORD` | Initial admin password. |
| `HMAC_SECRET` | Client HMAC root secret. |
| `ADMIN_ORIGIN` | Optional admin CORS origin. |
| `AGENT_ORIGIN` | Optional agent CORS origin. |
| `CERT_HOST` | Self-signed certificate host. |

Do not hardcode production secrets in source files.
```

- [ ] **Step 6: Create `docs/operations/backup-restore.md`**

Content:

```markdown
# Lingqiao Backup And Restore

## Backup

Stop the server or put it in maintenance mode before backup.

Back up:

- `injector-server/data/`
- release package files under `DATA_DIR/updates/`
- release database if present
- TLS certificates if reused
- release signing keys if stored outside `DATA_DIR`

## Restore

1. Stop the server.
2. Restore `DATA_DIR`.
3. Verify file permissions.
4. Start the server.
5. Open admin panel and verify cards, releases, agents, and audit entries.
6. Run a test client activation and heartbeat.
```

- [ ] **Step 7: Create `docs/operations/release-runbook.md`**

Content:

````markdown
# Lingqiao Release Runbook

## Preflight

Run from repository root:

```powershell
cmake --build build --config Release --target Injector
cmake --build build --config Release --target CefHook
```

Run from `injector-server`:

```powershell
go test ./...
```

Run from `injector-server/web`:

```powershell
npm run check
```

## Release

1. Build web assets.
2. Build server.
3. Build client and Hook DLL.
4. Build installer.
5. Compute SHA256.
6. Create release record.
7. Upload package.
8. Publish to a limited rollout.
9. Check release events.
10. Expand rollout.
````

- [ ] **Step 8: Commit**

Run:

```powershell
git add docs\security docs\operations
git commit -m "docs: add security and operations standards"
```

---

## 3. Phase 1: Server Safety Net

This phase makes current behavior testable before major refactors.

### Task 1.1: Add CardManager Lifecycle Tests

**Files:**
- Create: `injector-server/card_manager_test.go`
- Modify only if needed: `injector-server/card_ops.go`

- [ ] **Step 1: Create `injector-server/card_manager_test.go`**

Content:

```go
package main

import (
	"errors"
	"testing"
	"time"

	cardops "github.com/lingqiao/server/internal/cards"
)

func newTestCardManager(t *testing.T) *CardManager {
	t.Helper()
	return NewCardManager(NewJSONStorage(t.TempDir()))
}

func TestCardManagerGenerateActivateHeartbeatDeactivate(t *testing.T) {
	cm := newTestCardManager(t)

	card, err := cm.GenerateCard(24*time.Hour, "test card", 1, "")
	if err != nil {
		t.Fatalf("GenerateCard returned error: %v", err)
	}
	if card.Code == "" {
		t.Fatal("GenerateCard returned empty code")
	}

	session, err := cm.ActivateCard(card.Code, "machine-1", "fingerprint-1", "127.0.0.1", "2.4.0")
	if err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}
	if session.Token == "" {
		t.Fatal("ActivateCard returned empty token")
	}
	if session.MachineID != "machine-1" {
		t.Fatalf("MachineID = %q, want machine-1", session.MachineID)
	}

	heartbeat, err := cm.Heartbeat(session.Token, "machine-1", "127.0.0.1", "2.4.1")
	if err != nil {
		t.Fatalf("Heartbeat returned error: %v", err)
	}
	if heartbeat.ClientVersion != "2.4.1" {
		t.Fatalf("ClientVersion = %q, want 2.4.1", heartbeat.ClientVersion)
	}

	if err := cm.DeactivateSession(session.Token, false); err != nil {
		t.Fatalf("DeactivateSession returned error: %v", err)
	}
	if _, err := cm.Heartbeat(session.Token, "machine-1", "127.0.0.1", "2.4.1"); err == nil {
		t.Fatal("Heartbeat after deactivation returned nil error")
	}
}

func TestCardManagerRejectsDisabledExpiredAndBlacklistedCards(t *testing.T) {
	cm := newTestCardManager(t)

	card, err := cm.GenerateCard(24*time.Hour, "test card", 1, "")
	if err != nil {
		t.Fatalf("GenerateCard returned error: %v", err)
	}

	if err := cm.UpdateCardStatus(card.Code, CardDisabled); err != nil {
		t.Fatalf("UpdateCardStatus disabled returned error: %v", err)
	}
	if _, err := cm.ValidateCard(card.Code); !errors.Is(err, cardops.ErrCardDisabled) {
		t.Fatalf("ValidateCard disabled error = %v, want ErrCardDisabled", err)
	}

	if err := cm.UpdateCardStatus(card.Code, CardActive); err != nil {
		t.Fatalf("UpdateCardStatus active returned error: %v", err)
	}
	cm.AddBlacklist(normalizeCardCode(card.Code), normalizeCardCode(card.Code), "test blacklist")
	if _, err := cm.ValidateCard(card.Code); !errors.Is(err, cardops.ErrCardBlacklisted) {
		t.Fatalf("ValidateCard blacklisted error = %v, want ErrCardBlacklisted", err)
	}
}

func TestCardManagerEnforcesMaxSessions(t *testing.T) {
	cm := newTestCardManager(t)

	card, err := cm.GenerateCard(24*time.Hour, "single session", 1, "")
	if err != nil {
		t.Fatalf("GenerateCard returned error: %v", err)
	}
	if _, err := cm.ActivateCard(card.Code, "machine-1", "fingerprint-1", "127.0.0.1", "2.4.0"); err != nil {
		t.Fatalf("first ActivateCard returned error: %v", err)
	}
	if _, err := cm.ActivateCard(card.Code, "machine-1", "fingerprint-1", "127.0.0.1", "2.4.0"); !errors.Is(err, cardops.ErrMaxSessionsReached) {
		t.Fatalf("second ActivateCard error = %v, want ErrMaxSessionsReached", err)
	}
}
```

- [ ] **Step 2: Run the test**

Run from `injector-server`:

```powershell
go test ./... -run "TestCardManager" -count=1
```

Expected: tests compile and pass. If a test fails because the current behavior differs, do not change behavior immediately. First record the observed behavior in the test name or expected assertion, then decide whether the difference is a bug.

- [ ] **Step 3: Run all server tests**

Run from `injector-server`:

```powershell
go test ./...
```

Expected: all packages pass.

- [ ] **Step 4: Commit**

Run:

```powershell
git add injector-server\card_manager_test.go
git commit -m "test: cover card manager lifecycle"
```

### Task 1.2: Add Release Selector Tests

**Files:**
- Create: `injector-server/internal/releases/selector_test.go`

- [ ] **Step 1: Create `selector_test.go`**

Content:

```go
package releases

import (
	"context"
	"testing"
	"time"
)

func newSelectorTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	store, err := OpenSQLiteStore(t.TempDir() + "/releases.db")
	if err != nil {
		t.Fatalf("OpenSQLiteStore returned error: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func saveReleaseWithPackage(t *testing.T, store *SQLiteStore, release Release, pkg ReleasePackage) {
	t.Helper()
	ctx := context.Background()
	if err := store.SaveRelease(ctx, release); err != nil {
		t.Fatalf("SaveRelease returned error: %v", err)
	}
	if err := store.SavePackage(ctx, pkg); err != nil {
		t.Fatalf("SavePackage returned error: %v", err)
	}
}

func TestSelectorSelectsNewerPublishedRelease(t *testing.T) {
	store := newSelectorTestStore(t)
	now := time.Now().UTC()
	publishedAt := now
	saveReleaseWithPackage(t, store, Release{
		ID:             "rel-2-5-0",
		Version:        "2.5.0",
		Channel:        ChannelStable,
		Status:         StatusPublished,
		RolloutPercent: 100,
		CreatedAt:      now,
		PublishedAt:    &publishedAt,
	}, ReleasePackage{
		ID:        "pkg-2-5-0",
		ReleaseID: "rel-2-5-0",
		Kind:      PackageKindBundle,
		Filename:  "LingqiaoSetup-2.5.0.exe",
		Path:      "updates/LingqiaoSetup-2.5.0.exe",
		FileSize:  123,
		SHA256:    "abc",
		CreatedAt: now,
	})

	selected, err := NewSelector(store).Select(context.Background(), ClientContext{
		Version:   "2.4.0",
		Channel:   ChannelStable,
		MachineID: "machine-1",
	})
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}
	if selected == nil {
		t.Fatal("Select returned nil, want release")
	}
	if selected.Release.Version != "2.5.0" {
		t.Fatalf("selected version = %q, want 2.5.0", selected.Release.Version)
	}
	if selected.Package.ID != "pkg-2-5-0" {
		t.Fatalf("selected package = %q, want pkg-2-5-0", selected.Package.ID)
	}
}

func TestSelectorSkipsCurrentOrOlderVersion(t *testing.T) {
	store := newSelectorTestStore(t)
	now := time.Now().UTC()
	publishedAt := now
	saveReleaseWithPackage(t, store, Release{
		ID:             "rel-2-4-0",
		Version:        "2.4.0",
		Channel:        ChannelStable,
		Status:         StatusPublished,
		RolloutPercent: 100,
		CreatedAt:      now,
		PublishedAt:    &publishedAt,
	}, ReleasePackage{
		ID:        "pkg-2-4-0",
		ReleaseID: "rel-2-4-0",
		Kind:      PackageKindBundle,
		Filename:  "LingqiaoSetup-2.4.0.exe",
		Path:      "updates/LingqiaoSetup-2.4.0.exe",
		FileSize:  123,
		SHA256:    "abc",
		CreatedAt: now,
	})

	selected, err := NewSelector(store).Select(context.Background(), ClientContext{
		Version:   "2.4.0",
		Channel:   ChannelStable,
		MachineID: "machine-1",
	})
	if err != nil {
		t.Fatalf("Select returned error: %v", err)
	}
	if selected != nil {
		t.Fatalf("Select returned %+v, want nil", selected)
	}
}

func TestSelectorHonorsDenyAndAllowTargeting(t *testing.T) {
	store := newSelectorTestStore(t)
	now := time.Now().UTC()
	publishedAt := now
	saveReleaseWithPackage(t, store, Release{
		ID:             "rel-targeted",
		Version:        "2.5.0",
		Channel:        ChannelStable,
		Status:         StatusPublished,
		RolloutPercent: 100,
		Targeting: TargetingRules{
			AllowMachines: []string{"machine-allowed"},
			DenyMachines:  []string{"machine-denied"},
		},
		CreatedAt:   now,
		PublishedAt: &publishedAt,
	}, ReleasePackage{
		ID:        "pkg-targeted",
		ReleaseID: "rel-targeted",
		Kind:      PackageKindBundle,
		Filename:  "LingqiaoSetup-2.5.0.exe",
		Path:      "updates/LingqiaoSetup-2.5.0.exe",
		FileSize:  123,
		SHA256:    "abc",
		CreatedAt: now,
	})

	denied, err := NewSelector(store).Select(context.Background(), ClientContext{
		Version:   "2.4.0",
		Channel:   ChannelStable,
		MachineID: "machine-denied",
	})
	if err != nil {
		t.Fatalf("Select denied returned error: %v", err)
	}
	if denied != nil {
		t.Fatalf("denied selection = %+v, want nil", denied)
	}

	allowed, err := NewSelector(store).Select(context.Background(), ClientContext{
		Version:   "2.4.0",
		Channel:   ChannelStable,
		MachineID: "machine-allowed",
	})
	if err != nil {
		t.Fatalf("Select allowed returned error: %v", err)
	}
	if allowed == nil {
		t.Fatal("allowed selection nil, want release")
	}
}
```

- [ ] **Step 2: Run release selector tests**

Run from `injector-server`:

```powershell
go test ./internal/releases -run "TestSelector" -count=1
```

Expected: pass.

- [ ] **Step 3: Run all server tests**

Run from `injector-server`:

```powershell
go test ./...
```

Expected: pass.

- [ ] **Step 4: Commit**

Run:

```powershell
git add injector-server\internal\releases\selector_test.go
git commit -m "test: cover release selector behavior"
```

### Task 1.3: Introduce RFC 9457 Problem Helpers Without Switching All Routes

**Files:**
- Modify: `injector-server/httputil.go`
- Create: `injector-server/httputil_test.go`

The first step adds helpers but does not convert every handler. This keeps behavior risk low.

- [ ] **Step 1: Add problem types to `httputil.go`**

Add below `type errorResponse struct`:

```go
type problemResponse struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	Status    int    `json:"status"`
	Detail    string `json:"detail,omitempty"`
	Instance  string `json:"instance,omitempty"`
	Code      string `json:"code,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, title, detail string) {
	if title == "" {
		title = http.StatusText(status)
	}
	problemType := "https://lingqiao.local/problems/" + strings.ToLower(strings.ReplaceAll(code, "_", "-"))
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problemResponse{
		Type:     problemType,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: r.URL.Path,
		Code:     code,
	})
}
```

`httputil.go` already imports `encoding/json`, `net/http`, and `strings`, so no new imports should be required.

- [ ] **Step 2: Create `injector-server/httputil_test.go`**

Content:

```go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteProblemUsesRFC9457Shape(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/activate", nil)
	rec := httptest.NewRecorder()

	writeProblem(rec, req, http.StatusForbidden, "CARD_EXPIRED", "Card expired", "卡密已过期")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	var body problemResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if body.Type != "https://lingqiao.local/problems/card-expired" {
		t.Fatalf("Type = %q", body.Type)
	}
	if body.Title != "Card expired" {
		t.Fatalf("Title = %q", body.Title)
	}
	if body.Status != http.StatusForbidden {
		t.Fatalf("Status = %d", body.Status)
	}
	if body.Detail != "卡密已过期" {
		t.Fatalf("Detail = %q", body.Detail)
	}
	if body.Instance != "/api/v1/activate" {
		t.Fatalf("Instance = %q", body.Instance)
	}
	if body.Code != "CARD_EXPIRED" {
		t.Fatalf("Code = %q", body.Code)
	}
}
```

- [ ] **Step 3: Run helper test**

Run from `injector-server`:

```powershell
go test ./... -run "TestWriteProblem" -count=1
```

Expected: pass.

- [ ] **Step 4: Run all server tests**

Run from `injector-server`:

```powershell
go test ./...
```

Expected: pass.

- [ ] **Step 5: Commit**

Run:

```powershell
git add injector-server\httputil.go injector-server\httputil_test.go
git commit -m "feat: add problem detail response helper"
```

---

## 4. Phase 2: Update Protocol Unification

Goal: make signed release manifest the only long-term update path.

Do this after Phase 1 tests are green.

### Task 2.1: Document Current Update Routes And Compatibility Window

**Files:**
- Create: `docs/operations/update-protocol-migration.md`

- [ ] **Step 1: Create migration document**

Content:

```markdown
# Update Protocol Migration

## Current Paths

1. Legacy announcement update fields:
   - `latest_version`
   - `min_version`
   - `force_update`
   - `download_url`
   - `sha256`

2. Legacy update package index:
   - `DATA_DIR/updates/index.json`
   - `/admin/api/update/*`

3. V2 release manifest:
   - `/api/v1/update/check`
   - `/api/v1/update/package/{id}`
   - `/admin/api/releases/*`

## V2 Decision

Signed release manifest is the only long-term update protocol. Announcement content must not carry update protocol fields after migration.

## Compatibility Window

During migration:

- Existing legacy update data may be imported into releases.
- Legacy admin update routes remain read-only or hidden.
- Client keeps fallback only until all supported client versions understand signed manifests.

## Removal Criteria

Legacy update fields can be removed when:

1. The minimum supported client version uses `/api/v1/update/check`.
2. Admin UI release page fully supports upload, publish, pause, rollback, and rollout.
3. Release metrics are visible enough to diagnose failed upgrades.
4. A production backup exists before migration.
```

- [ ] **Step 2: Commit**

Run:

```powershell
git add docs\operations\update-protocol-migration.md
git commit -m "docs: define update protocol migration"
```

### Task 2.2: Add Release Manifest Handler Tests

**Files:**
- Create: `injector-server/release_handlers_test.go`

This test should start with service-level helper tests rather than full HMAC HTTP calls. Full HTTP tests can follow once auth helpers are easier to inject.

- [ ] **Step 1: Create manifest mapping test**

Content:

```go
package main

import (
	"testing"
	"time"

	releasesvc "github.com/lingqiao/server/internal/releases"
)

func TestManifestFromSelectionMapsReleaseAndPackage(t *testing.T) {
	now := time.Now().UTC()
	selected := releasesvc.SelectedRelease{
		Release: releasesvc.Release{
			ID:             "rel-2-5-0",
			Version:        "2.5.0",
			Channel:        releasesvc.ChannelStable,
			MinVersion:     "2.4.0",
			ForceUpdate:    true,
			Notes:          "Important update",
			RolloutPercent: 25,
			CreatedAt:      now,
		},
		Package: releasesvc.ReleasePackage{
			ID:       "pkg-2-5-0",
			Kind:     releasesvc.PackageKindBundle,
			FileSize: 12345,
			SHA256:   "abc123",
		},
	}

	manifest := manifestFromSelection(selected)

	if manifest.ReleaseID != "rel-2-5-0" {
		t.Fatalf("ReleaseID = %q", manifest.ReleaseID)
	}
	if manifest.Version != "2.5.0" {
		t.Fatalf("Version = %q", manifest.Version)
	}
	if manifest.MinVersion != "2.4.0" {
		t.Fatalf("MinVersion = %q", manifest.MinVersion)
	}
	if !manifest.ForceUpdate {
		t.Fatal("ForceUpdate = false, want true")
	}
	if manifest.PackageURL != "/api/v1/update/package/pkg-2-5-0" {
		t.Fatalf("PackageURL = %q", manifest.PackageURL)
	}
	if manifest.PackageSHA256 != "abc123" {
		t.Fatalf("PackageSHA256 = %q", manifest.PackageSHA256)
	}
	if manifest.RolloutPercent != 25 {
		t.Fatalf("RolloutPercent = %d, want 25", manifest.RolloutPercent)
	}
}
```

- [ ] **Step 2: Run release handler tests**

Run from `injector-server`:

```powershell
go test ./... -run "TestManifestFromSelection" -count=1
```

Expected: pass.

- [ ] **Step 3: Commit**

Run:

```powershell
git add injector-server\release_handlers_test.go
git commit -m "test: cover release manifest mapping"
```

### Task 2.3: Convert One Update Error To Problem Details

**Files:**
- Modify: `injector-server/release_handlers.go`
- Modify or create tests: `injector-server/release_handlers_test.go`

Start with one low-risk error branch. Do not convert every route at once.

- [ ] **Step 1: Convert malformed JSON error in `HandleUpdateCheck`**

Change:

```go
if err := json.Unmarshal(body, &req); err != nil {
	writeError(w, http.StatusBadRequest, "invalid JSON")
	return
}
```

To:

```go
if err := json.Unmarshal(body, &req); err != nil {
	writeProblem(w, r, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON", "请求体不是有效 JSON")
	return
}
```

- [ ] **Step 2: Add a handler test only if HMAC is injectable**

If HMAC remains hard to construct, skip the full handler test and leave this route conversion until an auth test helper exists. Do not fake auth by weakening production code.

- [ ] **Step 3: Run all server tests**

Run from `injector-server`:

```powershell
go test ./...
```

Expected: pass.

- [ ] **Step 4: Commit**

Run:

```powershell
git add injector-server\release_handlers.go injector-server\release_handlers_test.go
git commit -m "feat: return problem details for invalid update JSON"
```

---

## 5. Phase 3: Server Modularization

Goal: move from root-level `CardManager` as the center object toward feature services with repositories.

Do this in multiple PRs/commits. Do not move all root files at once.

### Task 3.1: Create Feature Package Skeletons

**Files:**
- Create directories:
  - `injector-server/internal/features/cards`
  - `injector-server/internal/features/sessions`
  - `injector-server/internal/features/agents`
  - `injector-server/internal/features/chat`
  - `injector-server/internal/features/releases`
  - `injector-server/internal/features/scripts`
  - `injector-server/internal/features/payloads`

- [ ] **Step 1: Create directories**

Run:

```powershell
New-Item -ItemType Directory -Force -Path `
  injector-server\internal\features\cards,`
  injector-server\internal\features\sessions,`
  injector-server\internal\features\agents,`
  injector-server\internal\features\chat,`
  injector-server\internal\features\releases,`
  injector-server\internal\features\scripts,`
  injector-server\internal\features\payloads | Out-Null
```

- [ ] **Step 2: Add `doc.go` files**

For `injector-server/internal/features/cards/doc.go`:

```go
// Package cards owns card creation, status changes, expiration, search, import,
// export, and card-related admin operations.
package cards
```

For `injector-server/internal/features/sessions/doc.go`:

```go
// Package sessions owns client activation sessions, heartbeats, forced logout,
// expiration, and machine binding session rules.
package sessions
```

For `injector-server/internal/features/agents/doc.go`:

```go
// Package agents owns agent accounts, invite codes, agent card generation, and
// agent dashboard data.
package agents
```

For `injector-server/internal/features/chat/doc.go`:

```go
// Package chat owns public chat messages, profiles, reactions, moderation, and
// presence.
package chat
```

For `injector-server/internal/features/releases/doc.go`:

```go
// Package releases owns admin release workflows and client update manifest
// orchestration.
package releases
```

For `injector-server/internal/features/scripts/doc.go`:

```go
// Package scripts owns versioned script modules and active AutoExam script
// selection.
package scripts
```

For `injector-server/internal/features/payloads/doc.go`:

```go
// Package payloads owns DLL/payload metadata, upload, activation, key exchange,
// and chunked payload delivery.
package payloads
```

- [ ] **Step 3: Run Go tests**

Run from `injector-server`:

```powershell
go test ./...
```

Expected: pass.

- [ ] **Step 4: Commit**

Run:

```powershell
git add injector-server\internal\features
git commit -m "chore: add server feature package skeletons"
```

### Task 3.2: Define Card Repository Interface

**Files:**
- Create: `injector-server/internal/features/cards/repository.go`

- [ ] **Step 1: Add repository interface**

Content:

```go
package cards

import (
	"context"
	"time"
)

type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
	StatusExpired  Status = "expired"
)

type Card struct {
	Code        string
	MachineID   string
	AgentID     string
	CreatedAt   time.Time
	ActivatedAt *time.Time
	ExpiresAt   time.Time
	Status      Status
	Note        string
	MaxSessions int
}

type Repository interface {
	Get(ctx context.Context, code string) (Card, error)
	Save(ctx context.Context, card Card) error
	List(ctx context.Context, filter ListFilter) ([]Card, error)
}

type ListFilter struct {
	Query     string
	Status    Status
	MachineID string
	AgentID   string
}
```

- [ ] **Step 2: Run Go tests**

Run from `injector-server`:

```powershell
go test ./...
```

Expected: pass.

- [ ] **Step 3: Commit**

Run:

```powershell
git add injector-server\internal\features\cards\repository.go
git commit -m "feat: define card repository contract"
```

### Task 3.3: Add Card Service Wrapper Without Replacing CardManager

**Files:**
- Create: `injector-server/internal/features/cards/service.go`
- Create: `injector-server/internal/features/cards/service_test.go`

- [ ] **Step 1: Add service**

Content:

```go
package cards

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidDuration = errors.New("duration must be positive")
	ErrInvalidSessions = errors.New("max sessions must be at least 1")
)

type CodeGenerator func() (string, error)
type Clock func() time.Time

type Service struct {
	repo Repository
	generateCode CodeGenerator
	now Clock
}

func NewService(repo Repository, generateCode CodeGenerator, now Clock) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, generateCode: generateCode, now: now}
}

func (s *Service) Create(ctx context.Context, duration time.Duration, note string, maxSessions int, agentID string) (Card, error) {
	if duration <= 0 {
		return Card{}, ErrInvalidDuration
	}
	if maxSessions < 1 {
		return Card{}, ErrInvalidSessions
	}
	code, err := s.generateCode()
	if err != nil {
		return Card{}, err
	}
	now := s.now()
	card := Card{
		Code:        code,
		AgentID:     agentID,
		CreatedAt:   now,
		ExpiresAt:   now.Add(duration),
		Status:      StatusActive,
		Note:        note,
		MaxSessions: maxSessions,
	}
	if err := s.repo.Save(ctx, card); err != nil {
		return Card{}, err
	}
	return card, nil
}
```

- [ ] **Step 2: Add service tests**

Content for `service_test.go`:

```go
package cards

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryRepo struct {
	cards map[string]Card
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{cards: map[string]Card{}}
}

func (r *memoryRepo) Get(ctx context.Context, code string) (Card, error) {
	card, ok := r.cards[code]
	if !ok {
		return Card{}, errors.New("not found")
	}
	return card, nil
}

func (r *memoryRepo) Save(ctx context.Context, card Card) error {
	r.cards[card.Code] = card
	return nil
}

func (r *memoryRepo) List(ctx context.Context, filter ListFilter) ([]Card, error) {
	result := make([]Card, 0, len(r.cards))
	for _, card := range r.cards {
		result = append(result, card)
	}
	return result, nil
}

func TestServiceCreateCard(t *testing.T) {
	now := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	repo := newMemoryRepo()
	service := NewService(repo, func() (string, error) {
		return "ABCD-EFGH-JKLM", nil
	}, func() time.Time {
		return now
	})

	card, err := service.Create(context.Background(), 24*time.Hour, "note", 2, "agent-1")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if card.Code != "ABCD-EFGH-JKLM" {
		t.Fatalf("Code = %q", card.Code)
	}
	if card.AgentID != "agent-1" {
		t.Fatalf("AgentID = %q", card.AgentID)
	}
	if card.Status != StatusActive {
		t.Fatalf("Status = %q", card.Status)
	}
	if !card.ExpiresAt.Equal(now.Add(24 * time.Hour)) {
		t.Fatalf("ExpiresAt = %s", card.ExpiresAt)
	}
	if _, ok := repo.cards[card.Code]; !ok {
		t.Fatal("card was not saved")
	}
}

func TestServiceCreateRejectsInvalidInputs(t *testing.T) {
	service := NewService(newMemoryRepo(), func() (string, error) {
		return "ABCD-EFGH-JKLM", nil
	}, time.Now)

	if _, err := service.Create(context.Background(), 0, "", 1, ""); !errors.Is(err, ErrInvalidDuration) {
		t.Fatalf("duration error = %v", err)
	}
	if _, err := service.Create(context.Background(), time.Hour, "", 0, ""); !errors.Is(err, ErrInvalidSessions) {
		t.Fatalf("session error = %v", err)
	}
}
```

- [ ] **Step 3: Run tests**

Run from `injector-server`:

```powershell
go test ./internal/features/cards -count=1
go test ./...
```

Expected: pass.

- [ ] **Step 4: Commit**

Run:

```powershell
git add injector-server\internal\features\cards
git commit -m "feat: add card feature service"
```

---

## 6. Phase 4: Client Controller Split

Goal: reduce `MainWindow` responsibility without rewriting UI.

Do this after server update flow stabilizes.

### Task 4.1: Create Client API Service Boundary

**Files:**
- Create: `src/services/api_client.h`
- Create: `src/services/api_client.cpp`
- Modify: `src/CMakeLists.txt`

Initial implementation can wrap existing `network/http_client.h` functions. Do not move all network calls at once.

- [ ] **Step 1: Add service directory**

Run:

```powershell
New-Item -ItemType Directory -Force -Path src\services | Out-Null
```

- [ ] **Step 2: Create `src/services/api_client.h`**

Content:

```cpp
#pragma once

#include <QString>
#include <QByteArray>
#include <QJsonObject>

struct ApiResult {
    int statusCode = 0;
    QByteArray body;
    QString error;

    bool ok() const { return statusCode >= 200 && statusCode < 300 && error.isEmpty(); }
};

class ApiClient {
public:
    ApiClient(const wchar_t* host, int port);

    ApiResult postJson(const wchar_t* path, const QJsonObject& body) const;
    ApiResult getSigned(const wchar_t* path,
                        const QString& sessionToken,
                        const QString& machineId,
                        const QString& cardCode) const;

private:
    const wchar_t* m_host;
    int m_port;
};
```

- [ ] **Step 3: Create `src/services/api_client.cpp`**

Content:

```cpp
#include "api_client.h"

#include "network/http_client.h"
#include <QJsonDocument>

ApiClient::ApiClient(const wchar_t* host, int port)
    : m_host(host), m_port(port) {}

ApiResult ApiClient::postJson(const wchar_t* path, const QJsonObject& body) const {
    HttpResponse resp = HttpPostJson(m_host, m_port, path, QJsonDocument(body).toJson(QJsonDocument::Compact));
    return ApiResult{resp.statusCode, resp.body, resp.error};
}

ApiResult ApiClient::getSigned(const wchar_t* path,
                               const QString& sessionToken,
                               const QString& machineId,
                               const QString& cardCode) const {
    HttpResponse resp = WinHttpGetSigned(
        m_host,
        m_port,
        path,
        reinterpret_cast<const wchar_t*>(sessionToken.utf16()),
        reinterpret_cast<const wchar_t*>(machineId.utf16()),
        reinterpret_cast<const wchar_t*>(cardCode.utf16()));
    return ApiResult{resp.statusCode, resp.body, resp.error};
}
```

- [ ] **Step 4: Add files to `src/CMakeLists.txt`**

Add to `INJECTOR_HEADERS`:

```cmake
    services/api_client.h
```

Add `services/api_client.cpp` to the `Injector` executable source list:

```cmake
add_executable(Injector WIN32 injector_qt.cpp services/api_client.cpp ${INJECTOR_HEADERS} ${CMAKE_CURRENT_BINARY_DIR}/version.rc)
```

- [ ] **Step 5: Build Injector**

Run from repository root:

```powershell
cmake --build build --config Release --target Injector
```

Expected: build succeeds.

- [ ] **Step 6: Commit**

Run:

```powershell
git add src\services\api_client.h src\services\api_client.cpp src\CMakeLists.txt
git commit -m "feat: add client API service boundary"
```

### Task 4.2: Create Update Controller Boundary

**Files:**
- Create: `src/controllers/update_controller.h`
- Create: `src/controllers/update_controller.cpp`
- Modify: `src/CMakeLists.txt`

This first controller only owns data structures and parsing. Move actual UI update actions later.

- [ ] **Step 1: Add controller directory**

Run:

```powershell
New-Item -ItemType Directory -Force -Path src\controllers | Out-Null
```

- [ ] **Step 2: Create `src/controllers/update_controller.h`**

Content:

```cpp
#pragma once

#include <QString>
#include <QJsonObject>

struct UpdateDecision {
    bool available = false;
    QString version;
    QString packageUrl;
    QString packageSha256;
    QString packageKind;
    QString releaseId;
    QString notes;
    bool force = false;
};

class UpdateController {
public:
    static UpdateDecision parseManifest(const QJsonObject& manifest);
};
```

- [ ] **Step 3: Create `src/controllers/update_controller.cpp`**

Content:

```cpp
#include "update_controller.h"

UpdateDecision UpdateController::parseManifest(const QJsonObject& manifest) {
    UpdateDecision decision;
    decision.available = !manifest.isEmpty();
    decision.version = manifest.value("version").toString();
    decision.packageUrl = manifest.value("package_url").toString();
    decision.packageSha256 = manifest.value("package_sha256").toString();
    decision.packageKind = manifest.value("package_kind").toString("bundle");
    decision.releaseId = manifest.value("release_id").toString();
    decision.notes = manifest.value("release_notes").toString();
    decision.force = manifest.value("force_update").toBool(false);
    return decision;
}
```

- [ ] **Step 4: Add files to `src/CMakeLists.txt`**

Add to `INJECTOR_HEADERS`:

```cmake
    controllers/update_controller.h
```

Add `controllers/update_controller.cpp` to the `Injector` source list:

```cmake
add_executable(Injector WIN32 injector_qt.cpp services/api_client.cpp controllers/update_controller.cpp ${INJECTOR_HEADERS} ${CMAKE_CURRENT_BINARY_DIR}/version.rc)
```

- [ ] **Step 5: Build Injector**

Run:

```powershell
cmake --build build --config Release --target Injector
```

Expected: build succeeds.

- [ ] **Step 6: Commit**

Run:

```powershell
git add src\controllers\update_controller.h src\controllers\update_controller.cpp src\CMakeLists.txt
git commit -m "feat: add update controller boundary"
```

---

## 7. Phase 5: AutoExam Single Source Chain

Goal: ensure local source, embedded fallback, server active script, and regression tests come from one source.

### Task 5.1: Document Script Source Of Truth

**Files:**
- Create: `docs/operations/autoexam-release-chain.md`

- [ ] **Step 1: Create document**

Content:

```markdown
# AutoExam Release Chain

## Source Of Truth

`packages/autoexam/src/autoexam.js` is the long-term source of truth.

During migration, current `AutoExam.js` remains the source of truth. `AutoExam.no-dry-run.js` must not diverge.

## Required Outputs

- bundled/minified script for server upload
- generated embedded fallback header for Hook DLL
- regression test input

## Migration Steps

1. Verify `AutoExam.js` and `AutoExam.no-dry-run.js` are identical.
2. Move `AutoExam.js` to `packages/autoexam/src/autoexam.js`.
3. Keep a temporary compatibility copy or update scripts.
4. Update regression script default path.
5. Add a build script that generates all outputs from one source.
6. Remove the duplicate file after one release cycle.
```

- [ ] **Step 2: Commit**

Run:

```powershell
git add docs\operations\autoexam-release-chain.md
git commit -m "docs: define autoexam release chain"
```

### Task 5.2: Add Duplicate Script Guard

**Files:**
- Create: `scripts/check_autoexam_duplicates.ps1`

- [ ] **Step 1: Create script**

Content:

```powershell
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$main = Join-Path $root "AutoExam.js"
$variant = Join-Path $root "AutoExam.no-dry-run.js"

if (!(Test-Path $main)) {
    throw "Missing AutoExam.js"
}
if (!(Test-Path $variant)) {
    Write-Output "AutoExam.no-dry-run.js not present; duplicate guard skipped."
    exit 0
}

$mainHash = (Get-FileHash -Algorithm SHA256 $main).Hash
$variantHash = (Get-FileHash -Algorithm SHA256 $variant).Hash

if ($mainHash -ne $variantHash) {
    throw "AutoExam.js and AutoExam.no-dry-run.js differ. Main=$mainHash Variant=$variantHash"
}

Write-Output "AutoExam duplicate guard passed: $mainHash"
```

- [ ] **Step 2: Run script**

Run:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\check_autoexam_duplicates.ps1
```

Expected:

```text
AutoExam duplicate guard passed: <hash>
```

- [ ] **Step 3: Commit**

Run:

```powershell
git add scripts\check_autoexam_duplicates.ps1
git commit -m "test: guard autoexam duplicate scripts"
```

---

## 8. Phase 6: Top-Level Directory Migration

Do not start this until:

1. Server feature modules exist.
2. Client controllers exist.
3. AutoExam source chain exists.
4. Release manifest is the only active update path.
5. CI/local release scripts can build the product.

When ready:

1. Move `injector-server` to `apps/server`.
2. Move `injector-server/web/admin` to `apps/admin-web`.
3. Move `injector-server/web/agent` to `apps/agent-web`.
4. Move `src/ui`, `src/network`, `src/security`, `src/injector` to `apps/client-qt`.
5. Move `src/hook` to `apps/hook-runtime`.
6. Move `AutoExam.js` to `packages/autoexam/src/autoexam.js`.
7. Move release scripts to `packages/release-tools` only after installer scripts still work.

Every move must be a separate commit with build verification.

---

## 9. Verification Matrix

Run these at the end of each phase.

Repository root:

```powershell
git status --short
cmake --build build --config Release --target Injector
cmake --build build --config Release --target CefHook
```

`injector-server`:

```powershell
go test ./...
```

`injector-server/web`:

```powershell
npm run check
```

AutoExam:

```powershell
powershell -ExecutionPolicy Bypass -File scripts\autoexam_multimodal_provider_regression_test.ps1
powershell -ExecutionPolicy Bypass -File scripts\check_autoexam_duplicates.ps1
```

Expected final state after each phase:

```text
git status --short
# no output
```

---

## 10. Rollback Strategy

Rollback rules:

1. If a test-only commit fails later, fix the product code or adjust test expectations only after confirming current behavior.
2. If a refactor commit changes behavior unintentionally, revert that commit only, not the whole branch.
3. If a data migration fails, stop the server, restore `DATA_DIR` backup, and rerun migration on a copy.
4. If release manifest migration fails, keep legacy update path enabled until clients are fixed.
5. If client controller extraction breaks build, revert the controller commit and keep the service boundary only.

Useful commands:

```powershell
git log --oneline -5
git revert <commit>
git status --short
```

Do not use `git reset --hard` unless the project owner explicitly approves it.

---

## 11. Definition Of Done

The V2 architecture implementation is complete when:

1. `CardManager` is no longer the only business entry point.
2. Server feature services have tests.
3. Release manifest is the only active update protocol.
4. Announcement no longer carries update protocol responsibility.
5. Client `MainWindow` delegates API, update, chat, session, and injection work to controllers/services.
6. AutoExam has a single source and repeatable output generation.
7. API contracts and error codes exist and match implemented routes.
8. Security and operations docs exist and are updated when behavior changes.
9. All verification commands pass.
10. `git status --short` is clean after final commit.
