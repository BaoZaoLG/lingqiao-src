# Sessions Module Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move client activation sessions, heartbeat, logout, forced logout, validation, cleanup, and session-consuming feature authorization out of legacy `CardManager` into `server/internal/features/sessions`.

**Architecture:** Keep Lingqiao as a modular monolith and use Clean/Hexagonal boundaries. The target dependency direction is `HTTP/root compatibility adapter -> sessions.Service -> sessions.Store/Tx port -> SQLite adapter`; session business rules live in the feature service, storage and transactions stay in repository adapters, and old root handlers only remain as temporary compatibility adapters until every caller is migrated.

**Tech Stack:** Go 1.25, `database/sql`, `modernc.org/sqlite`, existing `internal/httpapi`, existing signed client request helpers, PowerShell verification scripts, and current SQLite `cards`/`sessions` tables.

---

## 0. Current State Snapshot

This plan is based on the current repository shape on `2026-06-29`.

Existing new-architecture pattern:

- `server/internal/features/cards/repository.go`
- `server/internal/features/cards/repository_sqlite.go`
- `server/internal/features/cards/service.go`
- `server/internal/features/cards/handler.go`
- focused service, repository, and handler tests

Existing sessions skeleton:

- `server/internal/features/sessions/doc.go`

Legacy session implementation still lives in root server files:

- `server/card_types.go`: `CardManager` exposes session methods.
- `server/card_sqlite.go`: SQLite activation, heartbeat, validation, logout, cleanup.
- `server/card_ops.go`: JSON activation, heartbeat, validation, logout, cleanup.
- `server/handler.go`: client activate, heartbeat, deactivate, DLL download, cleanup task.
- `server/admin_manage.go`: admin session list and force logout.
- `server/admin_overview.go`: dashboard session statistics.
- `server/chat.go`: chat endpoints call `ValidateBoundSession`.
- `server/release_handlers.go`: update check and package download call `ValidateBoundSession`.
- `server/payload_handlers.go`: key exchange calls `GetSession` and validates manually.
- `server/script_module.go`: script download calls `GetSession` and validates manually.

Current compatibility contract to preserve:

- Client activate response keeps `session_token`, `expires_at`, `card_expires_at`, `message`, and optional `update`.
- Client heartbeat response keeps `expires_at`, `card_expires_at`, `message`, and optional `update`.
- Client deactivate remains idempotent to clients: missing or expired session still returns an `ok` response.
- DLL download, chat, releases, and payload key exchange continue to reject invalid, expired, wrong-machine, wrong-card, disabled-card, expired-card, and missing-card sessions.
- Script download initially preserves the current token-only contract; do not require `machine_id` there until the client protocol is updated.
- Cleanup continues deleting expired sessions every 5 minutes.
- Admin session list returns only active sessions, sorted by `last_seen` descending, with existing pagination fields.

Command working directory convention:

- Run all `go test ...` and `gofmt ...` commands from `server/`, because the Go module is `server/go.mod`.
- Run all `npm run check` commands from `server/web/`, because the package manifest is `server/web/package.json`.
- Run `powershell -ExecutionPolicy Bypass -File scripts\check.ps1 -SkipCpp` from the repository root.
- Run `git ...`, `rg ...`, and documentation commands from the repository root unless a task says otherwise.

## 1. Target Boundaries

Target package layout:

```text
server/internal/features/sessions/
├─ doc.go
├─ model.go
├─ errors.go
├─ token.go
├─ repository.go
├─ repository_sqlite.go
├─ service.go
├─ handler.go
├─ service_test.go
├─ repository_sqlite_test.go
└─ handler_test.go

server/internal/platform/cardcode/
├─ cardcode.go
└─ cardcode_test.go
```

Responsibility map:

| File | Responsibility |
| --- | --- |
| `model.go` | Session entity, card snapshot needed by session rules, list filters, result DTOs for service layer. |
| `errors.go` | Stable session errors used by service, handlers, and compatibility adapters. |
| `token.go` | Cryptographically random session token generator. |
| `repository.go` | Store and transaction ports. Defines what persistence must provide without exposing SQLite. |
| `repository_sqlite.go` | SQLite adapter using current `sessions`, `cards`, and blacklist tables. Owns transactions. |
| `service.go` | Activation, heartbeat, logout, admin force logout, validation, listing, and cleanup rules. |
| `handler.go` | Feature HTTP handler with injected request-signing, version-check, audit, and client-IP adapters. |
| `server/handler.go` | Temporary root compatibility adapter until routes call sessions handler directly. |
| `server/admin_manage.go` | Temporary admin compatibility adapter until routes call sessions handler directly. |
| `server/admin_overview.go` | Reads session statistics from `sessions.Service`. |
| `server/chat.go` | Calls `sessions.Service.ValidateBound`. |
| `server/release_handlers.go` | Calls `sessions.Service.ValidateBound`. |
| `server/payload_handlers.go` | Replaces manual `GetSession` validation with sessions service validation. |
| `server/script_module.go` | Uses a compatibility `ValidateActiveToken` first; hardening to bound validation is a later protocol task. |

Dependency rules:

- `internal/features/sessions` must not import package `main`.
- `internal/features/sessions` may depend on `server/internal/cards` only during this migration as a compatibility domain kernel for current activation and heartbeat rules.
- `internal/features/sessions` must not call HTTP helpers directly except through its own handler adapter.
- Root `server/*.go` files may temporarily call `sessions.Service`.
- Storage code must not make business decisions beyond mapping SQL rows and maintaining transaction consistency.

## 2. Core API Design

