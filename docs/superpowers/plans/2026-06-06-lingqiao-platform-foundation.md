# Lingqiao Platform Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first platform foundation layer for configuration, API responses, storage contracts, session management, and auditing while preserving current server behavior.

**Architecture:** Add focused internal packages that can be tested independently, then bridge the current `main` package to those packages in small steps. Existing HTTP routes and JSON persistence stay compatible in this phase.

**Tech Stack:** Go 1.25, standard library HTTP/testing packages, existing `golang.org/x/crypto/bcrypt`, current JSON file storage.

---

## File Structure

- Create: `_src_tmp/injector-server/internal/config/config.go`
  - Owns typed server configuration and environment parsing.
- Create: `_src_tmp/injector-server/internal/config/config_test.go`
  - Tests defaults, environment overrides, and duration parsing.
- Create: `_src_tmp/injector-server/internal/httpapi/response.go`
  - Owns consistent API response envelopes and error codes.
- Create: `_src_tmp/injector-server/internal/httpapi/response_test.go`
  - Tests JSON status, data, meta, and error encoding.
- Create: `_src_tmp/injector-server/internal/storage/storage.go`
  - Defines storage interfaces and a JSON store implementation compatible with existing `JSONStorage`.
- Create: `_src_tmp/injector-server/internal/storage/storage_test.go`
  - Tests save/load, missing file behavior, backups, and invalid JSON handling.
- Create: `_src_tmp/injector-server/internal/auth/session.go`
  - Owns hashed token sessions, TTL, lookup, cleanup, and invalidation.
- Create: `_src_tmp/injector-server/internal/auth/session_test.go`
  - Tests token hashing, expiry, cleanup, and invalidation.
- Create: `_src_tmp/injector-server/internal/audit/audit.go`
  - Owns typed audit event append/query behavior.
- Create: `_src_tmp/injector-server/internal/audit/audit_test.go`
  - Tests event append and query filters.
- Modify: `_src_tmp/injector-server/main.go`
  - Use `internal/config` for ports and data directory wiring.
- Modify: `_src_tmp/injector-server/storage.go`
  - Keep existing `NewJSONStorage` API but delegate to `internal/storage`.
- Modify: `_src_tmp/injector-server/httputil.go`
  - Keep existing `writeOK`/`writeError` compatibility while using `internal/httpapi` envelope helpers for new routes later.

## Task 1: Typed Configuration

**Files:**
- Create: `_src_tmp/injector-server/internal/config/config_test.go`
- Create: `_src_tmp/injector-server/internal/config/config.go`
- Modify: `_src_tmp/injector-server/main.go`

- [ ] **Step 1: Write failing config tests**

Create `_src_tmp/injector-server/internal/config/config_test.go`:

```go
package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("AGENT_PORT", "")
	t.Setenv("DATA_DIR", "")
	t.Setenv("SESSION_TTL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.AdminAddr != ":48901" {
		t.Fatalf("AdminAddr = %q, want :48901", cfg.AdminAddr)
	}
	if cfg.AgentAddr != ":38472" {
		t.Fatalf("AgentAddr = %q, want :38472", cfg.AgentAddr)
	}
	if cfg.DataDir != "data" {
		t.Fatalf("DataDir = %q, want data", cfg.DataDir)
	}
	if cfg.SessionTTL != 4*time.Hour {
		t.Fatalf("SessionTTL = %s, want 4h", cfg.SessionTTL)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("PORT", "19001")
	t.Setenv("AGENT_PORT", "19002")
	t.Setenv("DATA_DIR", "custom-data")
	t.Setenv("SESSION_TTL", "30m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.AdminAddr != ":19001" {
		t.Fatalf("AdminAddr = %q, want :19001", cfg.AdminAddr)
	}
	if cfg.AgentAddr != ":19002" {
		t.Fatalf("AgentAddr = %q, want :19002", cfg.AgentAddr)
	}
	if cfg.DataDir != "custom-data" {
		t.Fatalf("DataDir = %q, want custom-data", cfg.DataDir)
	}
	if cfg.SessionTTL != 30*time.Minute {
		t.Fatalf("SessionTTL = %s, want 30m", cfg.SessionTTL)
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	t.Setenv("SESSION_TTL", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("Load should reject invalid SESSION_TTL")
	}
}
```

