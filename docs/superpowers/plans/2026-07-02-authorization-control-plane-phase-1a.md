# Authorization Control Plane Phase 1A Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first read-only V3 authorization control-plane backend projection and admin API.

**Architecture:** Add `server/internal/features/authorizations` as an aggregation layer over existing `cards` and `sessions` services. It computes effective authorization lifecycle state in one backend place and exposes V3 admin routes under `/admin/api/control/authorizations` without destructive data migration.

**Tech Stack:** Go 1.25, `net/http`, existing cards repository, existing sessions store, existing admin auth middleware.

---

## File Structure

- Create `server/internal/features/authorizations/model.go`
  - Public response models for authorization list/detail projections.
- Create `server/internal/features/authorizations/service.go`
  - Read-only projection service and effective status calculation.
- Create `server/internal/features/authorizations/handler_admin.go`
  - Admin HTTP handler for list and detail endpoints.
- Create `server/internal/features/authorizations/service_test.go`
  - Unit tests for effective status and list/detail projections.
- Create `server/internal/features/authorizations/handler_admin_test.go`
  - HTTP handler tests.
- Modify `server/main.go`
  - Wire authorizations service and handler.
  - Register `/admin/api/control/authorizations` routes.

## Task 1: Add Read-Only Authorization Projection

**Files:**
- Create: `server/internal/features/authorizations/model.go`
- Create: `server/internal/features/authorizations/service.go`
- Test: `server/internal/features/authorizations/service_test.go`

- [ ] **Step 1: Write failing tests**

Create tests that prove:

- Active card past `expires_at` becomes `expired`.
- Active card with active session becomes `in_use`.
- Bound card without active session becomes `activated`.
- Unbound active card becomes `issued`.
- Disabled card becomes `frozen`.
- Detail projection returns sessions for the requested code.

Run:

```powershell
cd server
go test ./internal/features/authorizations -count=1
```

Expected: package does not compile because the package does not exist yet.

- [ ] **Step 2: Implement minimal projection service**

Implement:

- `AuthorizationSummary`
- `AuthorizationDetail`
- `LifecycleStage`
- `EffectiveStatus`
- `Service.List`
- `Service.Detail`

The service depends on:

- `cards.Repository`
- a small local `SessionLister` interface matching `ListSessions(ctx, filter, now)`.

- [ ] **Step 3: Run tests**

Run:

```powershell
cd server
go test ./internal/features/authorizations -count=1
```

Expected: pass.

## Task 2: Add Admin Control API Handler

**Files:**
- Create: `server/internal/features/authorizations/handler_admin.go`
- Test: `server/internal/features/authorizations/handler_admin_test.go`

- [ ] **Step 1: Write failing HTTP tests**

Test:

- `GET /admin/api/control/authorizations` returns `status=ok` and `authorizations`.
- `GET /admin/api/control/authorizations/{code}` returns `status=ok` and `authorization`.
- Missing detail returns 404.

Run:

```powershell
cd server
go test ./internal/features/authorizations -count=1
```

Expected: fail because handler does not exist.

- [ ] **Step 2: Implement admin handler**

Implement:

- `HandleList`
- `HandleDetail`
- `Handle`

The `Handle` method routes exact `/admin/api/control/authorizations` to list and `/admin/api/control/authorizations/{code}` to detail.

- [ ] **Step 3: Run tests**

Run:

```powershell
cd server
go test ./internal/features/authorizations -count=1
```

Expected: pass.

## Task 3: Wire Routes Into Server

**Files:**
- Modify: `server/main.go`

- [ ] **Step 1: Add package import**

Import:

```go
authorizations "github.com/lingqiao/server/internal/features/authorizations"
```

- [ ] **Step 2: Construct service and handler**

After `sessionsSvc` is created:

```go
authorizationsSvc := authorizations.NewService(cardsRepo, sessionsRepo, time.Now)
authorizationsHandler := authorizations.NewHandler(authorizationsSvc)
```

- [ ] **Step 3: Pass handler into admin route registration**

Update `buildAdminServer` and `registerAdminRoutes` signatures to include `authorizationsHandler *authorizations.Handler`.

- [ ] **Step 4: Register route**

Inside authenticated admin routes:

```go
mux.HandleFunc("/admin/api/control/authorizations", authP("cards.read", authorizationsHandler.Handle))
mux.HandleFunc("/admin/api/control/authorizations/", authP("cards.read", authorizationsHandler.Handle))
```

- [ ] **Step 5: Run full server tests**

Run:

```powershell
cd server
go test ./...
```

Expected: pass.

## Task 4: Verification Checkpoint

**Files:**
- No new files.

- [ ] **Step 1: Run backend tests**

```powershell
cd server
go test ./...
```

- [ ] **Step 2: Run frontend checks**

```powershell
cd server/web
npm run typecheck
npm run build
```

- [ ] **Step 3: Stop before Phase 1B**

Do not implement archive deletion or UI replacement in Phase 1A. Phase 1A ends when the read-only authorization control API exists and is tested.