Add the following service-facing model and ports.

### `server/internal/platform/cardcode/cardcode.go`

```go
package cardcode

import "strings"

func Normalize(code string) string {
	code = strings.ToUpper(code)
	code = strings.ReplaceAll(code, "-", "")
	code = strings.ReplaceAll(code, " ", "")
	return code
}

func Same(a, b string) bool {
	return Normalize(a) == Normalize(b)
}
```

### `server/internal/features/sessions/model.go`

```go
package sessions

import "time"

type Session struct {
	Token         string    `json:"token"`
	CardCode      string    `json:"card_code"`
	MachineID     string    `json:"machine_id"`
	Fingerprint   string    `json:"fingerprint"`
	ClientVersion string    `json:"client_version"`
	CreatedAt     time.Time `json:"created_at"`
	LastSeen      time.Time `json:"last_seen"`
	ExpiresAt     time.Time `json:"expires_at"`
	RemoteAddr    string    `json:"remote_addr"`
}

type CardSnapshot struct {
	Code        string
	MachineID   string
	AgentID     string
	CreatedAt   time.Time
	ActivatedAt *time.Time
	ExpiresAt   time.Time
	Status      string
	Note        string
	MaxSessions int
}

type ActivationInput struct {
	CardCode      string
	MachineID     string
	Fingerprint   string
	RemoteAddr    string
	ClientVersion string
}

type HeartbeatInput struct {
	SessionToken  string
	MachineID     string
	RemoteAddr    string
	ClientVersion string
}

type DeactivateInput struct {
	SessionToken string
	Unbind       bool
}

type ListFilter struct {
	ActiveOnly bool
	Page       int
	PerPage    int
}

type ListResult struct {
	Sessions []Session
	Total    int
	Page     int
	PerPage  int
}
```

### `server/internal/features/sessions/errors.go`

```go
package sessions

import "errors"

var (
	ErrTokenRequired        = errors.New("session_token is required")
	ErrMachineRequired      = errors.New("machine_id is required")
	ErrCardRequired         = errors.New("card is required")
	ErrCardNotFound         = errors.New("card not found")
	ErrCardBlacklisted      = errors.New("card blacklisted")
	ErrCardDisabled         = errors.New("card disabled")
	ErrCardExpired          = errors.New("card expired")
	ErrCardNoLongerValid    = errors.New("card no longer valid")
	ErrSessionNotFound      = errors.New("session not found")
	ErrSessionExpired       = errors.New("invalid or expired session")
	ErrMachineMismatch      = errors.New("session machine mismatch")
	ErrCardMismatch         = errors.New("session card mismatch")
	ErrMaxSessionsReached   = errors.New("max sessions reached")
	ErrBoundToOtherMachine  = errors.New("card already bound to another machine")
)
```

### `server/internal/features/sessions/repository.go`

```go
package sessions

import (
	"context"
	"time"
)

type Store interface {
	RunInTx(ctx context.Context, fn func(Tx) error) error
	GetSession(ctx context.Context, token string) (Session, error)
	ListSessions(ctx context.Context, filter ListFilter, now time.Time) ([]Session, int, error)
	DeleteExpired(ctx context.Context, now time.Time) (int, error)
}

type Tx interface {
	GetCard(ctx context.Context, code string) (CardSnapshot, error)
	IsCardBlacklisted(ctx context.Context, code string) (bool, error)
	ListSessionsByCard(ctx context.Context, code string) ([]Session, error)
	InsertSession(ctx context.Context, session Session) error
	GetSession(ctx context.Context, token string) (Session, error)
	UpdateSession(ctx context.Context, session Session) error
	DeleteSession(ctx context.Context, token string) error
	UpdateCardActivation(ctx context.Context, card CardSnapshot) error
	UnbindCardPreservingRemaining(ctx context.Context, cardCode string, now time.Time) error
}
```

### `server/internal/features/sessions/service.go`

Core service surface:

```go
package sessions

import (
	"context"
	"errors"
	"time"

	cardops "github.com/lingqiao/server/internal/cards"
	"github.com/lingqiao/server/internal/platform/cardcode"
)

type Clock func() time.Time
type TokenGenerator func() string
type AuditSink func(action, cardCode, machineID, remoteAddr string)

type Service struct {
	store         Store
	now           Clock
	sessionTTL    time.Duration
	generateToken TokenGenerator
	auditSink     AuditSink
}

func NewService(store Store, sessionTTL time.Duration, generateToken TokenGenerator, now Clock) *Service {
	if now == nil {
		now = time.Now
	}
	if sessionTTL <= 0 {
		sessionTTL = 30 * time.Minute
	}
	if generateToken == nil {
		generateToken = GenerateToken
	}
	return &Service{store: store, now: now, sessionTTL: sessionTTL, generateToken: generateToken}
}

func (s *Service) SetAuditSink(sink AuditSink) {
	s.auditSink = sink
}
```

Service methods to implement in this order:

```go
func (s *Service) Activate(ctx context.Context, input ActivationInput) (Session, CardSnapshot, error)
func (s *Service) Heartbeat(ctx context.Context, input HeartbeatInput) (Session, CardSnapshot, error)
func (s *Service) Deactivate(ctx context.Context, input DeactivateInput) error
func (s *Service) ValidateActiveToken(ctx context.Context, token string) (Session, error)
func (s *Service) ValidateBound(ctx context.Context, token, machineID, cardCode string) (Session, error)
func (s *Service) List(ctx context.Context, filter ListFilter) (ListResult, error)
func (s *Service) CleanupExpired(ctx context.Context) (int, error)
```