- [ ] **Step 2: Run config tests and verify RED**

Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./internal/config
```

Expected: FAIL because package `internal/config` or `Load` does not exist.

- [ ] **Step 3: Implement config package**

Create `_src_tmp/injector-server/internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	AdminAddr  string
	AgentAddr  string
	DataDir    string
	SessionTTL time.Duration
}

func Load() (Config, error) {
	ttl, err := durationEnv("SESSION_TTL", 4*time.Hour)
	if err != nil {
		return Config{}, err
	}
	return Config{
		AdminAddr:  ":" + stringEnv("PORT", "48901"),
		AgentAddr:  ":" + stringEnv("AGENT_PORT", "38472"),
		DataDir:    stringEnv("DATA_DIR", "data"),
		SessionTTL: ttl,
	}, nil
}

func stringEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}
	return d, nil
}
```

- [ ] **Step 4: Run config tests and verify GREEN**

Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./internal/config
```

Expected: PASS.

- [ ] **Step 5: Wire config into startup**

Modify `_src_tmp/injector-server/main.go`:

```go
import (
	"context"
	"crypto/tls"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/lingqiao/server/internal/config"
)
```

In `main()`, replace:

```go
	storage := NewJSONStorage("data")
```

with:

```go
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[CONFIG] %v", err)
	}

	storage := NewJSONStorage(cfg.DataDir)
```

Replace:

```go
	adminServer := buildAdminServer(api, admin, payload)
	agentServer := buildAgentServer(agent)
```

with:

```go
	adminServer := buildAdminServer(cfg.AdminAddr, api, admin, payload)
	agentServer := buildAgentServer(cfg.AgentAddr, agent)
```

Change function signatures:

```go
func buildAdminServer(addr string, api *APIHandler, admin *AdminHandler, payload *PayloadHandler) *http.Server
func buildAgentServer(addr string, agent *AgentHandler) *http.Server
```

Inside those functions, replace `newServer(":"+envOr("PORT", "48901"), ...)` with `newServer(addr, ...)` and `newServer(":"+envOr("AGENT_PORT", "38472"), ...)` with `newServer(addr, ...)`.

- [ ] **Step 6: Run all tests**

Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit when git is available**

Run after repository state is resolved:

```powershell
git add _src_tmp/injector-server/internal/config _src_tmp/injector-server/main.go
git commit -m "feat: add typed server configuration"
```

## Task 2: API Response Envelope

**Files:**
- Create: `_src_tmp/injector-server/internal/httpapi/response_test.go`
- Create: `_src_tmp/injector-server/internal/httpapi/response.go`
- Modify: `_src_tmp/injector-server/httputil.go`

- [ ] **Step 1: Write failing response tests**

Create `_src_tmp/injector-server/internal/httpapi/response_test.go`:

```go
package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteOKIncludesStatusDataAndMeta(t *testing.T) {
	rr := httptest.NewRecorder()

	WriteOK(rr, map[string]any{"name": "lingqiao"}, map[string]any{"page": 1})

	if rr.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", rr.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["status"] != "ok" {
		t.Fatalf("status = %v, want ok", got["status"])
	}
	if got["data"].(map[string]any)["name"] != "lingqiao" {
		t.Fatalf("data.name mismatch: %#v", got["data"])
	}
	if got["meta"].(map[string]any)["page"].(float64) != 1 {
		t.Fatalf("meta.page mismatch: %#v", got["meta"])
	}
}

func TestWriteErrorIncludesCodeAndMessage(t *testing.T) {
	rr := httptest.NewRecorder()

	WriteError(rr, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want 401", rr.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["status"] != "error" {
		t.Fatalf("status = %v, want error", got["status"])
	}
	if got["code"] != "UNAUTHORIZED" {
		t.Fatalf("code = %v, want UNAUTHORIZED", got["code"])
	}
	if got["message"] != "unauthorized" {
		t.Fatalf("message = %v, want unauthorized", got["message"])
	}
}
```