Activation and heartbeat should use `cardops.NewSessionService` during migration so behavior stays aligned with the existing tested domain logic:

```go
lifecycle := cardops.NewSessionService(s.now, s.sessionTTL)
```

Do not keep `cardops` forever. Once `sessions.Service` has full test coverage, move the remaining session lifecycle rules directly into `internal/features/sessions` and delete the compatibility dependency in a separate commit.

## 3. Task Plan

### Task 1: Add Legacy Contract Safety Tests

**Files:**

- Create: `server/session_contract_test.go`
- Modify only if needed: `server/card_manager_test.go`

- [ ] **Step 1: Add contract tests before changing production code**

Create tests covering current externally visible behavior:

```go
func TestSessionContractValidateBoundRejectsWrongMachineAndCard(t *testing.T)
func TestSessionContractDeactivateClientIsIdempotent(t *testing.T)
func TestSessionContractAdminListOnlyActiveSessionsSortedByLastSeen(t *testing.T)
func TestSessionContractPayloadKeyExchangeRejectsExpiredSession(t *testing.T)
func TestSessionContractScriptDownloadAcceptsTokenOnly(t *testing.T)
```

Test expectations:

- Wrong machine returns a forbidden/unauthorized session error.
- Wrong card returns a forbidden/unauthorized session error.
- Client deactivate returns `status: ok` when the token is missing or already expired.
- Admin list excludes expired sessions and sorts by `last_seen` descending.
- Payload key exchange rejects expired session tokens.
- Script download still accepts a valid token without requiring a machine header.

- [ ] **Step 2: Run only the contract tests**

```powershell
go test ./... -run "TestSessionContract" -count=1
```

Expected: all new tests pass against the current legacy implementation.

- [ ] **Step 3: Commit**

```powershell
git add server\session_contract_test.go server\card_manager_test.go
git commit -m "test: capture session compatibility contract"
```

### Task 2: Add Shared Card Code Normalization

**Files:**

- Create: `server/internal/platform/cardcode/cardcode.go`
- Create: `server/internal/platform/cardcode/cardcode_test.go`

- [ ] **Step 1: Create platform package**

Use the exact implementation shown in section 2.

- [ ] **Step 2: Add tests**

```go
package cardcode

import "testing"

func TestNormalize(t *testing.T) {
	got := Normalize(" abcd-efgh ijkl ")
	if got != "ABCDEFGHIJKL" {
		t.Fatalf("Normalize() = %q", got)
	}
}

func TestSame(t *testing.T) {
	if !Same("ABCD-EFGH", "abcd efgh") {
		t.Fatal("Same() returned false for equivalent card codes")
	}
}
```

- [ ] **Step 3: Run tests**

```powershell
go test ./internal/platform/cardcode -count=1
go test ./... -count=1
```

Expected: pass.

- [ ] **Step 4: Commit**

```powershell
git add server\internal\platform\cardcode
git commit -m "chore: add card code normalization helper"
```

### Task 3: Define Sessions Models, Errors, Token Generator, And Ports

**Files:**

- Create: `server/internal/features/sessions/model.go`
- Create: `server/internal/features/sessions/errors.go`
- Create: `server/internal/features/sessions/token.go`
- Create: `server/internal/features/sessions/repository.go`
- Create: `server/internal/features/sessions/token_test.go`

- [ ] **Step 1: Add model, errors, and repository ports**

Use the exact code in section 2.

- [ ] **Step 2: Add token generator**

```go
package sessions

import (
	"crypto/rand"
	"fmt"
)

func GenerateToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(fmt.Sprintf("generate session token: %v", err))
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:])
}
```

- [ ] **Step 3: Add token tests**

```go
package sessions

import "testing"

func TestGenerateTokenFormatAndUniqueness(t *testing.T) {
	first := GenerateToken()
	second := GenerateToken()
	if first == second {
		t.Fatal("GenerateToken returned duplicate values")
	}
	if len(first) != 36 {
		t.Fatalf("token length = %d, want 36", len(first))
	}
}
```

- [ ] **Step 4: Run tests**

```powershell
go test ./internal/features/sessions -count=1
go test ./... -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```powershell
git add server\internal\features\sessions
git commit -m "feat: define session feature contracts"
```

### Task 4: Implement Sessions Service With In-Memory Tests First

**Files:**

- Create: `server/internal/features/sessions/service.go`
- Create: `server/internal/features/sessions/service_test.go`

- [ ] **Step 1: Add in-memory store test double**

The test double should implement `Store` and `Tx` in `service_test.go` with maps:

```go
type memoryStore struct {
	cards       map[string]CardSnapshot
	sessions    map[string]Session
	blacklisted map[string]bool
}
```

It must execute `RunInTx` synchronously:

```go
func (m *memoryStore) RunInTx(ctx context.Context, fn func(Tx) error) error {
	return fn(m)
}
```

- [ ] **Step 2: Write service tests**

Add these tests before implementation:

```go
func TestServiceActivateBindsCardAndCreatesSession(t *testing.T)
func TestServiceActivateRejectsDisabledExpiredAndBlacklistedCards(t *testing.T)
func TestServiceActivateRejectsOtherMachineWhenSingleSession(t *testing.T)
func TestServiceActivateRejectsMaxSessions(t *testing.T)
func TestServiceHeartbeatExtendsSessionAndUpdatesClientVersion(t *testing.T)
func TestServiceHeartbeatDeletesExpiredOrInvalidSession(t *testing.T)
func TestServiceDeactivateDeletesSession(t *testing.T)
func TestServiceDeactivateWithUnbindPreservesRemainingDuration(t *testing.T)
func TestServiceValidateBoundRejectsMissingTokenMachineMismatchAndCardMismatch(t *testing.T)
func TestServiceValidateActiveTokenPreservesTokenOnlyCompatibility(t *testing.T)
func TestServiceListReturnsOnlyActiveSortedAndPaginated(t *testing.T)
func TestServiceCleanupExpiredDeletesOnlyExpiredSessions(t *testing.T)
```

Key test data:

```go
now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
card := CardSnapshot{
	Code:        "ABCDEF123456",
	CreatedAt:   now.Add(-24 * time.Hour),
	ExpiresAt:   now.Add(24 * time.Hour),
	Status:      "active",
	MaxSessions: 1,
}
```

- [ ] **Step 3: Run service tests and confirm failure**

```powershell
go test ./internal/features/sessions -run "TestService" -count=1
```

Expected: fail because `Service` methods are not implemented.

- [ ] **Step 4: Implement service methods**

Implementation rules:

- Normalize card codes on activation and card comparison.
- Validate `CardCode`, `SessionToken`, and `MachineID` at the service boundary.
- Use a transaction for activate, heartbeat, and deactivate.
- On activate, load card, blacklist status, and existing sessions inside the same transaction.
- On heartbeat, delete the session when it is expired or the card is no longer valid.
- On client deactivate, service may return `ErrSessionNotFound`; root client handler maps that to `ok`.
- On admin force logout, keep returning `404` for missing sessions.
- `ValidateActiveToken` checks token existence and session expiry only to preserve script download compatibility.
- `ValidateBound` checks token, session expiry, machine match, optional card match, card existence, card status, and card expiry.

- [ ] **Step 5: Run service tests**

```powershell
go test ./internal/features/sessions -run "TestService" -count=1
```

Expected: pass.

- [ ] **Step 6: Run all Go tests**

```powershell
go test ./... -count=1
```

Expected: pass.

- [ ] **Step 7: Commit**

```powershell
git add server\internal\features\sessions\service.go server\internal\features\sessions\service_test.go
git commit -m "feat: add session feature service"
```

### Task 5: Add SQLite Repository Adapter

**Files:**

- Create: `server/internal/features/sessions/repository_sqlite.go`
- Create: `server/internal/features/sessions/repository_sqlite_test.go`

- [ ] **Step 1: Write repository tests**

Repository tests must use an in-memory SQLite database with the same schema columns used by current root code:

```sql
CREATE TABLE cards (
	code TEXT PRIMARY KEY,
	machine_id TEXT,
	agent_id TEXT,
	created_at DATETIME,
	activated_at DATETIME,
	expires_at DATETIME,
	status TEXT,
	note TEXT,
	max_sessions INTEGER
);

CREATE TABLE sessions (
	token TEXT PRIMARY KEY,
	card_code TEXT,
	machine_id TEXT,
	fingerprint TEXT,
	client_version TEXT,
	created_at DATETIME,
	last_seen DATETIME,
	expires_at DATETIME,
	remote_addr TEXT
);
```

If the blacklist schema is already available in existing migrations/helpers, reuse it. If not, the repository test can create the minimal table used by current blacklist checks.

Required tests:

```go
func TestSQLiteRepositoryRunInTxRollsBackOnError(t *testing.T)
func TestSQLiteRepositoryInsertGetUpdateDeleteSession(t *testing.T)
func TestSQLiteRepositoryListSessionsActiveSortedPaginated(t *testing.T)
func TestSQLiteRepositoryCardActivationRoundTrip(t *testing.T)
func TestSQLiteRepositoryUnbindPreservingRemainingDuration(t *testing.T)
func TestSQLiteRepositoryDeleteExpired(t *testing.T)
```

- [ ] **Step 2: Run repository tests and confirm failure**

```powershell
go test ./internal/features/sessions -run "TestSQLiteRepository" -count=1
```

Expected: fail because `NewSQLiteRepository` is not implemented.

- [ ] **Step 3: Implement SQLite repository**

Required constructor:

```go
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository
```

Implementation structure:

```go
type SQLiteRepository struct {
	db *sql.DB
}