- [ ] **Step 2: Run response tests and verify RED**

Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./internal/httpapi
```

Expected: FAIL because package `internal/httpapi` or `WriteOK` does not exist.

- [ ] **Step 3: Implement response package**

Create `_src_tmp/injector-server/internal/httpapi/response.go`:

```go
package httpapi

import (
	"encoding/json"
	"net/http"
)

type OKResponse struct {
	Status string         `json:"status"`
	Data   map[string]any `json:"data,omitempty"`
	Meta   map[string]any `json:"meta,omitempty"`
}

type ErrorResponse struct {
	Status  string `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func WriteOK(w http.ResponseWriter, data map[string]any, meta map[string]any) {
	WriteJSON(w, http.StatusOK, OKResponse{
		Status: "ok",
		Data:   data,
		Meta:   meta,
	})
}

func WriteError(w http.ResponseWriter, status int, code string, message string) {
	WriteJSON(w, status, ErrorResponse{
		Status:  "error",
		Code:    code,
		Message: message,
	})
}
```

- [ ] **Step 4: Run response tests and verify GREEN**

Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./internal/httpapi
```

Expected: PASS.

- [ ] **Step 5: Preserve current compatibility helpers**

Modify `_src_tmp/injector-server/httputil.go` only if needed to import `internal/httpapi` later. Current client/admin responses must keep their current top-level fields in Phase 1, so do not rewrite `writeOK` yet.

- [ ] **Step 6: Run all tests**

Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit when git is available**

```powershell
git add _src_tmp/injector-server/internal/httpapi
git commit -m "feat: add API response envelope helpers"
```

## Task 3: Storage Interface and JSON Store

**Files:**
- Create: `_src_tmp/injector-server/internal/storage/storage_test.go`
- Create: `_src_tmp/injector-server/internal/storage/storage.go`
- Modify: `_src_tmp/injector-server/storage.go`

- [ ] **Step 1: Write failing storage tests**

Create `_src_tmp/injector-server/internal/storage/storage_test.go`:

```go
package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type sample struct {
	Name string `json:"name"`
	N    int    `json:"n"`
}

func TestJSONStoreSaveLoadAndBackup(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONStore(dir)

	if err := store.Save("sample", sample{Name: "one", N: 1}); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}
	if err := store.Save("sample", sample{Name: "two", N: 2}); err != nil {
		t.Fatalf("second Save failed: %v", err)
	}

	var got sample
	if err := store.Load("sample", &got); err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if got.Name != "two" || got.N != 2 {
		t.Fatalf("loaded %#v, want second value", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "sample.json.bak")); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}
}

func TestJSONStoreLoadMissingFileReturnsNil(t *testing.T) {
	store := NewJSONStore(t.TempDir())

	var got sample
	if err := store.Load("missing", &got); err != nil {
		t.Fatalf("Load missing returned error: %v", err)
	}
}

func TestJSONStoreRejectsInvalidName(t *testing.T) {
	store := NewJSONStore(t.TempDir())

	err := store.Save("../escape", sample{Name: "bad"})
	if err == nil {
		t.Fatal("Save should reject path traversal name")
	}
	if !strings.Contains(err.Error(), "invalid storage name") {
		t.Fatalf("error = %v, want invalid storage name", err)
	}
}
```

- [ ] **Step 2: Run storage tests and verify RED**

Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./internal/storage
```

Expected: FAIL because `NewJSONStore` does not exist.

- [ ] **Step 3: Implement storage package**

Create `_src_tmp/injector-server/internal/storage/storage.go`:

```go
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Store interface {
	Save(name string, value any) error
	Load(name string, value any) error
}

type JSONStore struct {
	dir string
	mu  sync.Mutex
}

func NewJSONStore(dir string) *JSONStore {
	_ = os.MkdirAll(dir, 0755)
	return &JSONStore{dir: dir}
}

func (s *JSONStore) Save(name string, value any) error {
	if err := validateName(name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, name+".json")
	tmpPath := path + ".tmp"
	bakPath := path + ".bak"

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		_ = os.Remove(bakPath)
		if err := os.Rename(path, bakPath); err != nil {
			_ = os.Remove(tmpPath)
			return err
		}
	}
	return os.Rename(tmpPath, path)
}

func (s *JSONStore) Load(name string, value any) error {
	if err := validateName(name); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(filepath.Join(s.dir, name+".json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, value)
}

func validateName(name string) error {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return fmt.Errorf("invalid storage name: %q", name)
	}
	return nil
}
```

- [ ] **Step 4: Run storage tests and verify GREEN**

Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./internal/storage
```

Expected: PASS.

- [ ] **Step 5: Delegate current storage wrapper**

Modify `_src_tmp/injector-server/storage.go`:

```go
package main

import "github.com/lingqiao/server/internal/storage"

type JSONStorage struct {
	store storage.Store
}

func NewJSONStorage(dir string) *JSONStorage {
	return &JSONStorage{store: storage.NewJSONStore(dir)}
}

func (s *JSONStorage) Save(name string, v interface{}) error {
	return s.store.Save(name, v)
}

func (s *JSONStorage) Load(name string, v interface{}) error {
	return s.store.Load(name, v)
}
```

- [ ] **Step 6: Run all tests**

Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./...
```

Expected: PASS.

- [ ] **Step 7: Commit when git is available**

```powershell
git add _src_tmp/injector-server/internal/storage _src_tmp/injector-server/storage.go
git commit -m "feat: add storage interface and JSON implementation"
```

## Task 4: Auth Session Service

**Files:**
- Create: `_src_tmp/injector-server/internal/auth/session_test.go`
- Create: `_src_tmp/injector-server/internal/auth/session.go`

- [ ] **Step 1: Write failing session tests**

Create `_src_tmp/injector-server/internal/auth/session_test.go`:

```go
package auth

import (
	"strings"
	"testing"
	"time"
)

func TestSessionStoreCreatesAndChecksSession(t *testing.T) {
	store := NewSessionStore("admin", time.Hour)

	token, err := store.Create("actor-1")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if !strings.HasPrefix(token, "admin_") {
		t.Fatalf("token = %q, want admin_ prefix", token)
	}
	actor, ok := store.Check(token)
	if !ok {
		t.Fatal("Check should accept fresh token")
	}
	if actor != "actor-1" {
		t.Fatalf("actor = %q, want actor-1", actor)
	}
	if store.RawTokenStoredForTest(token) {
		t.Fatal("session store must not store raw token")
	}
}

func TestSessionStoreRejectsExpiredSession(t *testing.T) {
	store := NewSessionStore("agent", time.Nanosecond)

	token, err := store.Create("agent-1")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	time.Sleep(time.Millisecond)

	if _, ok := store.Check(token); ok {
		t.Fatal("Check should reject expired token")
	}
}

func TestSessionStoreInvalidate(t *testing.T) {
	store := NewSessionStore("agent", time.Hour)

	token, err := store.Create("agent-1")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	store.Invalidate(token)

	if _, ok := store.Check(token); ok {
		t.Fatal("Check should reject invalidated token")
	}
}
```