type sqliteTx struct {
	tx *sql.Tx
}
```

Transaction behavior:

```go
func (r *SQLiteRepository) RunInTx(ctx context.Context, fn func(Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(&sqliteTx{tx: tx}); err != nil {
		return err
	}
	return tx.Commit()
}
```

SQL behavior to preserve:

- `GetCard` scans `code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions`.
- `ListSessionsByCard` reads all sessions for normalized `card_code`.
- `InsertSession` inserts all current session columns.
- `UpdateSession` updates `last_seen`, `expires_at`, `client_version`, and `remote_addr`.
- `DeleteSession` deletes by token.
- `UpdateCardActivation` updates `machine_id`, `activated_at`, and `expires_at`.
- `UnbindCardPreservingRemaining` mirrors current root SQLite behavior: calculate remaining duration from `expires_at - now`, clamp to zero, set `machine_id = ''`, `activated_at = NULL`, and set `expires_at = now + remaining`.
- `DeleteExpired` deletes `expires_at < now`.
- `ListSessions` filters active sessions with `expires_at > now` when `ActiveOnly` is true, sorts by `last_seen DESC`, and applies `Page`/`PerPage`.

- [ ] **Step 4: Run repository tests**

```powershell
go test ./internal/features/sessions -run "TestSQLiteRepository" -count=1
```

Expected: pass.

- [ ] **Step 5: Run all Go tests**

```powershell
go test ./... -count=1
```

Expected: pass.

- [ ] **Step 6: Commit**

```powershell
git add server\internal\features\sessions\repository_sqlite.go server\internal\features\sessions\repository_sqlite_test.go
git commit -m "feat: add sqlite session repository"
```

### Task 6: Wire Sessions Service Beside Existing CardManager

**Files:**

- Modify: `server/main.go`
- Modify: `server/handler.go`
- Modify: `server/admin.go` or the file defining `AdminHandler`
- Modify: `server/admin_manage.go`

- [ ] **Step 1: Add service fields to root handlers**

Add to `APIHandler`:

```go
sessions *sessions.Service
```

Add setter:

```go
func (h *APIHandler) SetSessionService(svc *sessions.Service) {
	h.sessions = svc
}
```

Add to `AdminHandler`:

```go
sessions *sessions.Service
```

Add setter:

```go
func (h *AdminHandler) SetSessionService(svc *sessions.Service) {
	h.sessions = svc
}
```

Use import alias to avoid name collision:

```go
sessionfeature "github.com/lingqiao/server/internal/features/sessions"
```

- [ ] **Step 2: Instantiate sessions service in `main.go`**

After `db` is available:

```go
sessionsRepo := sessionfeature.NewSQLiteRepository(db)
sessionsSvc := sessionfeature.NewService(sessionsRepo, 30*time.Minute, sessionfeature.GenerateToken, time.Now)
sessionsSvc.SetAuditSink(func(action, cardCode, machineID, remoteAddr string) {
	cm.RecordAudit(AuditEntry{
		Time:    time.Now(),
		Action:  action,
		Card:    cardCode,
		Machine: machineID,
		Addr:    remoteAddr,
	})
})
api.SetSessionService(sessionsSvc)
admin.SetSessionService(sessionsSvc)
```

If `db == nil`, keep using legacy `CardManager` and log one startup warning. Do not panic in this migration commit.

- [ ] **Step 3: Add cleanup task using sessions service**

Either replace `api.SessionCleanupTask(ctx)` with a new service-backed task or update the existing task to prefer `h.sessions`:

```go
if h.sessions != nil {
	_, _ = h.sessions.CleanupExpired(context.Background())
	return
}
h.cm.CleanupExpired()
```

- [ ] **Step 4: Run tests**

```powershell
go test ./... -count=1
```

Expected: pass.

- [ ] **Step 5: Commit**

```powershell
git add server\main.go server\handler.go server\admin*.go
git commit -m "chore: wire session feature service"
```

### Task 7: Migrate Client Session Endpoints

**Files:**

- Modify: `server/handler.go`
- Create or modify: `server/handler_session_test.go`

- [ ] **Step 1: Update `HandleActivate` to prefer sessions service**

Behavior:

- Continue using `readSignedRequest`.
- Continue using `checkVersion`.
- Call `h.sessions.Activate` when `h.sessions != nil`.
- Fall back to `h.cm.ActivateCard` only during the compatibility window.
- Preserve response field names and messages.

Mapping:

```go
session, card, err := h.sessions.Activate(r.Context(), sessionfeature.ActivationInput{
	CardCode:      req.Card,
	MachineID:     req.MachineID,
	Fingerprint:   req.Fingerprint,
	RemoteAddr:    getClientIP(r),
	ClientVersion: req.ClientVersion,
})
```

Response:

```go
resp := map[string]interface{}{
	"session_token":   session.Token,
	"expires_at":      session.ExpiresAt.Unix(),
	"card_expires_at": card.ExpiresAt.Unix(),
	"message":         "激活成功",
}
```

- [ ] **Step 2: Update `HandleHeartbeat`**

Mapping:

```go
session, card, err := h.sessions.Heartbeat(r.Context(), sessionfeature.HeartbeatInput{
	SessionToken:  req.SessionToken,
	MachineID:     req.MachineID,
	RemoteAddr:    getClientIP(r),
	ClientVersion: req.ClientVersion,
})
```

Preserve response:

```go
resp := map[string]interface{}{
	"expires_at":      session.ExpiresAt.Unix(),
	"card_expires_at": card.ExpiresAt.Unix(),
	"message":         "心跳正常",
}
```

- [ ] **Step 3: Update `HandleDeactivate`**

Mapping:

```go
err := h.sessions.Deactivate(r.Context(), sessionfeature.DeactivateInput{
	SessionToken: req.SessionToken,
	Unbind:       false,
})
if err != nil {
	writeJSON(w, response{Status: "ok", Message: "会话不存在或已过期"})
	return
}
writeJSON(w, response{Status: "ok", Message: "已注销"})
```

- [ ] **Step 4: Update `HandleDllDownload`**

Mapping:

```go
session, err := h.sessions.ValidateBound(r.Context(), token, r.Header.Get("X-Machine-ID"), r.Header.Get("X-Card-Code"))
```

Fallback to `h.cm.ValidateBoundSession` only if `h.sessions == nil`.

- [ ] **Step 5: Add handler tests**

Add or update tests to assert:

- Activate response schema is unchanged.
- Heartbeat response schema is unchanged.
- Deactivate missing session still returns `status: ok`.
- DLL download rejects missing token and invalid session before reading DLL file.

- [ ] **Step 6: Run tests**

```powershell
go test ./... -run "Test.*Activate|Test.*Heartbeat|Test.*Deactivate|Test.*Dll" -count=1
go test ./... -count=1
```

Expected: pass.

- [ ] **Step 7: Commit**

```powershell
git add server\handler.go server\handler_session_test.go
git commit -m "feat: route client sessions through session service"
```

### Task 8: Migrate Admin Session Screens And Dashboard Statistics

**Files:**

- Modify: `server/admin_manage.go`
- Modify: `server/admin_overview.go`
- Create or modify: `server/admin_session_test.go`

- [ ] **Step 1: Update `HandleListSessions`**

Use:

```go
result, err := h.sessions.List(r.Context(), sessionfeature.ListFilter{
	ActiveOnly: true,
	Page:       page,
	PerPage:    perPage,
})
```

Preserve response fields:

```go
writeOK(w, map[string]interface{}{
	"sessions": result.Sessions,
	"total":    result.Total,
	"page":     result.Page,
	"per_page": result.PerPage,
})
```

- [ ] **Step 2: Update `HandleForceLogout`**

Use service:

```go
if err := h.sessions.Deactivate(r.Context(), sessionfeature.DeactivateInput{
	SessionToken: req.SessionToken,
	Unbind:       false,
}); err != nil {
	writeError(w, http.StatusNotFound, err.Error())
	return
}
writeOK(w, nil)
```

Admin force logout should keep missing-session as `404`.

- [ ] **Step 3: Update overview statistics**

Replace `h.cm.AllSessions()` reads with `h.sessions.List(...ActiveOnly: true...)` for active counts. If total historical sessions are needed, add a `CountSessions` method only when the UI actually displays it.

- [ ] **Step 4: Add admin tests**

Assert:

- Active sessions only.
- Sort by `last_seen` descending.
- Pagination fields match existing response.
- Force logout deletes the session.
- Missing force logout token returns `404`.

- [ ] **Step 5: Run tests**

```powershell
go test ./... -run "Test.*Session.*Admin|Test.*Overview" -count=1
go test ./... -count=1
```

Expected: pass.

- [ ] **Step 6: Commit**

```powershell
git add server\admin_manage.go server\admin_overview.go server\admin_session_test.go
git commit -m "feat: route admin sessions through session service"
```

### Task 9: Migrate Feature Consumers

**Files:**

- Modify: `server/chat.go`
- Modify: `server/release_handlers.go`
- Modify: `server/payload_handlers.go`
- Modify: `server/script_module.go`
- Create or modify focused tests for each touched handler.

- [ ] **Step 1: Migrate chat endpoints**

Replace:

```go
session, err := h.cm.ValidateBoundSession(req.SessionToken, req.MachineID, req.Card)
```

With:

```go
session, err := h.sessions.ValidateBound(r.Context(), req.SessionToken, req.MachineID, req.Card)
```

Keep fallback to `h.cm.ValidateBoundSession` only while `h.sessions == nil`.

- [ ] **Step 2: Migrate release update check and package download**

Replace `h.cm.ValidateBoundSession` with `h.sessions.ValidateBound`.

Preserve current HTTP status code and response body style for this task. Moving these routes to RFC 9457 Problem Details is a separate API-standardization task.

- [ ] **Step 3: Migrate payload key exchange**

Replace manual `GetSession` validation:

```go
session := cm.GetSession(req.SessionToken)
if session == nil || session.ExpiresAt.Before(time.Now()) {
	writeError(w, http.StatusForbidden, "invalid or expired session")
	return
}
if session.MachineID != req.MachineID {
	writeError(w, http.StatusForbidden, "machine mismatch")
	return
}
```

With service validation:

```go
session, err := sessionSvc.ValidateBound(r.Context(), req.SessionToken, req.MachineID, "")
if err != nil {
	writeError(w, http.StatusForbidden, err.Error())
	return
}
```

Because `PayloadHandler.HandleKeyExchange` currently receives `cm CardManager`, update the route wiring to pass a sessions validator or add a field:

```go
type PayloadHandler struct {
	store    *PayloadStore
	sessions *sessionfeature.Service
}
```

- [ ] **Step 4: Migrate script download without changing client protocol**

Replace raw `GetSession` with:

```go
session, err := h.sessions.ValidateActiveToken(r.Context(), token)
if err != nil {
	writeError(w, http.StatusUnauthorized, "invalid or expired session")
	return
}
_ = session
```

Do not require `X-Machine-ID` in this task. Add a future API task if script download should be machine-bound.

- [ ] **Step 5: Add tests**

Assert:

- Chat rejects wrong machine and wrong card through sessions service.
- Release update check rejects invalid sessions.
- Payload key exchange rejects expired and wrong-machine sessions through sessions service.
- Script download still works with token only.

- [ ] **Step 6: Run tests**

```powershell
go test ./... -run "Test.*Chat|Test.*Release|Test.*Payload|Test.*Script" -count=1
go test ./... -count=1
```

Expected: pass.

- [ ] **Step 7: Commit**

```powershell
git add server\chat.go server\release_handlers.go server\payload_handlers.go server\script_module.go
git commit -m "feat: migrate session consumers to session service"
```

### Task 10: Introduce Feature HTTP Handler After Adapters Are Stable

**Files:**

- Create: `server/internal/features/sessions/handler.go`
- Create: `server/internal/features/sessions/handler_test.go`
- Modify: `server/handler.go`
- Modify: `server/admin_manage.go`
- Modify: route wiring in `server/main.go` or route builder file.

- [ ] **Step 1: Add injected adapter interfaces**

`sessions.Handler` should receive dependencies instead of importing root helpers:

```go
type SignedRequestReader interface {
	ReadSignedRequest(r *http.Request) (ClientRequest, error)
}

type VersionChecker interface {
	CheckVersion(w http.ResponseWriter, clientVersion string) (bool, map[string]interface{})
}

type ClientIPFunc func(r *http.Request) string
```

Feature request DTO:

```go
type ClientRequest struct {
	Card          string
	MachineID     string
	Fingerprint   string
	ClientVersion string
	SessionToken  string
}
```

- [ ] **Step 2: Implement handler methods**

Methods:

```go
func (h *Handler) HandleActivate(w http.ResponseWriter, r *http.Request)
func (h *Handler) HandleHeartbeat(w http.ResponseWriter, r *http.Request)
func (h *Handler) HandleDeactivate(w http.ResponseWriter, r *http.Request)
func (h *Handler) HandleListSessions(w http.ResponseWriter, r *http.Request)
func (h *Handler) HandleForceLogout(w http.ResponseWriter, r *http.Request)
```

Keep response schemas byte-compatible with current root handlers.

- [ ] **Step 3: Add handler tests**

Use fake `SignedRequestReader`, fake `VersionChecker`, and fake service/store.

Assert:

- Unsupported method returns method-not-allowed.
- Bad signature returns unauthorized.
- Blocked version short-circuits activation and heartbeat.
- Activate/heartbeat/deactivate responses match current fields.
- Admin list and force logout responses match current fields.

- [ ] **Step 4: Route root handlers through feature handler**

Root `APIHandler` can delegate:

```go
func (h *APIHandler) HandleActivate(w http.ResponseWriter, r *http.Request) {
	h.sessionHandler.HandleActivate(w, r)
}
```

Do this only after feature handler tests pass.

- [ ] **Step 5: Run tests**

```powershell
go test ./internal/features/sessions -run "TestHandler" -count=1
go test ./... -count=1
```

Expected: pass.

- [ ] **Step 6: Commit**

```powershell
git add server\internal\features\sessions\handler.go server\internal\features\sessions\handler_test.go server\handler.go server\admin_manage.go server\main.go
git commit -m "feat: add session feature HTTP handler"
```

### Task 11: Remove Legacy Session Surface From CardManager

**Files:**

- Modify: `server/card_types.go`
- Modify: `server/card_sqlite.go`
- Modify: `server/card_ops.go`
- Modify: `server/card_manager_test.go`
- Modify any remaining files reported by `rg`.

- [ ] **Step 1: Confirm no root caller still uses legacy methods**

```powershell
rg -n "ActivateCard|Heartbeat\(|DeactivateSession|ValidateBoundSession|GetSession|AllSessions|CleanupExpired" server
```

Expected remaining references before deletion:

- tests intentionally covering old behavior, if not yet moved
- method definitions in `card_types.go`, `card_sqlite.go`, `card_ops.go`

No production caller should remain outside compatibility fallback code.

- [ ] **Step 2: Delete session methods from `CardManager` interface**

Remove:

```go
GetSession(token string) *Session
ValidateBoundSession(token, machineID, cardCode string) (*Session, error)
ActivateCard(code, machineID, fingerprint, addr, clientVersion string) (*Session, error)
Heartbeat(token, machineID, addr, clientVersion string) (*Session, error)
DeactivateSession(token string, unbind bool) error
AllSessions() []*Session
CleanupExpired()
```

- [ ] **Step 3: Delete legacy SQLite session methods**

Remove session method implementations from `server/card_sqlite.go` only after the search confirms no production references remain.

- [ ] **Step 4: Delete legacy JSON session methods or isolate them**

If JSON storage is still a supported runtime mode, do not delete JSON behavior in this task. Instead, create a small JSON sessions store adapter and wire it behind `sessions.Store`. If JSON runtime is no longer supported, remove the legacy JSON session methods from `server/card_ops.go`.

- [ ] **Step 5: Move tests to sessions package**

Tests in `server/card_manager_test.go` that only cover sessions should move or be rewritten under:

```text
server/internal/features/sessions/service_test.go
server/internal/features/sessions/repository_sqlite_test.go
server/internal/features/sessions/handler_test.go
```

- [ ] **Step 6: Run reference scan**

```powershell
rg -n "ActivateCard|Heartbeat\(|DeactivateSession|ValidateBoundSession|GetSession|AllSessions|CleanupExpired" server
```

Expected:

- No production reference to removed `CardManager` session methods.
- Remaining `Heartbeat` references are either HTTP handler names or sessions service methods.

- [ ] **Step 7: Run tests**

```powershell
go test ./... -count=1
```

Expected: pass.

- [ ] **Step 8: Commit**

```powershell
git add server
git commit -m "refactor: remove sessions from card manager"
```

### Task 12: Update Contracts And Handoff Documentation

**Files:**

- Modify: `docs/PROJECT_HANDOFF_REPORT.md`
- Modify: `docs/api/openapi.yaml`
- Modify: `docs/api/error-codes.md`
- Modify if needed: `docs/ARCHITECTURE_V2_PROPOSAL.md`

- [ ] **Step 1: Update handoff report**

Add a section:

```markdown
## Sessions Module

`server/internal/features/sessions` owns activation sessions, heartbeats, logout, forced logout, session validation, and expired-session cleanup.

Runtime flow:

1. Client signed request enters root compatibility handler or sessions handler.
2. Handler delegates to `sessions.Service`.
3. Service executes business rules and calls `sessions.Store`.
4. SQLite repository updates existing `cards` and `sessions` tables in one transaction.
5. Consumers such as chat, releases, payloads, scripts, and DLL download validate sessions through service methods.
```

- [ ] **Step 2: Update OpenAPI session paths**

Ensure these paths document current response fields:

- `/api/v1/activate`
- `/api/v1/heartbeat`
- `/api/v1/deactivate`

- [ ] **Step 3: Update error-code docs**

Add or verify:

```markdown
| `SESSION_NOT_FOUND` | 401 | Session token does not exist. |
| `SESSION_EXPIRED` | 401 | Session expired. |
| `MACHINE_MISMATCH` | 403 | Session machine does not match request machine. |
| `SESSION_CARD_MISMATCH` | 403 | Session card does not match request card. |
| `MAX_SESSIONS_REACHED` | 403 | Card reached active session limit. |
```

- [ ] **Step 4: Run docs and product checks**

```powershell
Push-Location server
go test ./... -count=1
Pop-Location
Push-Location server\web
npm run check
Pop-Location
powershell -ExecutionPolicy Bypass -File scripts\check.ps1 -SkipCpp
```

Expected: pass.

- [ ] **Step 5: Commit**

```powershell
git add docs server
git commit -m "docs: describe session module ownership"
```

## 4. Migration Order And Review Gates

Recommended execution order:

1. Task 1: legacy contract tests.
2. Task 2: card code normalization helper.
3. Task 3: sessions model/ports/token.
4. Task 4: sessions service.
5. Task 5: SQLite repository.
6. Task 6: wire service beside old `CardManager`.
7. Task 7: client session endpoints.
8. Task 8: admin sessions and dashboard.
9. Task 9: feature consumers.
10. Task 10: feature HTTP handler.
11. Task 11: remove legacy `CardManager` surface.
12. Task 12: docs and API contract update.

Review gates:

- Gate A after Task 5: sessions service and SQLite adapter are independently tested, but no route has changed yet.
- Gate B after Task 7: client activation/heartbeat/logout and DLL download use sessions service.
- Gate C after Task 9: all production consumers use sessions service.
- Gate D after Task 11: `CardManager` no longer owns sessions.

Do not cross a gate while `go test ./... -count=1` is failing.

## 5. Risk Register

| Risk | Why It Matters | Control |
| --- | --- | --- |
| Transaction race in activation | `max_sessions` can be bypassed if card/session reads and writes are split. | Load card, active sessions, card update, and session insert in one transaction. |
| Card expiry adjustment regression | First activation and unbind modify card expiration semantics. | Cover activation and unbind with service and SQLite tests before rewiring routes. |
| Script download protocol break | Current client may not send machine ID for script download. | Use `ValidateActiveToken` first; add machine-bound script download as a later API version task. |
| Manual `GetSession` consumers | They skip card status validation and diverge from session rules. | Replace payload/script/manual callers explicitly in Task 9. |
| Handler dependency on root helpers | `internal/features/sessions` cannot import package `main`. | Inject signed request reader, version checker, and client-IP function. |
| JSON runtime compatibility | New SQLite repository assumes DB is available. | Keep legacy fallback until JSON storage support is formally removed or a JSON sessions store exists. |
| Error message drift | Client and admin UI may rely on current strings. | Preserve response schema and messages in route tasks; standard error codes can come later. |

## 6. Verification Matrix

Run these after each implementation task that changes production code:

```powershell
Push-Location server
gofmt -w internal\features\sessions internal\platform\cardcode *.go
go test ./... -count=1
Pop-Location
```

Run these after frontend-touching or full-stack route changes:

```powershell
Push-Location server\web
npm run check
Pop-Location
powershell -ExecutionPolicy Bypass -File scripts\check.ps1 -SkipCpp
```

Run these before removing legacy methods:

```powershell
rg -n "ActivateCard|Heartbeat\(|DeactivateSession|ValidateBoundSession|GetSession|AllSessions|CleanupExpired" server
Push-Location server
go test ./... -count=1
Pop-Location
```

Expected final scan:

- `ActivateCard`, `DeactivateSession`, `ValidateBoundSession`, `GetSession`, `AllSessions`, and `CleanupExpired` are gone from `CardManager`.
- `Heartbeat` remains only as HTTP/session service names, not as `CardManager.Heartbeat`.

## 7. Rollback Strategy

Rollback rules:

1. Before Task 6, rollback is only package-local: revert the sessions feature package commit.
2. Between Task 6 and Task 9, root handlers must keep legacy fallback while the new service is being adopted.
3. If client activation or heartbeat breaks after Task 7, switch route calls back to legacy `h.cm.*` and keep the sessions service code for investigation.
4. If feature consumers break after Task 9, revert that consumer commit only; do not revert the service and repository foundation.
5. Do not remove legacy `CardManager` methods until the reference scan proves every production caller has moved.
6. Do not use `git reset --hard`; use `git revert <commit>` for committed rollback.

Useful rollback commands:

```powershell
git log --oneline -8
git revert <commit>
go test ./... -count=1
```

## 8. Definition Of Done

The sessions migration is complete when:

1. `server/internal/features/sessions` contains tested service, SQLite repository, and handler/adapters.
2. Client activate, heartbeat, deactivate, and DLL download use `sessions.Service`.
3. Admin session list, force logout, and overview statistics use `sessions.Service`.
4. Chat, releases, payload key exchange, and script download no longer call `CardManager` session methods.
5. `CardManager` no longer exposes session methods.
6. SQLite session behavior is covered by repository tests.
7. Service-level tests cover activation, heartbeat, logout, validation, listing, cleanup, and error cases.
8. API responses remain backward compatible for current clients.
9. `go test ./... -count=1`, `npm run check`, and `scripts/check.ps1 -SkipCpp` pass.
10. `rg` confirms no production dependency on legacy session methods remains.