- [ ] **Step 2: Run session tests and verify RED**

Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./internal/auth
```

Expected: FAIL because `NewSessionStore` does not exist.

- [ ] **Step 3: Implement session service**

Create `_src_tmp/injector-server/internal/auth/session.go`:

```go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

type Session struct {
	ActorID   string
	ExpiresAt time.Time
}

type SessionStore struct {
	prefix string
	ttl    time.Duration
	mu     sync.Mutex
	items  map[string]Session
}

func NewSessionStore(prefix string, ttl time.Duration) *SessionStore {
	return &SessionStore{
		prefix: prefix,
		ttl:    ttl,
		items:  make(map[string]Session),
	}
}

func (s *SessionStore) Create(actorID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := s.prefix + "_" + hex.EncodeToString(b)
	s.mu.Lock()
	s.items[HashToken(token)] = Session{ActorID: actorID, ExpiresAt: time.Now().Add(s.ttl)}
	s.mu.Unlock()
	return token, nil
}

func (s *SessionStore) Check(token string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.items[HashToken(token)]
	if !ok || !time.Now().Before(session.ExpiresAt) {
		return "", false
	}
	return session.ActorID, true
}

func (s *SessionStore) Invalidate(token string) {
	s.mu.Lock()
	delete(s.items, HashToken(token))
	s.mu.Unlock()
}

func (s *SessionStore) Cleanup(now time.Time) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	for tokenHash, session := range s.items {
		if !now.Before(session.ExpiresAt) {
			delete(s.items, tokenHash)
			removed++
		}
	}
	return removed
}

func (s *SessionStore) RawTokenStoredForTest(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.items[token]
	return ok
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: Run session tests and verify GREEN**

Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./internal/auth
```

Expected: PASS.

- [ ] **Step 5: Run all tests**

Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit when git is available**

```powershell
git add _src_tmp/injector-server/internal/auth
git commit -m "feat: add hashed auth session store"
```

## Task 5: Audit Service Shell

**Files:**
- Create: `_src_tmp/injector-server/internal/audit/audit_test.go`
- Create: `_src_tmp/injector-server/internal/audit/audit.go`

- [ ] **Step 1: Write failing audit tests**

Create `_src_tmp/injector-server/internal/audit/audit_test.go`:

```go
package audit

import (
	"testing"
	"time"
)

func TestRecorderAppendsAndQueriesEvents(t *testing.T) {
	rec := NewRecorder()
	now := time.Now()

	rec.Append(Event{Time: now, Action: "card_generated", ActorID: "admin", Card: "ABC"})
	rec.Append(Event{Time: now.Add(time.Second), Action: "agent_login", ActorID: "agent-1"})

	events := rec.Query(Filter{Action: "card_generated"})
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Card != "ABC" {
		t.Fatalf("Card = %q, want ABC", events[0].Card)
	}
}

func TestRecorderQueryByTimeRange(t *testing.T) {
	rec := NewRecorder()
	base := time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC)

	rec.Append(Event{Time: base.Add(-time.Hour), Action: "old"})
	rec.Append(Event{Time: base, Action: "inside"})
	rec.Append(Event{Time: base.Add(time.Hour), Action: "new"})

	events := rec.Query(Filter{From: base.Add(-time.Minute), To: base.Add(time.Minute)})
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].Action != "inside" {
		t.Fatalf("Action = %q, want inside", events[0].Action)
	}
}
```

- [ ] **Step 2: Run audit tests and verify RED**

Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./internal/audit
```

Expected: FAIL because `NewRecorder` does not exist.

- [ ] **Step 3: Implement audit package**

Create `_src_tmp/injector-server/internal/audit/audit.go`:

```go
package audit

import (
	"sync"
	"time"
)

type Event struct {
	Time    time.Time
	Action  string
	ActorID string
	Card    string
	AgentID string
	Machine string
	IP      string
	Detail  string
}

type Filter struct {
	Action  string
	ActorID string
	Card    string
	AgentID string
	Machine string
	IP      string
	From    time.Time
	To      time.Time
}

type Recorder struct {
	mu     sync.RWMutex
	events []Event
}

func NewRecorder() *Recorder {
	return &Recorder{events: make([]Event, 0)}
}

func (r *Recorder) Append(event Event) {
	if event.Time.IsZero() {
		event.Time = time.Now()
	}
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
}

func (r *Recorder) Query(filter Filter) []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Event, 0)
	for _, event := range r.events {
		if !matches(event, filter) {
			continue
		}
		out = append(out, event)
	}
	return out
}

func matches(event Event, filter Filter) bool {
	if filter.Action != "" && event.Action != filter.Action {
		return false
	}
	if filter.ActorID != "" && event.ActorID != filter.ActorID {
		return false
	}
	if filter.Card != "" && event.Card != filter.Card {
		return false
	}
	if filter.AgentID != "" && event.AgentID != filter.AgentID {
		return false
	}
	if filter.Machine != "" && event.Machine != filter.Machine {
		return false
	}
	if filter.IP != "" && event.IP != filter.IP {
		return false
	}
	if !filter.From.IsZero() && event.Time.Before(filter.From) {
		return false
	}
	if !filter.To.IsZero() && event.Time.After(filter.To) {
		return false
	}
	return true
}
```

- [ ] **Step 4: Run audit tests and verify GREEN**

Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./internal/audit
```

Expected: PASS.

- [ ] **Step 5: Run all tests**

Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit when git is available**

```powershell
git add _src_tmp/injector-server/internal/audit
git commit -m "feat: add audit recorder service"
```

## Task 6: Baseline Documentation

**Files:**
- Create: `_src_tmp/injector-server/README.md`

- [ ] **Step 1: Create runtime documentation**

Create `_src_tmp/injector-server/README.md`:

```markdown
# Lingqiao Server

## Test

PowerShell:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'
go test ./...
```

## Run

```powershell
$env:PORT='48901'
$env:AGENT_PORT='38472'
$env:DATA_DIR='data'
go run .
```

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `PORT` | `48901` | Admin and client API HTTPS port |
| `AGENT_PORT` | `38472` | Agent panel HTTPS port |
| `DATA_DIR` | `data` | JSON persistence directory |
| `SESSION_TTL` | `4h` | Default platform session TTL for new internal services |
| `ADMIN_PASSWORD` | generated | Initial admin password when no password hash exists |
| `HMAC_SECRET` | generated | Client API HMAC secret |
| `ADMIN_ORIGIN` | empty | Optional allowed admin CORS origin |
| `AGENT_ORIGIN` | empty | Optional allowed agent CORS origin |

## Notes

Existing JSON data remains the persistence format for the platform foundation phase. Keep `data/` backed up before migration work.
```
```

- [ ] **Step 2: Run all tests**

Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./...
```

Expected: PASS.

- [ ] **Step 3: Commit when git is available**

```powershell
git add _src_tmp/injector-server/README.md
git commit -m "docs: add server runtime notes"
```

## Self-Review Checklist

- [ ] Spec coverage: Phase 1 covers config, response envelope, storage interface, session service, audit shell, startup wiring, and docs.
- [ ] Placeholder scan: no `TBD`, `TODO`, `FIXME`, `implement later`, or vague test instructions remain.
- [ ] Type consistency: package names and function names match across test and implementation steps.
- [ ] Safety check: no task enhances injection, stealth, bypass, or covert delivery behavior.
- [ ] Verification check: every production code task starts with a failing test and ends with `go test ./...`.

## Execution Choice

Plan complete and saved to `docs/superpowers/plans/2026-06-06-lingqiao-platform-foundation.md`.

Because the user requested execution immediately, default to inline execution in the current session using `superpowers:executing-plans`, task by task with verification checkpoints. Subagent-driven execution can be used later when the workspace is git-backed and tasks can be reviewed independently.
