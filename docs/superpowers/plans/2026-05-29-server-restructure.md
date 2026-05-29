# Server Restructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate the Go server from a monolithic JSON-file-based architecture to a modular PostgreSQL-backed architecture with proper separation of concerns.

**Architecture:** Domain-driven layered architecture with repository pattern. Single process, clean internal package boundaries. Chi router, pgx database driver, golang-migrate for schema management.

**Tech Stack:** Go 1.22+, Chi (router), pgx (PostgreSQL driver), golang-migrate (migrations), testify (testing), Docker Compose (dev environment)

---

## File Map

```
server/
├── cmd/
│   ├── server/main.go                    # Application entry, dependency injection
│   └── migrate-json/main.go              # JSON → PostgreSQL data migration tool
├── internal/
│   ├── domain/
│   │   ├── card.go                       # Card entity + business rules
│   │   ├── session.go                    # Session entity
│   │   ├── user.go                       # Admin/Agent user entity
│   │   ├── machine.go                    # Machine entity
│   │   ├── announcement.go               # Announcement entity
│   │   └── errors.go                     # Domain error types
│   ├── repository/
│   │   ├── interfaces.go                 # Repository interfaces
│   │   ├── postgres/
│   │   │   ├── db.go                     # Connection pool setup
│   │   │   ├── card_repo.go              # Card repository implementation
│   │   │   ├── session_repo.go           # Session repository implementation
│   │   │   ├── user_repo.go              # User repository implementation
│   │   │   ├── machine_repo.go           # Machine repository implementation
│   │   │   ├── audit_repo.go             # Audit log repository implementation
│   │   │   ├── cache_repo.go             # Answer cache repository implementation
│   │   │   └── script_repo.go            # Script/adapter repository implementation
│   │   └── memory/
│   │       └── repos.go                  # In-memory implementations (testing)
│   ├── service/
│   │   ├── card_service.go               # Card CRUD + activation logic
│   │   ├── session_service.go            # Session lifecycle management
│   │   ├── auth_service.go               # Authentication (HMAC + JWT)
│   │   ├── admin_service.go              # Admin operations
│   │   ├── agent_service.go              # Agent operations
│   │   └── ai_gateway.go                 # AI model routing (placeholder for M3)
│   ├── handler/
│   │   ├── client.go                     # Client API handlers (activate, heartbeat, dll)
│   │   ├── admin.go                      # Admin panel API handlers
│   │   ├── agent.go                      # Agent panel API handlers
│   │   └── response.go                   # Unified response helpers
│   ├── middleware/
│   │   ├── auth.go                       # HMAC + JWT authentication middleware
│   │   ├── ratelimit.go                  # Rate limiting middleware
│   │   ├── cors.go                       # CORS middleware
│   │   └── logging.go                    # Structured request logging
│   └── config/
│       └── config.go                     # Configuration loading (env + YAML)
├── migrations/
│   ├── 001_init.up.sql                   # Initial schema
│   └── 001_init.down.sql                 # Rollback
├── docker-compose.yml                    # PostgreSQL + server
├── Makefile                              # Build, migrate, test, run targets
├── go.mod
└── go.sum
```

---

## Task 1: Project Structure and Dependencies

**Files:**
- Create: `server/go.mod` (replace existing)
- Create: `server/Makefile`
- Create: `server/docker-compose.yml`
- Create: `server/.env.example`

- [ ] **Step 1: Initialize new Go module**

```bash
cd server
# Backup old files
cp go.mod go.mod.bak
cp main.go main.go.bak
# Create new module
rm go.mod
go mod init github.com/lingqiao/server
go mod edit -go=1.22
```

- [ ] **Step 2: Add dependencies**

```bash
go get github.com/go-chi/chi/v5@latest
go get github.com/go-chi/cors@latest
go get github.com/jackc/pgx/v5@latest
go get github.com/golang-migrate/migrate/v4@latest
go get github.com/golang-migrate/migrate/v4/database/postgres@latest
go get github.com/golang-migrate/migrate/v4/source/file@latest
go get github.com/stretchr/testify@latest
go get github.com/golang-jwt/jwt/v5@latest
go get gopkg.in/yaml.v3@latest
```

- [ ] **Step 3: Create Makefile**

```makefile
# server/Makefile
.PHONY: build run test migrate migrate-up migrate-down docker-up docker-down

build:
	go build -o bin/server ./cmd/server/

run: build
	./bin/server

test:
	go test ./... -v -count=1

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down

migrate-create:
	migrate create -ext sql -dir migrations -seq $(NAME)

docker-up:
	docker compose up -d

docker-down:
	docker compose down

lint:
	golangci-lint run
```

- [ ] **Step 4: Create docker-compose.yml**

```yaml
# server/docker-compose.yml
version: '3.8'
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: lingqiao
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: lingqiao_dev
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  pgdata:
```

- [ ] **Step 5: Create .env.example**

```
# server/.env.example
DATABASE_URL=postgres://postgres:lingqiao_dev@localhost:5432/lingqiao?sslmode=disable
SERVER_ADMIN_PORT=48901
SERVER_AGENT_PORT=38472
TLS_CERT=./certs/server.crt
TLS_KEY=./certs/server.key
ADMIN_PASSWORD=changeme
HMAC_SECRET=changeme
JWT_SECRET=changeme
UPLOAD_KEY=changeme
DEEPSEEK_API_KEY=
OPENAI_API_KEY=
```

- [ ] **Step 6: Create directory structure**

```bash
cd server
mkdir -p cmd/server cmd/migrate-json
mkdir -p internal/domain internal/repository/postgres internal/repository/memory
mkdir -p internal/service internal/handler internal/middleware internal/config
mkdir -p migrations
```

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum Makefile docker-compose.yml .env.example
git commit -m "chore: initialize project structure with dependencies"
```

---

## Task 2: Domain Models

**Files:**
- Create: `server/internal/domain/errors.go`
- Create: `server/internal/domain/card.go`
- Create: `server/internal/domain/session.go`
- Create: `server/internal/domain/user.go`
- Create: `server/internal/domain/machine.go`
- Create: `server/internal/domain/announcement.go`

- [ ] **Step 1: Define domain errors**

```go
// server/internal/domain/errors.go
package domain

import "errors"

var (
    ErrCardNotFound      = errors.New("card not found")
    ErrCardDisabled      = errors.New("card is disabled")
    ErrCardExpired       = errors.New("card has expired")
    ErrCardAlreadyActive = errors.New("card is already active")
    ErrMachineBlacklisted = errors.New("machine is blacklisted")
    ErrSessionNotFound   = errors.New("session not found")
    ErrSessionExpired    = errors.New("session has expired")
    ErrUserNotFound      = errors.New("user not found")
    ErrDuplicateCard     = errors.New("card code already exists")
    ErrDuplicateUser     = errors.New("username already exists")
    ErrInvalidCredentials = errors.New("invalid credentials")
    ErrMaxSessions       = errors.New("maximum sessions reached")
    ErrInvalidCardCode   = errors.New("invalid card code format")
)
```

- [ ] **Step 2: Define Card entity**

```go
// server/internal/domain/card.go
package domain

import (
    "crypto/rand"
    "fmt"
    "strings"
    "time"

    "github.com/google/uuid"
)

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

type CardStatus string

const (
    CardStatusUnused   CardStatus = "unused"
    CardStatusActive   CardStatus = "active"
    CardStatusExpired  CardStatus = "expired"
    CardStatusDisabled CardStatus = "disabled"
)

type Card struct {
    ID          uuid.UUID  `json:"id"`
    Code        string     `json:"code"`
    Duration    int        `json:"duration_hours"`
    Status      CardStatus `json:"status"`
    AgentID     *uuid.UUID `json:"agent_id,omitempty"`
    Note        string     `json:"note,omitempty"`
    MaxSessions int        `json:"max_sessions"`
    MachineFP   string     `json:"machine_fp,omitempty"`
    CreatedAt   time.Time  `json:"created_at"`
    ActivatedAt *time.Time `json:"activated_at,omitempty"`
    ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

func GenerateCardCode() (string, error) {
    buf := make([]byte, 18)
    if _, err := rand.Read(buf); err != nil {
        return "", fmt.Errorf("generate card code: %w", err)
    }
    for i, b := range buf {
        buf[i] = crockford[b%32]
    }
    code := string(buf[:6]) + "-" + string(buf[6:12]) + "-" + string(buf[12:18])
    return code, nil
}

func NormalizeCardCode(code string) string {
    code = strings.ToUpper(code)
    code = strings.ReplaceAll(code, " ", "")
    code = strings.ReplaceAll(code, "-", "")
    if len(code) == 18 {
        return code[:6] + "-" + code[6:12] + "-" + code[12:18]
    }
    return code
}

func (c *Card) IsExpired() bool {
    return c.ExpiresAt != nil && time.Now().After(*c.ExpiresAt)
}

func (c *Card) CanActivate() error {
    switch c.Status {
    case CardStatusDisabled:
        return ErrCardDisabled
    case CardStatusExpired:
        return ErrCardExpired
    case CardStatusActive:
        if c.IsExpired() {
            return ErrCardExpired
        }
        return nil
    }
    return nil
}
```

- [ ] **Step 3: Define Session entity**

```go
// server/internal/domain/session.go
package domain

import (
    "time"

    "github.com/google/uuid"
)

type Session struct {
    ID            uuid.UUID  `json:"id"`
    CardID        uuid.UUID  `json:"card_id"`
    CardCode      string     `json:"card_code"`
    MachineFP     string     `json:"machine_fp"`
    ClientVersion string     `json:"client_version,omitempty"`
    IPAddress     string     `json:"ip_address"`
    CreatedAt     time.Time  `json:"created_at"`
    LastSeenAt    time.Time  `json:"last_seen_at"`
    ExpiresAt     time.Time  `json:"expires_at"`
}

func (s *Session) IsExpired() bool {
    return time.Now().After(s.ExpiresAt)
}
```

- [ ] **Step 4: Define User entity**

```go
// server/internal/domain/user.go
package domain

import (
    "time"

    "github.com/google/uuid"
)

type UserRole string

const (
    RoleAdmin UserRole = "admin"
    RoleAgent UserRole = "agent"
)

type User struct {
    ID        uuid.UUID  `json:"id"`
    Username  string     `json:"username"`
    Password  string     `json:"-"` // bcrypt hash
    Role      UserRole   `json:"role"`
    Prefix    string     `json:"prefix,omitempty"` // agent card prefix
    Disabled  bool       `json:"disabled"`
    CreatedAt time.Time  `json:"created_at"`
}
```

- [ ] **Step 5: Define Machine entity**

```go
// server/internal/domain/machine.go
package domain

import "time"

type Machine struct {
    Fingerprint string     `json:"fingerprint"`
    IPAddress   string     `json:"ip_address"`
    Blacklisted bool       `json:"blacklisted"`
    Note        string     `json:"note,omitempty"`
    FirstSeen   time.Time  `json:"first_seen"`
    LastSeen    time.Time  `json:"last_seen"`
    CardCount   int        `json:"card_count"` // computed
}
```

- [ ] **Step 6: Define Announcement entity**

```go
// server/internal/domain/announcement.go
package domain

import "time"

type Announcement struct {
    ID            int64     `json:"id"`
    Content       string    `json:"content"`
    LatestVersion string    `json:"latest_version"`
    MinVersion    string    `json:"min_version"`
    ForceUpdate   bool      `json:"force_update"`
    DownloadURL   string    `json:"download_url,omitempty"`
    UpdatedAt     time.Time `json:"updated_at"`
}
```

- [ ] **Step 7: Run `go build ./internal/domain/` to verify compilation**

- [ ] **Step 8: Commit**

```bash
git add internal/domain/
git commit -m "feat: define domain models (card, session, user, machine, announcement)"
```

---

## Task 3: Database Schema and Migrations

**Files:**
- Create: `server/migrations/001_init.up.sql`
- Create: `server/migrations/001_init.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- server/migrations/001_init.up.sql

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users (admin + agent)
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username    VARCHAR(50) UNIQUE NOT NULL,
    password    VARCHAR(128) NOT NULL,
    role        VARCHAR(20) NOT NULL CHECK (role IN ('admin', 'agent')),
    prefix      VARCHAR(10) DEFAULT '',
    disabled    BOOLEAN DEFAULT false,
    created_at  TIMESTAMPTZ DEFAULT now()
);

-- Cards
CREATE TABLE cards (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code         VARCHAR(20) UNIQUE NOT NULL,
    duration     INTERVAL NOT NULL,
    status       VARCHAR(20) DEFAULT 'unused' CHECK (status IN ('unused', 'active', 'expired', 'disabled')),
    agent_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    note         TEXT DEFAULT '',
    max_sessions INT DEFAULT 1,
    machine_fp   VARCHAR(64) DEFAULT '',
    created_at   TIMESTAMPTZ DEFAULT now(),
    activated_at TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ
);

-- Sessions
CREATE TABLE sessions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id        UUID NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    card_code      VARCHAR(20) NOT NULL,
    machine_fp     VARCHAR(64) NOT NULL,
    client_version VARCHAR(20) DEFAULT '',
    ip_address     VARCHAR(45) DEFAULT '',
    created_at     TIMESTAMPTZ DEFAULT now(),
    last_seen_at   TIMESTAMPTZ DEFAULT now(),
    expires_at     TIMESTAMPTZ NOT NULL
);

-- Machines
CREATE TABLE machines (
    fingerprint VARCHAR(64) PRIMARY KEY,
    ip_address  VARCHAR(45) DEFAULT '',
    blacklisted BOOLEAN DEFAULT false,
    note        TEXT DEFAULT '',
    first_seen  TIMESTAMPTZ DEFAULT now(),
    last_seen   TIMESTAMPTZ DEFAULT now()
);

-- Audit log
CREATE TABLE audit_logs (
    id         BIGSERIAL PRIMARY KEY,
    actor_id   UUID,
    action     VARCHAR(50) NOT NULL,
    target_type VARCHAR(30) DEFAULT '',
    target_id  VARCHAR(64) DEFAULT '',
    detail     TEXT DEFAULT '',
    ip_address VARCHAR(45) DEFAULT '',
    created_at TIMESTAMPTZ DEFAULT now()
);

-- Announcements
CREATE TABLE announcements (
    id             BIGSERIAL PRIMARY KEY,
    content        TEXT DEFAULT '',
    latest_version VARCHAR(20) DEFAULT '',
    min_version    VARCHAR(20) DEFAULT '',
    force_update   BOOLEAN DEFAULT false,
    download_url   TEXT DEFAULT '',
    updated_at     TIMESTAMPTZ DEFAULT now()
);

-- Answer cache
CREATE TABLE answer_cache (
    question_hash  VARCHAR(64) PRIMARY KEY,
    platform       VARCHAR(30) NOT NULL,
    question_type  VARCHAR(20) NOT NULL,
    question_text  TEXT NOT NULL,
    answer         TEXT NOT NULL,
    model          VARCHAR(50) NOT NULL,
    confidence     REAL DEFAULT 1.0,
    hit_count      INT DEFAULT 0,
    created_at     TIMESTAMPTZ DEFAULT now(),
    last_hit_at    TIMESTAMPTZ
);

-- Scripts (JS payloads)
CREATE TABLE scripts (
    id         VARCHAR(30) PRIMARY KEY,
    version    VARCHAR(20) NOT NULL,
    js_code    TEXT NOT NULL,
    signature  VARCHAR(128) NOT NULL,
    enabled    BOOLEAN DEFAULT true,
    updated_at TIMESTAMPTZ DEFAULT now()
);

-- Platform adapters
CREATE TABLE platform_adapters (
    id             VARCHAR(30) PRIMARY KEY,
    name           VARCHAR(100) NOT NULL,
    version        VARCHAR(20) NOT NULL,
    js_code        TEXT NOT NULL,
    match_patterns JSONB NOT NULL DEFAULT '[]',
    enabled        BOOLEAN DEFAULT true,
    created_at     TIMESTAMPTZ DEFAULT now(),
    updated_at     TIMESTAMPTZ DEFAULT now()
);

-- Payloads (encrypted DLLs)
CREATE TABLE payloads (
    id            VARCHAR(30) PRIMARY KEY,
    version       VARCHAR(20) NOT NULL,
    encrypted_dll BYTEA NOT NULL,
    encrypted_key BYTEA NOT NULL,
    sha256        VARCHAR(64) NOT NULL,
    created_at    TIMESTAMPTZ DEFAULT now()
);

-- Invite codes
CREATE TABLE invite_codes (
    code       VARCHAR(20) PRIMARY KEY,
    created_at TIMESTAMPTZ DEFAULT now(),
    created_by VARCHAR(50) DEFAULT '',
    used_by    VARCHAR(50) DEFAULT '',
    used_at    TIMESTAMPTZ,
    max_uses   INT DEFAULT 1,
    use_count  INT DEFAULT 0
);

-- Client credentials (for HMAC auth)
CREATE TABLE client_credentials (
    client_id VARCHAR(50) PRIMARY KEY,
    secret    VARCHAR(128) NOT NULL
);

-- Indexes
CREATE INDEX idx_cards_code ON cards(code);
CREATE INDEX idx_cards_status ON cards(status);
CREATE INDEX idx_cards_agent_id ON cards(agent_id);
CREATE INDEX idx_cards_machine_fp ON cards(machine_fp);
CREATE INDEX idx_sessions_card_id ON sessions(card_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
CREATE INDEX idx_sessions_machine_fp ON sessions(machine_fp);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_machines_blacklisted ON machines(blacklisted);
```

- [ ] **Step 2: Write the down migration**

```sql
-- server/migrations/001_init.down.sql
DROP TABLE IF EXISTS invite_codes CASCADE;
DROP TABLE IF EXISTS payloads CASCADE;
DROP TABLE IF EXISTS platform_adapters CASCADE;
DROP TABLE IF EXISTS scripts CASCADE;
DROP TABLE IF EXISTS answer_cache CASCADE;
DROP TABLE IF EXISTS announcements CASCADE;
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS machines CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;
DROP TABLE IF EXISTS cards CASCADE;
DROP TABLE IF EXISTS client_credentials CASCADE;
DROP TABLE IF EXISTS users CASCADE;
```

- [ ] **Step 3: Start PostgreSQL and run migration**

```bash
cd server
docker compose up -d
sleep 3
DATABASE_URL="postgres://postgres:lingqiao_dev@localhost:5432/lingqiao?sslmode=disable" make migrate-up
```

- [ ] **Step 4: Verify tables exist**

```bash
docker compose exec postgres psql -U postgres -d lingqiao -c "\dt"
```

Expected: list of all 11 tables.

- [ ] **Step 5: Commit**

```bash
git add migrations/
git commit -m "feat: add initial database schema migration"
```

---

## Task 4: Repository Interfaces and PostgreSQL Connection

**Files:**
- Create: `server/internal/repository/interfaces.go`
- Create: `server/internal/repository/postgres/db.go`

- [ ] **Step 1: Define repository interfaces**

```go
// server/internal/repository/interfaces.go
package repository

import (
    "context"
    "time"

    "github.com/google/uuid"
    "github.com/lingqiao/server/internal/domain"
)

type CardRepository interface {
    Create(ctx context.Context, card *domain.Card) error
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Card, error)
    GetByCode(ctx context.Context, code string) (*domain.Card, error)
    Update(ctx context.Context, card *domain.Card) error
    List(ctx context.Context, filter CardFilter) ([]*domain.Card, error)
    Count(ctx context.Context, filter CardFilter) (int, error)
    Delete(ctx context.Context, id uuid.UUID) error
}

type CardFilter struct {
    Status    string
    AgentID   *uuid.UUID
    MachineFP string
    Search    string
    Page      int
    PerPage   int
}

type SessionRepository interface {
    Create(ctx context.Context, session *domain.Session) error
    GetByID(ctx context.Context, id uuid.UUID) (*domain.Session, error)
    GetByCardID(ctx context.Context, cardID uuid.UUID) ([]*domain.Session, error)
    Update(ctx context.Context, session *domain.Session) error
    Delete(ctx context.Context, id uuid.UUID) error
    DeleteExpired(ctx context.Context) (int64, error)
    List(ctx context.Context, page, perPage int) ([]*domain.Session, int, error)
    CountActive(ctx context.Context) (int, error)
}

type UserRepository interface {
    Create(ctx context.Context, user *domain.User) error
    GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
    GetByUsername(ctx context.Context, username string) (*domain.User, error)
    Update(ctx context.Context, user *domain.User) error
    List(ctx context.Context, role domain.UserRole) ([]*domain.User, error)
    Delete(ctx context.Context, id uuid.UUID) error
}

type MachineRepository interface {
    Upsert(ctx context.Context, machine *domain.Machine) error
    GetByFingerprint(ctx context.Context, fp string) (*domain.Machine, error)
    SetBlacklisted(ctx context.Context, fp string, blacklisted bool) error
    List(ctx context.Context, blacklistedOnly bool) ([]*domain.Machine, error)
}

type AuditRepository interface {
    Create(ctx context.Context, entry *domain.AuditEntry) error
    List(ctx context.Context, filter AuditFilter) ([]*domain.AuditEntry, int, error)
}

type AuditFilter struct {
    Action  string
    Page    int
    PerPage int
}

type AnnouncementRepository interface {
    Get(ctx context.Context) (*domain.Announcement, error)
    Update(ctx context.Context, ann *domain.Announcement) error
}

type AnswerCacheRepository interface {
    Get(ctx context.Context, hash string) (*domain.AnswerCache, error)
    Set(ctx context.Context, entry *domain.AnswerCache) error
    IncrementHit(ctx context.Context, hash string) error
}
```

Also add `AnswerCache` and `AuditEntry` to domain:

```go
// Add to server/internal/domain/ (new file or append to existing)

// server/internal/domain/audit.go
package domain

import "time"

type AuditEntry struct {
    ID         int64     `json:"id"`
    ActorID    string    `json:"actor_id"`
    Action     string    `json:"action"`
    TargetType string    `json:"target_type"`
    TargetID   string    `json:"target_id"`
    Detail     string    `json:"detail"`
    IPAddress  string    `json:"ip_address"`
    CreatedAt  time.Time `json:"created_at"`
}

// server/internal/domain/cache.go
package domain

import "time"

type AnswerCache struct {
    QuestionHash string    `json:"question_hash"`
    Platform     string    `json:"platform"`
    QuestionType string    `json:"question_type"`
    QuestionText string    `json:"question_text"`
    Answer       string    `json:"answer"`
    Model        string    `json:"model"`
    Confidence   float32   `json:"confidence"`
    HitCount     int       `json:"hit_count"`
    CreatedAt    time.Time `json:"created_at"`
    LastHitAt    *time.Time `json:"last_hit_at,omitempty"`
}
```

- [ ] **Step 2: Create PostgreSQL connection pool**

```go
// server/internal/repository/postgres/db.go
package postgres

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
    config, err := pgxpool.ParseConfig(databaseURL)
    if err != nil {
        return nil, fmt.Errorf("parse database url: %w", err)
    }
    config.MaxConns = 20
    config.MinConns = 2

    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, fmt.Errorf("create pool: %w", err)
    }
    if err := pool.Ping(ctx); err != nil {
        return nil, fmt.Errorf("ping database: %w", err)
    }
    return pool, nil
}
```

- [ ] **Step 3: Verify compilation**

```bash
cd server
go build ./internal/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/repository/ internal/domain/audit.go internal/domain/cache.go
git commit -m "feat: add repository interfaces and PostgreSQL connection pool"
```

---

## Task 5: PostgreSQL Card Repository

**Files:**
- Create: `server/internal/repository/postgres/card_repo.go`
- Create: `server/internal/repository/postgres/card_repo_test.go`

- [ ] **Step 1: Write the failing test**

```go
// server/internal/repository/postgres/card_repo_test.go
package postgres_test

import (
    "context"
    "os"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/lingqiao/server/internal/domain"
    "github.com/lingqiao/server/internal/repository/postgres"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func testPool(t *testing.T) *pgxpool.Pool {
    t.Helper()
    url := os.Getenv("DATABASE_URL")
    if url == "" {
        url = "postgres://postgres:lingqiao_dev@localhost:5432/lingqiao?sslmode=disable"
    }
    pool, err := postgres.NewPool(context.Background(), url)
    require.NoError(t, err)
    t.Cleanup(func() { pool.Close() })
    return pool
}

func TestCardRepo_CreateAndGetByCode(t *testing.T) {
    pool := testPool(t)
    repo := postgres.NewCardRepo(pool)
    ctx := context.Background()

    code, err := domain.GenerateCardCode()
    require.NoError(t, err)

    card := &domain.Card{
        ID:          uuid.New(),
        Code:        code,
        Duration:    24 * 30, // 30 days in hours
        Status:      domain.CardStatusUnused,
        MaxSessions: 1,
        CreatedAt:   time.Now(),
    }

    err = repo.Create(ctx, card)
    require.NoError(t, err)

    got, err := repo.GetByCode(ctx, code)
    require.NoError(t, err)
    assert.Equal(t, card.ID, got.ID)
    assert.Equal(t, card.Code, got.Code)
    assert.Equal(t, card.Status, got.Status)

    // cleanup
    _ = repo.Delete(ctx, card.ID)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd server
DATABASE_URL="postgres://postgres:lingqiao_dev@localhost:5432/lingqiao?sslmode=disable" \
  go test ./internal/repository/postgres/ -run TestCardRepo_CreateAndGetByCode -v
```

Expected: compilation failure — `NewCardRepo` not defined.

- [ ] **Step 3: Implement CardRepository**

```go
// server/internal/repository/postgres/card_repo.go
package postgres

import (
    "context"
    "fmt"
    "strings"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/lingqiao/server/internal/domain"
    "github.com/lingqiao/server/internal/repository"
)

type CardRepo struct {
    pool *pgxpool.Pool
}

func NewCardRepo(pool *pgxpool.Pool) *CardRepo {
    return &CardRepo{pool: pool}
}

func (r *CardRepo) Create(ctx context.Context, card *domain.Card) error {
    _, err := r.pool.Exec(ctx, `
        INSERT INTO cards (id, code, duration, status, agent_id, note, max_sessions, machine_fp, created_at, activated_at, expires_at)
        VALUES ($1, $2, make_interval(hours => $3), $4, $5, $6, $7, $8, $9, $10, $11)
    `, card.ID, card.Code, card.Duration, card.Status, card.AgentID,
        card.Note, card.MaxSessions, card.MachineFP, card.CreatedAt,
        card.ActivatedAt, card.ExpiresAt)
    if err != nil {
        if strings.Contains(err.Error(), "duplicate key") {
            return domain.ErrDuplicateCard
        }
        return fmt.Errorf("insert card: %w", err)
    }
    return nil
}

func (r *CardRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Card, error) {
    card := &domain.Card{}
    var durHours int
    err := r.pool.QueryRow(ctx, `
        SELECT id, code, EXTRACT(EPOCH FROM duration)::int / 3600, status, agent_id, note,
               max_sessions, machine_fp, created_at, activated_at, expires_at
        FROM cards WHERE id = $1
    `, id).Scan(
        &card.ID, &card.Code, &durHours, &card.Status, &card.AgentID,
        &card.Note, &card.MaxSessions, &card.MachineFP, &card.CreatedAt,
        &card.ActivatedAt, &card.ExpiresAt,
    )
    if err != nil {
        if err.Error() == "no rows in result set" {
            return nil, domain.ErrCardNotFound
        }
        return nil, fmt.Errorf("get card by id: %w", err)
    }
    card.Duration = durHours
    return card, nil
}

func (r *CardRepo) GetByCode(ctx context.Context, code string) (*domain.Card, error) {
    card := &domain.Card{}
    var durHours int
    err := r.pool.QueryRow(ctx, `
        SELECT id, code, EXTRACT(EPOCH FROM duration)::int / 3600, status, agent_id, note,
               max_sessions, machine_fp, created_at, activated_at, expires_at
        FROM cards WHERE code = $1
    `, code).Scan(
        &card.ID, &card.Code, &durHours, &card.Status, &card.AgentID,
        &card.Note, &card.MaxSessions, &card.MachineFP, &card.CreatedAt,
        &card.ActivatedAt, &card.ExpiresAt,
    )
    if err != nil {
        if err.Error() == "no rows in result set" {
            return nil, domain.ErrCardNotFound
        }
        return nil, fmt.Errorf("get card by code: %w", err)
    }
    card.Duration = durHours
    return card, nil
}

func (r *CardRepo) Update(ctx context.Context, card *domain.Card) error {
    tag, err := r.pool.Exec(ctx, `
        UPDATE cards SET status = $2, agent_id = $3, note = $4, max_sessions = $5,
               machine_fp = $6, activated_at = $7, expires_at = $8
        WHERE id = $1
    `, card.ID, card.Status, card.AgentID, card.Note, card.MaxSessions,
        card.MachineFP, card.ActivatedAt, card.ExpiresAt)
    if err != nil {
        return fmt.Errorf("update card: %w", err)
    }
    if tag.RowsAffected() == 0 {
        return domain.ErrCardNotFound
    }
    return nil
}

func (r *CardRepo) List(ctx context.Context, filter repository.CardFilter) ([]*domain.Card, error) {
    query := `SELECT id, code, EXTRACT(EPOCH FROM duration)::int / 3600, status, agent_id, note,
              max_sessions, machine_fp, created_at, activated_at, expires_at FROM cards WHERE 1=1`
    args := []interface{}{}
    argIdx := 1

    if filter.Status != "" {
        query += fmt.Sprintf(" AND status = $%d", argIdx)
        args = append(args, filter.Status)
        argIdx++
    }
    if filter.AgentID != nil {
        query += fmt.Sprintf(" AND agent_id = $%d", argIdx)
        args = append(args, *filter.AgentID)
        argIdx++
    }
    if filter.MachineFP != "" {
        query += fmt.Sprintf(" AND machine_fp = $%d", argIdx)
        args = append(args, filter.MachineFP)
        argIdx++
    }
    if filter.Search != "" {
        query += fmt.Sprintf(" AND code ILIKE $%d", argIdx)
        args = append(args, "%"+filter.Search+"%")
        argIdx++
    }

    query += " ORDER BY created_at DESC"

    if filter.PerPage > 0 {
        offset := (filter.Page - 1) * filter.PerPage
        query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
        args = append(args, filter.PerPage, offset)
    }

    rows, err := r.pool.Query(ctx, query, args...)
    if err != nil {
        return nil, fmt.Errorf("list cards: %w", err)
    }
    defer rows.Close()

    var cards []*domain.Card
    for rows.Next() {
        card := &domain.Card{}
        var durHours int
        if err := rows.Scan(&card.ID, &card.Code, &durHours, &card.Status, &card.AgentID,
            &card.Note, &card.MaxSessions, &card.MachineFP, &card.CreatedAt,
            &card.ActivatedAt, &card.ExpiresAt); err != nil {
            return nil, fmt.Errorf("scan card: %w", err)
        }
        card.Duration = durHours
        cards = append(cards, card)
    }
    return cards, nil
}

func (r *CardRepo) Count(ctx context.Context, filter repository.CardFilter) (int, error) {
    // Similar to List but SELECT COUNT(*)
    query := `SELECT COUNT(*) FROM cards WHERE 1=1`
    args := []interface{}{}
    argIdx := 1
    if filter.Status != "" {
        query += fmt.Sprintf(" AND status = $%d", argIdx)
        args = append(args, filter.Status)
        argIdx++
    }
    if filter.AgentID != nil {
        query += fmt.Sprintf(" AND agent_id = $%d", argIdx)
        args = append(args, *filter.AgentID)
        argIdx++
    }
    var count int
    err := r.pool.QueryRow(ctx, query, args...).Scan(&count)
    return count, err
}

func (r *CardRepo) Delete(ctx context.Context, id uuid.UUID) error {
    _, err := r.pool.Exec(ctx, `DELETE FROM cards WHERE id = $1`, id)
    return err
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd server
DATABASE_URL="postgres://postgres:lingqiao_dev@localhost:5432/lingqiao?sslmode=disable" \
  go test ./internal/repository/postgres/ -run TestCardRepo_CreateAndGetByCode -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/postgres/card_repo.go internal/repository/postgres/card_repo_test.go
git commit -m "feat: implement PostgreSQL card repository"
```

---

## Task 6: Remaining PostgreSQL Repositories

**Files:**
- Create: `server/internal/repository/postgres/session_repo.go`
- Create: `server/internal/repository/postgres/user_repo.go`
- Create: `server/internal/repository/postgres/machine_repo.go`
- Create: `server/internal/repository/postgres/audit_repo.go`
- Create: `server/internal/repository/postgres/cache_repo.go`

Each follows the same pattern as CardRepo. Implement with tests.

- [ ] **Step 1: Implement SessionRepo**

```go
// server/internal/repository/postgres/session_repo.go
package postgres

import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/lingqiao/server/internal/domain"
)

type SessionRepo struct {
    pool *pgxpool.Pool
}

func NewSessionRepo(pool *pgxpool.Pool) *SessionRepo {
    return &SessionRepo{pool: pool}
}

func (r *SessionRepo) Create(ctx context.Context, s *domain.Session) error {
    _, err := r.pool.Exec(ctx, `
        INSERT INTO sessions (id, card_id, card_code, machine_fp, client_version, ip_address, created_at, last_seen_at, expires_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
    `, s.ID, s.CardID, s.CardCode, s.MachineFP, s.ClientVersion, s.IPAddress, s.CreatedAt, s.LastSeenAt, s.ExpiresAt)
    return err
}

func (r *SessionRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Session, error) {
    s := &domain.Session{}
    err := r.pool.QueryRow(ctx, `
        SELECT id, card_id, card_code, machine_fp, client_version, ip_address, created_at, last_seen_at, expires_at
        FROM sessions WHERE id = $1
    `, id).Scan(&s.ID, &s.CardID, &s.CardCode, &s.MachineFP, &s.ClientVersion, &s.IPAddress, &s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt)
    if err != nil {
        if err.Error() == "no rows in result set" {
            return nil, domain.ErrSessionNotFound
        }
        return nil, err
    }
    return s, nil
}

func (r *SessionRepo) GetByCardID(ctx context.Context, cardID uuid.UUID) ([]*domain.Session, error) {
    rows, err := r.pool.Query(ctx, `
        SELECT id, card_id, card_code, machine_fp, client_version, ip_address, created_at, last_seen_at, expires_at
        FROM sessions WHERE card_id = $1 ORDER BY created_at DESC
    `, cardID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var sessions []*domain.Session
    for rows.Next() {
        s := &domain.Session{}
        if err := rows.Scan(&s.ID, &s.CardID, &s.CardCode, &s.MachineFP, &s.ClientVersion, &s.IPAddress, &s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt); err != nil {
            return nil, err
        }
        sessions = append(sessions, s)
    }
    return sessions, nil
}

func (r *SessionRepo) Update(ctx context.Context, s *domain.Session) error {
    _, err := r.pool.Exec(ctx, `
        UPDATE sessions SET last_seen_at = $2, expires_at = $3, ip_address = $4, client_version = $5
        WHERE id = $1
    `, s.ID, s.LastSeenAt, s.ExpiresAt, s.IPAddress, s.ClientVersion)
    return err
}

func (r *SessionRepo) Delete(ctx context.Context, id uuid.UUID) error {
    _, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, id)
    return err
}

func (r *SessionRepo) DeleteExpired(ctx context.Context) (int64, error) {
    tag, err := r.pool.Exec(ctx, `DELETE FROM sessions WHERE expires_at < now()`)
    if err != nil {
        return 0, err
    }
    return tag.RowsAffected(), nil
}

func (r *SessionRepo) List(ctx context.Context, page, perPage int) ([]*domain.Session, int, error) {
    var total int
    err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM sessions`).Scan(&total)
    if err != nil {
        return nil, 0, err
    }

    offset := (page - 1) * perPage
    rows, err := r.pool.Query(ctx, `
        SELECT id, card_id, card_code, machine_fp, client_version, ip_address, created_at, last_seen_at, expires_at
        FROM sessions ORDER BY last_seen_at DESC LIMIT $1 OFFSET $2
    `, perPage, offset)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()
    var sessions []*domain.Session
    for rows.Next() {
        s := &domain.Session{}
        if err := rows.Scan(&s.ID, &s.CardID, &s.CardCode, &s.MachineFP, &s.ClientVersion, &s.IPAddress, &s.CreatedAt, &s.LastSeenAt, &s.ExpiresAt); err != nil {
            return nil, 0, err
        }
        sessions = append(sessions, s)
    }
    return sessions, total, nil
}

func (r *SessionRepo) CountActive(ctx context.Context) (int, error) {
    var count int
    err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM sessions WHERE expires_at > now()`).Scan(&count)
    return count, err
}
```

- [ ] **Step 2: Implement UserRepo**

```go
// server/internal/repository/postgres/user_repo.go
package postgres

import (
    "context"
    "fmt"
    "strings"

    "github.com/google/uuid"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/lingqiao/server/internal/domain"
)

type UserRepo struct {
    pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
    return &UserRepo{pool: pool}
}

func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
    _, err := r.pool.Exec(ctx, `
        INSERT INTO users (id, username, password, role, prefix, disabled, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
    `, u.ID, u.Username, u.Password, u.Role, u.Prefix, u.Disabled, u.CreatedAt)
    if err != nil && strings.Contains(err.Error(), "duplicate key") {
        return domain.ErrDuplicateUser
    }
    return err
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
    u := &domain.User{}
    err := r.pool.QueryRow(ctx, `SELECT id, username, password, role, prefix, disabled, created_at FROM users WHERE id = $1`, id).
        Scan(&u.ID, &u.Username, &u.Password, &u.Role, &u.Prefix, &u.Disabled, &u.CreatedAt)
    if err != nil {
        if err.Error() == "no rows in result set" {
            return nil, domain.ErrUserNotFound
        }
        return nil, err
    }
    return u, nil
}

func (r *UserRepo) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
    u := &domain.User{}
    err := r.pool.QueryRow(ctx, `SELECT id, username, password, role, prefix, disabled, created_at FROM users WHERE username = $1`, username).
        Scan(&u.ID, &u.Username, &u.Password, &u.Role, &u.Prefix, &u.Disabled, &u.CreatedAt)
    if err != nil {
        if err.Error() == "no rows in result set" {
            return nil, domain.ErrUserNotFound
        }
        return nil, err
    }
    return u, nil
}

func (r *UserRepo) Update(ctx context.Context, u *domain.User) error {
    _, err := r.pool.Exec(ctx, `UPDATE users SET username=$2, password=$3, role=$4, prefix=$5, disabled=$6 WHERE id=$1`,
        u.ID, u.Username, u.Password, u.Role, u.Prefix, u.Disabled)
    return err
}

func (r *UserRepo) List(ctx context.Context, role domain.UserRole) ([]*domain.User, error) {
    query := `SELECT id, username, password, role, prefix, disabled, created_at FROM users`
    args := []interface{}{}
    if role != "" {
        query += " WHERE role = $1"
        args = append(args, role)
    }
    query += " ORDER BY created_at DESC"
    rows, err := r.pool.Query(ctx, query, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var users []*domain.User
    for rows.Next() {
        u := &domain.User{}
        if err := rows.Scan(&u.ID, &u.Username, &u.Password, &u.Role, &u.Prefix, &u.Disabled, &u.CreatedAt); err != nil {
            return nil, err
        }
        users = append(users, u)
    }
    return users, nil
}

func (r *UserRepo) Delete(ctx context.Context, id uuid.UUID) error {
    _, err := r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
    return err
}
```

- [ ] **Step 3: Implement MachineRepo**

```go
// server/internal/repository/postgres/machine_repo.go
package postgres

import (
    "context"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/lingqiao/server/internal/domain"
)

type MachineRepo struct {
    pool *pgxpool.Pool
}

func NewMachineRepo(pool *pgxpool.Pool) *MachineRepo {
    return &MachineRepo{pool: pool}
}

func (r *MachineRepo) Upsert(ctx context.Context, m *domain.Machine) error {
    _, err := r.pool.Exec(ctx, `
        INSERT INTO machines (fingerprint, ip_address, blacklisted, note, first_seen, last_seen)
        VALUES ($1, $2, $3, $4, $5, $6)
        ON CONFLICT (fingerprint) DO UPDATE SET
            ip_address = EXCLUDED.ip_address,
            last_seen = EXCLUDED.last_seen
    `, m.Fingerprint, m.IPAddress, m.Blacklisted, m.Note, m.FirstSeen, m.LastSeen)
    return err
}

func (r *MachineRepo) GetByFingerprint(ctx context.Context, fp string) (*domain.Machine, error) {
    m := &domain.Machine{}
    err := r.pool.QueryRow(ctx, `SELECT fingerprint, ip_address, blacklisted, note, first_seen, last_seen FROM machines WHERE fingerprint = $1`, fp).
        Scan(&m.Fingerprint, &m.IPAddress, &m.Blacklisted, &m.Note, &m.FirstSeen, &m.LastSeen)
    if err != nil {
        if err.Error() == "no rows in result set" {
            return nil, nil
        }
        return nil, err
    }
    return m, nil
}

func (r *MachineRepo) SetBlacklisted(ctx context.Context, fp string, blacklisted bool) error {
    _, err := r.pool.Exec(ctx, `UPDATE machines SET blacklisted = $2 WHERE fingerprint = $1`, fp, blacklisted)
    return err
}

func (r *MachineRepo) List(ctx context.Context, blacklistedOnly bool) ([]*domain.Machine, error) {
    query := `SELECT m.fingerprint, m.ip_address, m.blacklisted, m.note, m.first_seen, m.last_seen,
              (SELECT COUNT(*) FROM cards WHERE machine_fp = m.fingerprint) as card_count
              FROM machines m`
    if blacklistedOnly {
        query += " WHERE m.blacklisted = true"
    }
    query += " ORDER BY card_count DESC"

    rows, err := r.pool.Query(ctx, query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var machines []*domain.Machine
    for rows.Next() {
        m := &domain.Machine{}
        if err := rows.Scan(&m.Fingerprint, &m.IPAddress, &m.Blacklisted, &m.Note, &m.FirstSeen, &m.LastSeen, &m.CardCount); err != nil {
            return nil, err
        }
        machines = append(machines, m)
    }
    return machines, nil
}
```

- [ ] **Step 4: Implement AuditRepo**

```go
// server/internal/repository/postgres/audit_repo.go
package postgres

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/lingqiao/server/internal/domain"
    "github.com/lingqiao/server/internal/repository"
)

type AuditRepo struct {
    pool *pgxpool.Pool
}

func NewAuditRepo(pool *pgxpool.Pool) *AuditRepo {
    return &AuditRepo{pool: pool}
}

func (r *AuditRepo) Create(ctx context.Context, e *domain.AuditEntry) error {
    _, err := r.pool.Exec(ctx, `
        INSERT INTO audit_logs (actor_id, action, target_type, target_id, detail, ip_address)
        VALUES ($1, $2, $3, $4, $5, $6)
    `, e.ActorID, e.Action, e.TargetType, e.TargetID, e.Detail, e.IPAddress)
    return err
}

func (r *AuditRepo) List(ctx context.Context, filter repository.AuditFilter) ([]*domain.AuditEntry, int, error) {
    countQuery := `SELECT COUNT(*) FROM audit_logs WHERE 1=1`
    query := `SELECT id, actor_id, action, target_type, target_id, detail, ip_address, created_at FROM audit_logs WHERE 1=1`
    args := []interface{}{}
    argIdx := 1

    if filter.Action != "" {
        clause := fmt.Sprintf(" AND action = $%d", argIdx)
        countQuery += clause
        query += clause
        args = append(args, filter.Action)
        argIdx++
    }

    var total int
    err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
    if err != nil {
        return nil, 0, err
    }

    offset := (filter.Page - 1) * filter.PerPage
    query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
    args = append(args, filter.PerPage, offset)

    rows, err := r.pool.Query(ctx, query, args...)
    if err != nil {
        return nil, 0, err
    }
    defer rows.Close()

    var entries []*domain.AuditEntry
    for rows.Next() {
        e := &domain.AuditEntry{}
        if err := rows.Scan(&e.ID, &e.ActorID, &e.Action, &e.TargetType, &e.TargetID, &e.Detail, &e.IPAddress, &e.CreatedAt); err != nil {
            return nil, 0, err
        }
        entries = append(entries, e)
    }
    return entries, total, nil
}
```

- [ ] **Step 5: Implement AnswerCacheRepo**

```go
// server/internal/repository/postgres/cache_repo.go
package postgres

import (
    "context"
    "time"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/lingqiao/server/internal/domain"
)

type CacheRepo struct {
    pool *pgxpool.Pool
}

func NewCacheRepo(pool *pgxpool.Pool) *CacheRepo {
    return &CacheRepo{pool: pool}
}

func (r *CacheRepo) Get(ctx context.Context, hash string) (*domain.AnswerCache, error) {
    c := &domain.AnswerCache{}
    err := r.pool.QueryRow(ctx, `
        SELECT question_hash, platform, question_type, question_text, answer, model, confidence, hit_count, created_at, last_hit_at
        FROM answer_cache WHERE question_hash = $1
    `, hash).Scan(&c.QuestionHash, &c.Platform, &c.QuestionType, &c.QuestionText, &c.Answer, &c.Model, &c.Confidence, &c.HitCount, &c.CreatedAt, &c.LastHitAt)
    if err != nil {
        if err.Error() == "no rows in result set" {
            return nil, nil
        }
        return nil, err
    }
    return c, nil
}

func (r *CacheRepo) Set(ctx context.Context, c *domain.AnswerCache) error {
    _, err := r.pool.Exec(ctx, `
        INSERT INTO answer_cache (question_hash, platform, question_type, question_text, answer, model, confidence, hit_count, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        ON CONFLICT (question_hash) DO UPDATE SET
            answer = EXCLUDED.answer, model = EXCLUDED.model, confidence = EXCLUDED.confidence,
            hit_count = answer_cache.hit_count + 1, last_hit_at = now()
    `, c.QuestionHash, c.Platform, c.QuestionType, c.QuestionText, c.Answer, c.Model, c.Confidence, c.HitCount, c.CreatedAt)
    return err
}

func (r *CacheRepo) IncrementHit(ctx context.Context, hash string) error {
    _, err := r.pool.Exec(ctx, `UPDATE answer_cache SET hit_count = hit_count + 1, last_hit_at = $2 WHERE question_hash = $1`, hash, time.Now())
    return err
}
```

- [ ] **Step 6: Verify all repos compile**

```bash
cd server
go build ./internal/repository/...
```

- [ ] **Step 7: Commit**

```bash
git add internal/repository/postgres/
git commit -m "feat: implement remaining PostgreSQL repositories (session, user, machine, audit, cache)"
```

---

## Task 7: Configuration Management

**Files:**
- Create: `server/internal/config/config.go`
- Create: `server/internal/config/config_test.go`

- [ ] **Step 1: Implement config loading**

```go
// server/internal/config/config.go
package config

import (
    "fmt"
    "os"
    "strconv"
)

type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    Auth     AuthConfig
    TLS      TLSConfig
    AI       AIConfig
}

type ServerConfig struct {
    AdminPort int
    AgentPort int
}

type DatabaseConfig struct {
    URL string
}

type AuthConfig struct {
    AdminPassword string
    HMACSecret    string
    JWTSecret     string
    UploadKey     string
}

type TLSConfig struct {
    Cert string
    Key  string
}

type AIConfig struct {
    DeepSeekKey string
    OpenAIKey   string
}

func Load() (*Config, error) {
    cfg := &Config{
        Server: ServerConfig{
            AdminPort: envInt("SERVER_ADMIN_PORT", 48901),
            AgentPort: envInt("SERVER_AGENT_PORT", 38472),
        },
        Database: DatabaseConfig{
            URL: envStr("DATABASE_URL", "postgres://postgres:lingqiao_dev@localhost:5432/lingqiao?sslmode=disable"),
        },
        Auth: AuthConfig{
            AdminPassword: envStr("ADMIN_PASSWORD", ""),
            HMACSecret:    envStr("HMAC_SECRET", ""),
            JWTSecret:     envStr("JWT_SECRET", "change-me-in-production"),
            UploadKey:     envStr("UPLOAD_KEY", ""),
        },
        TLS: TLSConfig{
            Cert: envStr("TLS_CERT", "./certs/server.crt"),
            Key:  envStr("TLS_KEY", "./certs/server.key"),
        },
        AI: AIConfig{
            DeepSeekKey: envStr("DEEPSEEK_API_KEY", ""),
            OpenAIKey:   envStr("OPENAI_API_KEY", ""),
        },
    }
    if cfg.Database.URL == "" {
        return nil, fmt.Errorf("DATABASE_URL is required")
    }
    return cfg, nil
}

func envStr(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}

func envInt(key string, fallback int) int {
    if v := os.Getenv(key); v != "" {
        if n, err := strconv.Atoi(v); err == nil {
            return n
        }
    }
    return fallback
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd server
go build ./internal/config/
```

- [ ] **Step 3: Commit**

```bash
git add internal/config/
git commit -m "feat: add environment-based configuration management"
```

---

## Task 8: Service Layer — Card Service

**Files:**
- Create: `server/internal/service/card_service.go`
- Create: `server/internal/service/card_service_test.go`

- [ ] **Step 1: Write the failing test**

```go
// server/internal/service/card_service_test.go
package service_test

import (
    "context"
    "testing"
    "time"

    "github.com/google/uuid"
    "github.com/lingqiao/server/internal/domain"
    "github.com/lingqiao/server/internal/repository/memory"
    "github.com/lingqiao/server/internal/service"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestCardService_GenerateAndActivate(t *testing.T) {
    cardRepo := memory.NewCardRepo()
    sessionRepo := memory.NewSessionRepo()
    machineRepo := memory.NewMachineRepo()
    auditRepo := memory.NewAuditRepo()
    svc := service.NewCardService(cardRepo, sessionRepo, machineRepo, auditRepo)

    ctx := context.Background()

    // Generate a card
    card, err := svc.Generate(ctx, 24*30, "test card", 1, nil)
    require.NoError(t, err)
    assert.NotEmpty(t, card.Code)
    assert.Equal(t, domain.CardStatusUnused, card.Status)

    // Activate the card
    session, err := svc.Activate(ctx, card.Code, "machine-fp-123", "fingerprint-abc", "1.2.3.4", "v2.1.13")
    require.NoError(t, err)
    assert.NotEmpty(t, session.ID)
    assert.Equal(t, card.ID, session.CardID)

    // Verify card is now active
    updatedCard, err := cardRepo.GetByID(ctx, card.ID)
    require.NoError(t, err)
    assert.Equal(t, domain.CardStatusActive, updatedCard.Status)
}
```

- [ ] **Step 2: Create in-memory repository implementations for testing**

```go
// server/internal/repository/memory/repos.go
package memory

import (
    "context"
    "sync"
    "time"

    "github.com/google/uuid"
    "github.com/lingqiao/server/internal/domain"
    "github.com/lingqiao/server/internal/repository"
)

// CardRepo
type CardRepo struct {
    mu    sync.RWMutex
    cards map[uuid.UUID]*domain.Card
    byCode map[string]*domain.Card
}

func NewCardRepo() *CardRepo {
    return &CardRepo{cards: make(map[uuid.UUID]*domain.Card), byCode: make(map[string]*domain.Card)}
}

func (r *CardRepo) Create(_ context.Context, card *domain.Card) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    if _, exists := r.byCode[card.Code]; exists {
        return domain.ErrDuplicateCard
    }
    r.cards[card.ID] = card
    r.byCode[card.Code] = card
    return nil
}

func (r *CardRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Card, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    if c, ok := r.cards[id]; ok {
        return c, nil
    }
    return nil, domain.ErrCardNotFound
}

func (r *CardRepo) GetByCode(_ context.Context, code string) (*domain.Card, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    if c, ok := r.byCode[code]; ok {
        return c, nil
    }
    return nil, domain.ErrCardNotFound
}

func (r *CardRepo) Update(_ context.Context, card *domain.Card) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.cards[card.ID] = card
    r.byCode[card.Code] = card
    return nil
}

func (r *CardRepo) List(_ context.Context, _ repository.CardFilter) ([]*domain.Card, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    var out []*domain.Card
    for _, c := range r.cards {
        out = append(out, c)
    }
    return out, nil
}

func (r *CardRepo) Count(_ context.Context, _ repository.CardFilter) (int, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return len(r.cards), nil
}

func (r *CardRepo) Delete(_ context.Context, id uuid.UUID) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    if c, ok := r.cards[id]; ok {
        delete(r.byCode, c.Code)
        delete(r.cards, id)
    }
    return nil
}

// SessionRepo
type SessionRepo struct {
    mu       sync.RWMutex
    sessions map[uuid.UUID]*domain.Session
}

func NewSessionRepo() *SessionRepo {
    return &SessionRepo{sessions: make(map[uuid.UUID]*domain.Session)}
}

func (r *SessionRepo) Create(_ context.Context, s *domain.Session) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.sessions[s.ID] = s
    return nil
}

func (r *SessionRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Session, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    if s, ok := r.sessions[id]; ok {
        return s, nil
    }
    return nil, domain.ErrSessionNotFound
}

func (r *SessionRepo) GetByCardID(_ context.Context, cardID uuid.UUID) ([]*domain.Session, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    var out []*domain.Session
    for _, s := range r.sessions {
        if s.CardID == cardID {
            out = append(out, s)
        }
    }
    return out, nil
}

func (r *SessionRepo) Update(_ context.Context, s *domain.Session) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.sessions[s.ID] = s
    return nil
}

func (r *SessionRepo) Delete(_ context.Context, id uuid.UUID) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    delete(r.sessions, id)
    return nil
}

func (r *SessionRepo) DeleteExpired(_ context.Context) (int64, error) {
    r.mu.Lock()
    defer r.mu.Unlock()
    now := time.Now()
    var count int64
    for id, s := range r.sessions {
        if now.After(s.ExpiresAt) {
            delete(r.sessions, id)
            count++
        }
    }
    return count, nil
}

func (r *SessionRepo) List(_ context.Context, page, perPage int) ([]*domain.Session, int, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    var all []*domain.Session
    for _, s := range r.sessions {
        all = append(all, s)
    }
    start := (page - 1) * perPage
    if start >= len(all) {
        return nil, len(all), nil
    }
    end := start + perPage
    if end > len(all) {
        end = len(all)
    }
    return all[start:end], len(all), nil
}

func (r *SessionRepo) CountActive(_ context.Context) (int, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    now := time.Now()
    count := 0
    for _, s := range r.sessions {
        if now.Before(s.ExpiresAt) {
            count++
        }
    }
    return count, nil
}

// MachineRepo
type MachineRepo struct {
    mu        sync.RWMutex
    machines  map[string]*domain.Machine
}

func NewMachineRepo() *MachineRepo {
    return &MachineRepo{machines: make(map[string]*domain.Machine)}
}

func (r *MachineRepo) Upsert(_ context.Context, m *domain.Machine) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.machines[m.Fingerprint] = m
    return nil
}

func (r *MachineRepo) GetByFingerprint(_ context.Context, fp string) (*domain.Machine, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    return r.machines[fp], nil
}

func (r *MachineRepo) SetBlacklisted(_ context.Context, fp string, blacklisted bool) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    if m, ok := r.machines[fp]; ok {
        m.Blacklisted = blacklisted
    }
    return nil
}

func (r *MachineRepo) List(_ context.Context, blacklistedOnly bool) ([]*domain.Machine, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    var out []*domain.Machine
    for _, m := range r.machines {
        if !blacklistedOnly || m.Blacklisted {
            out = append(out, m)
        }
    }
    return out, nil
}

// AuditRepo
type AuditRepo struct {
    mu      sync.RWMutex
    entries []*domain.AuditEntry
}

func NewAuditRepo() *AuditRepo {
    return &AuditRepo{}
}

func (r *AuditRepo) Create(_ context.Context, e *domain.AuditEntry) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.entries = append(r.entries, e)
    return nil
}

func (r *AuditRepo) List(_ context.Context, filter repository.AuditFilter) ([]*domain.AuditEntry, int, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    var out []*domain.AuditEntry
    for _, e := range r.entries {
        if filter.Action == "" || e.Action == filter.Action {
            out = append(out, e)
        }
    }
    start := (filter.Page - 1) * filter.PerPage
    if start >= len(out) {
        return nil, len(out), nil
    }
    end := start + filter.PerPage
    if end > len(out) {
        end = len(out)
    }
    return out[start:end], len(out), nil
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
cd server
go test ./internal/service/ -run TestCardService_GenerateAndActivate -v
```

Expected: compilation failure — `NewCardService` not defined.

- [ ] **Step 4: Implement CardService**

```go
// server/internal/service/card_service.go
package service

import (
    "context"
    "fmt"
    "time"

    "github.com/google/uuid"
    "github.com/lingqiao/server/internal/domain"
    "github.com/lingqiao/server/internal/repository"
)

type CardService struct {
    cards    repository.CardRepository
    sessions repository.SessionRepository
    machines repository.MachineRepository
    audit    repository.AuditRepository
}

func NewCardService(
    cards repository.CardRepository,
    sessions repository.SessionRepository,
    machines repository.MachineRepository,
    audit repository.AuditRepository,
) *CardService {
    return &CardService{
        cards:    cards,
        sessions: sessions,
        machines: machines,
        audit:    audit,
    }
}

func (s *CardService) Generate(ctx context.Context, durationHours int, note string, maxSessions int, agentID *uuid.UUID) (*domain.Card, error) {
    if durationHours <= 0 {
        durationHours = 24 * 30
    }
    if maxSessions <= 0 {
        maxSessions = 1
    }

    code, err := domain.GenerateCardCode()
    if err != nil {
        return nil, fmt.Errorf("generate card code: %w", err)
    }

    card := &domain.Card{
        ID:          uuid.New(),
        Code:        code,
        Duration:    durationHours,
        Status:      domain.CardStatusUnused,
        AgentID:     agentID,
        Note:        note,
        MaxSessions: maxSessions,
        CreatedAt:   time.Now(),
    }

    if err := s.cards.Create(ctx, card); err != nil {
        return nil, err
    }

    _ = s.audit.Create(ctx, &domain.AuditEntry{
        Action:    "card_generated",
        TargetID:  card.Code,
        CreatedAt: time.Now(),
    })

    return card, nil
}

func (s *CardService) Activate(ctx context.Context, code, machineFP, fingerprint, ip, clientVersion string) (*domain.Session, error) {
    normalizedCode := domain.NormalizeCardCode(code)

    card, err := s.cards.GetByCode(ctx, normalizedCode)
    if err != nil {
        return nil, err
    }

    if err := card.CanActivate(); err != nil {
        return nil, err
    }

    // Check machine blacklist
    machine, _ := s.machines.GetByFingerprint(ctx, machineFP)
    if machine != nil && machine.Blacklisted {
        return nil, domain.ErrMachineBlacklisted
    }

    // Upsert machine
    now := time.Now()
    _ = s.machines.Upsert(ctx, &domain.Machine{
        Fingerprint: machineFP,
        IPAddress:   ip,
        FirstSeen:   now,
        LastSeen:    now,
    })

    // Activate card if unused
    if card.Status == domain.CardStatusUnused {
        expiresAt := now.Add(time.Duration(card.Duration) * time.Hour)
        card.Status = domain.CardStatusActive
        card.ActivatedAt = &now
        card.ExpiresAt = &expiresAt
        card.MachineFP = machineFP
        if err := s.cards.Update(ctx, card); err != nil {
            return nil, err
        }
    }

    // Create session
    session := &domain.Session{
        ID:            uuid.New(),
        CardID:        card.ID,
        CardCode:      card.Code,
        MachineFP:     machineFP,
        ClientVersion: clientVersion,
        IPAddress:     ip,
        CreatedAt:     now,
        LastSeenAt:    now,
        ExpiresAt:     now.Add(24 * time.Hour),
    }

    if err := s.sessions.Create(ctx, session); err != nil {
        return nil, err
    }

    _ = s.audit.Create(ctx, &domain.AuditEntry{
        Action:    "card_activated",
        TargetID:  card.Code,
        Detail:    machineFP,
        IPAddress: ip,
        CreatedAt: now,
    })

    return session, nil
}

func (s *CardService) Heartbeat(ctx context.Context, sessionToken uuid.UUID, machineFP, ip, clientVersion string) (*domain.Session, error) {
    session, err := s.sessions.GetByID(ctx, sessionToken)
    if err != nil {
        return nil, err
    }
    if session.IsExpired() {
        return nil, domain.ErrSessionExpired
    }

    now := time.Now()
    session.LastSeenAt = now
    session.ExpiresAt = now.Add(24 * time.Hour)
    session.IPAddress = ip
    session.ClientVersion = clientVersion

    if err := s.sessions.Update(ctx, session); err != nil {
        return nil, err
    }
    return session, nil
}

func (s *CardService) Deactivate(ctx context.Context, sessionToken uuid.UUID) error {
    return s.sessions.Delete(ctx, sessionToken)
}

func (s *CardService) List(ctx context.Context, filter repository.CardFilter) ([]*domain.Card, int, error) {
    cards, err := s.cards.List(ctx, filter)
    if err != nil {
        return nil, 0, err
    }
    total, err := s.cards.Count(ctx, filter)
    if err != nil {
        return nil, 0, err
    }
    return cards, total, nil
}

func (s *CardService) UpdateStatus(ctx context.Context, code string, status domain.CardStatus) error {
    card, err := s.cards.GetByCode(ctx, domain.NormalizeCardCode(code))
    if err != nil {
        return err
    }
    card.Status = status
    return s.cards.Update(ctx, card)
}

func (s *CardService) Extend(ctx context.Context, code string, hours int) error {
    card, err := s.cards.GetByCode(ctx, domain.NormalizeCardCode(code))
    if err != nil {
        return err
    }
    if card.ExpiresAt != nil {
        extended := card.ExpiresAt.Add(time.Duration(hours) * time.Hour)
        card.ExpiresAt = &extended
    }
    return s.cards.Update(ctx, card)
}
```

- [ ] **Step 5: Run test to verify it passes**

```bash
cd server
go test ./internal/service/ -run TestCardService_GenerateAndActivate -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/service/card_service.go internal/service/card_service_test.go internal/repository/memory/
git commit -m "feat: implement card service with in-memory test repositories"
```

---

## Task 9: Auth Service

**Files:**
- Create: `server/internal/service/auth_service.go`

- [ ] **Step 1: Implement HMAC verification and JWT token service**

```go
// server/internal/service/auth_service.go
package service

import (
    "context"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "strconv"
    "strings"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"
    "github.com/lingqiao/server/internal/domain"
    "github.com/lingqiao/server/internal/repository"
)

type AuthService struct {
    users      repository.UserRepository
    jwtSecret  []byte
    nonces     *nonceTracker
}

type nonceTracker struct {
    nonces map[string]time.Time
}

func NewAuthService(users repository.UserRepository, jwtSecret string) *AuthService {
    return &AuthService{
        users:     users,
        jwtSecret: []byte(jwtSecret),
        nonces: &nonceTracker{
            nonces: make(map[string]time.Time),
        },
    }
}

// VerifyHMAC validates client API requests with timestamp+nonce anti-replay
func (s *AuthService) VerifyHMAC(clientSecret, body, signature, timestamp, nonce string) error {
    // Parse timestamp
    ts, err := strconv.ParseInt(timestamp, 10, 64)
    if err != nil {
        return fmt.Errorf("invalid timestamp")
    }

    // Check timestamp within 30 seconds
    now := time.Now().Unix()
    if abs(now-ts) > 30 {
        return fmt.Errorf("request expired")
    }

    // Check nonce uniqueness
    s.nonces.cleanup()
    nonceKey := timestamp + ":" + nonce
    if _, exists := s.nonces.nonces[nonceKey]; exists {
        return fmt.Errorf("nonce already used")
    }
    s.nonces.nonces[nonceKey] = time.Now()

    // Verify HMAC: HMAC-SHA256(clientSecret, "timestamp|nonce|body")
    message := timestamp + "|" + nonce + "|" + body
    mac := hmac.New(sha256.New, []byte(clientSecret))
    mac.Write([]byte(message))
    expected := hex.EncodeToString(mac.Sum(nil))

    if !hmac.Equal([]byte(signature), []byte(expected)) {
        return fmt.Errorf("invalid signature")
    }
    return nil
}

// GenerateJWT creates a JWT token for admin/agent sessions
func (s *AuthService) GenerateJWT(userID uuid.UUID, role domain.UserRole, duration time.Duration) (string, error) {
    claims := jwt.MapClaims{
        "sub":  userID.String(),
        "role": string(role),
        "exp":  time.Now().Add(duration).Unix(),
        "iat":  time.Now().Unix(),
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(s.jwtSecret)
}

// ValidateJWT parses and validates a JWT token
func (s *AuthService) ValidateJWT(tokenString string) (uuid.UUID, domain.UserRole, error) {
    token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
        if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method")
        }
        return s.jwtSecret, nil
    })
    if err != nil {
        return uuid.Nil, "", err
    }

    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok || !token.Valid {
        return uuid.Nil, "", fmt.Errorf("invalid token")
    }

    sub, _ := claims.GetSubject()
    userID, err := uuid.Parse(sub)
    if err != nil {
        return uuid.Nil, "", err
    }

    role, _ := claims["role"].(string)
    return userID, domain.UserRole(role), nil
}

func (s *AuthService) AuthenticateUser(ctx context.Context, username, passwordHash string) (*domain.User, error) {
    user, err := s.users.GetByUsername(ctx, username)
    if err != nil {
        return nil, domain.ErrInvalidCredentials
    }
    if user.Disabled {
        return nil, domain.ErrInvalidCredentials
    }
    // Password comparison happens at handler level (constant-time)
    return user, nil
}

func (nt *nonceTracker) cleanup() {
    cutoff := time.Now().Add(-2 * time.Minute)
    for k, t := range nt.nonces {
        if t.Before(cutoff) {
            delete(nt.nonces, k)
        }
    }
}

func abs(x int64) int64 {
    if x < 0 {
        return -x
    }
    return x
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd server
go build ./internal/service/
```

- [ ] **Step 3: Commit**

```bash
git add internal/service/auth_service.go
git commit -m "feat: implement auth service with HMAC verification and JWT"
```

---

## Task 10: HTTP Handlers

**Files:**
- Create: `server/internal/handler/response.go`
- Create: `server/internal/handler/client.go`
- Create: `server/internal/handler/admin.go`
- Create: `server/internal/handler/agent.go`

- [ ] **Step 1: Create unified response helpers**

```go
// server/internal/handler/response.go
package handler

import (
    "encoding/json"
    "net/http"
)

type Response struct {
    Status  string      `json:"status"`
    Message string      `json:"message,omitempty"`
    Data    interface{} `json:"data,omitempty"`
}

func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(v)
}

func WriteOK(w http.ResponseWriter, data interface{}) {
    WriteJSON(w, http.StatusOK, Response{Status: "ok", Data: data})
}

func WriteError(w http.ResponseWriter, status int, message string) {
    WriteJSON(w, status, Response{Status: "error", Message: message})
}
```

- [ ] **Step 2: Implement client API handlers**

```go
// server/internal/handler/client.go
package handler

import (
    "encoding/json"
    "net/http"

    "github.com/google/uuid"
    "github.com/lingqiao/server/internal/service"
)

type ClientHandler struct {
    cardSvc *service.CardService
    authSvc *service.AuthService
}

func NewClientHandler(cardSvc *service.CardService, authSvc *service.AuthService) *ClientHandler {
    return &ClientHandler{cardSvc: cardSvc, authSvc: authSvc}
}

type activateRequest struct {
    ClientID      string `json:"client_id"`
    Card          string `json:"card"`
    MachineID     string `json:"machine_id"`
    Fingerprint   string `json:"fingerprint"`
    ClientVersion string `json:"client_version"`
}

type sessionResponse struct {
    Status       string `json:"status"`
    Message      string `json:"message,omitempty"`
    SessionToken string `json:"session_token,omitempty"`
    ExpiresAt    int64  `json:"expires_at,omitempty"`
    CardExpAt    int64  `json:"card_expires_at,omitempty"`
}

func (h *ClientHandler) HandleActivate(w http.ResponseWriter, r *http.Request) {
    var req activateRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        WriteError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    session, err := h.cardSvc.Activate(r.Context(), req.Card, req.MachineID, req.Fingerprint, getClientIP(r), req.ClientVersion)
    if err != nil {
        WriteError(w, http.StatusBadRequest, translateError(err))
        return
    }

    WriteJSON(w, http.StatusOK, sessionResponse{
        Status:       "ok",
        SessionToken: session.ID.String(),
        ExpiresAt:    session.ExpiresAt.Unix(),
    })
}

func (h *ClientHandler) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
    var req struct {
        SessionToken  string `json:"session_token"`
        MachineID     string `json:"machine_id"`
        ClientVersion string `json:"client_version"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        WriteError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    tokenID, err := uuid.Parse(req.SessionToken)
    if err != nil {
        WriteError(w, http.StatusBadRequest, "invalid session token")
        return
    }

    session, err := h.cardSvc.Heartbeat(r.Context(), tokenID, req.MachineID, getClientIP(r), req.ClientVersion)
    if err != nil {
        WriteError(w, http.StatusBadRequest, translateError(err))
        return
    }

    WriteJSON(w, http.StatusOK, sessionResponse{
        Status:    "ok",
        ExpiresAt: session.ExpiresAt.Unix(),
    })
}

func (h *ClientHandler) HandleDeactivate(w http.ResponseWriter, r *http.Request) {
    var req struct {
        SessionToken string `json:"session_token"`
    }
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        WriteError(w, http.StatusBadRequest, "invalid request body")
        return
    }

    tokenID, err := uuid.Parse(req.SessionToken)
    if err != nil {
        WriteError(w, http.StatusBadRequest, "invalid session token")
        return
    }

    if err := h.cardSvc.Deactivate(r.Context(), tokenID); err != nil {
        WriteError(w, http.StatusInternalServerError, "failed to deactivate")
        return
    }

    WriteOK(w, nil)
}

func translateError(err error) string {
    switch err {
    case domain.ErrCardNotFound:
        return "卡密不存在"
    case domain.ErrCardDisabled:
        return "卡密已被禁用"
    case domain.ErrCardExpired:
        return "卡密已过期"
    case domain.ErrMachineBlacklisted:
        return "机器已被封禁"
    case domain.ErrSessionNotFound:
        return "会话不存在"
    case domain.ErrSessionExpired:
        return "会话已过期"
    case domain.ErrMaxSessions:
        return "已达最大会话数"
    default:
        return "操作失败"
    }
}

func getClientIP(r *http.Request) string {
    if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
        return fwd
    }
    return r.RemoteAddr
}
```

Note: add `import "github.com/lingqiao/server/internal/domain"` to client.go.

- [ ] **Step 3: Verify compilation**

```bash
cd server
go build ./internal/handler/
```

- [ ] **Step 4: Commit**

```bash
git add internal/handler/
git commit -m "feat: implement HTTP handlers for client API"
```

---

## Task 11: Middleware

**Files:**
- Create: `server/internal/middleware/auth.go`
- Create: `server/internal/middleware/ratelimit.go`
- Create: `server/internal/middleware/cors.go`
- Create: `server/internal/middleware/logging.go`

- [ ] **Step 1: Implement HMAC auth middleware**

```go
// server/internal/middleware/auth.go
package middleware

import (
    "net/http"

    "github.com/lingqiao/server/internal/service"
)

func HMACAuth(authSvc *service.AuthService, clientSecret string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Extract HMAC headers
            signature := r.Header.Get("X-Signature")
            timestamp := r.Header.Get("X-Timestamp")
            nonce := r.Header.Get("X-Nonce")

            if signature == "" || timestamp == "" || nonce == "" {
                http.Error(w, `{"status":"error","message":"missing auth headers"}`, http.StatusUnauthorized)
                return
            }

            // Read body for signature verification
            body := readBodyForAuth(r)

            if err := authSvc.VerifyHMAC(clientSecret, body, signature, timestamp, nonce); err != nil {
                http.Error(w, `{"status":"error","message":"auth failed"}`, http.StatusUnauthorized)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

func readBodyForAuth(r *http.Request) string {
    if r.Body == nil {
        return ""
    }
    buf, _ := io.ReadAll(r.Body)
    r.Body = io.NopCloser(bytes.NewReader(buf))
    return string(buf)
}
```

Add imports: `bytes`, `io`.

- [ ] **Step 2: Implement rate limiter**

```go
// server/internal/middleware/ratelimit.go
package middleware

import (
    "net/http"
    "sync"
    "time"

    "github.com/lingqiao/server/internal/handler"
)

type RateLimiter struct {
    mu       sync.Mutex
    attempts map[string][]time.Time
    window   time.Duration
    limit    int
}

func NewRateLimiter(window time.Duration, limit int) *RateLimiter {
    return &RateLimiter{
        attempts: make(map[string][]time.Time),
        window:   window,
        limit:    limit,
    }
}

func (rl *RateLimiter) Allow(key string) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    now := time.Now()
    cutoff := now.Add(-rl.window)
    var valid []time.Time
    for _, t := range rl.attempts[key] {
        if t.After(cutoff) {
            valid = append(valid, t)
        }
    }
    if len(valid) >= rl.limit {
        rl.attempts[key] = valid
        return false
    }
    rl.attempts[key] = append(valid, now)
    return true
}

func (rl *RateLimiter) Middleware(keyFunc func(*http.Request) string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            key := keyFunc(r)
            if !rl.Allow(key) {
                handler.WriteError(w, http.StatusTooManyRequests, "rate limit exceeded")
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

- [ ] **Step 3: Implement CORS**

```go
// server/internal/middleware/cors.go
package middleware

import (
    "net/http"
    "os"
)

func CORS(allowedOriginEnv string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            origin := os.Getenv(allowedOriginEnv)
            if origin == "" {
                origin = "*"
            }
            w.Header().Set("Access-Control-Allow-Origin", origin)
            w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
            w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Signature, X-Timestamp, X-Nonce")
            w.Header().Set("Access-Control-Allow-Credentials", "true")

            if r.Method == http.MethodOptions {
                w.WriteHeader(http.StatusOK)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

- [ ] **Step 4: Implement request logging**

```go
// server/internal/middleware/logging.go
package middleware

import (
    "log"
    "net/http"
    "time"
)

func Logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        next.ServeHTTP(w, r)
        log.Printf("%s %s %s %v", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
    })
}
```

- [ ] **Step 5: Verify compilation**

```bash
cd server
go build ./internal/middleware/
```

- [ ] **Step 6: Commit**

```bash
git add internal/middleware/
git commit -m "feat: implement middleware (HMAC auth, rate limiter, CORS, logging)"
```

---

## Task 12: Application Entry Point

**Files:**
- Create: `server/cmd/server/main.go`

- [ ] **Step 1: Implement main.go**

```go
// server/cmd/server/main.go
package main

import (
    "context"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/lingqiao/server/internal/config"
    "github.com/lingqiao/server/internal/handler"
    "github.com/lingqiao/server/internal/middleware"
    "github.com/lingqiao/server/internal/repository/postgres"
    "github.com/lingqiao/server/internal/service"
)

func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("load config: %v", err)
    }

    ctx := context.Background()

    // Database
    pool, err := postgres.NewPool(ctx, cfg.Database.URL)
    if err != nil {
        log.Fatalf("connect to database: %v", err)
    }
    defer pool.Close()

    // Repositories
    cardRepo := postgres.NewCardRepo(pool)
    sessionRepo := postgres.NewSessionRepo(pool)
    userRepo := postgres.NewUserRepo(pool)
    machineRepo := postgres.NewMachineRepo(pool)
    auditRepo := postgres.NewAuditRepo(pool)

    // Services
    cardSvc := service.NewCardService(cardRepo, sessionRepo, machineRepo, auditRepo)
    authSvc := service.NewAuthService(userRepo, cfg.Auth.JWTSecret)

    // Handlers
    clientHandler := handler.NewClientHandler(cardSvc, authSvc)

    // Session cleanup goroutine
    go func() {
        ticker := time.NewTicker(5 * time.Minute)
        defer ticker.Stop()
        for range ticker.C {
            n, _ := sessionRepo.DeleteExpired(context.Background())
            if n > 0 {
                log.Printf("cleaned up %d expired sessions", n)
            }
        }
    }()

    // Client API router
    clientRouter := chi.NewRouter()
    clientRouter.Use(middleware.CORS("ADMIN_ORIGIN"))
    clientRouter.Use(middleware.Logging)

    clientRouter.Post("/api/v1/activate", clientHandler.HandleActivate)
    clientRouter.Post("/api/v1/heartbeat", clientHandler.HandleHeartbeat)
    clientRouter.Post("/api/v1/deactivate", clientHandler.HandleDeactivate)
    clientRouter.Get("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"status":"ok"}`))
    })

    // Start servers
    adminAddr := fmt.Sprintf(":%d", cfg.Server.AdminPort)
    agentAddr := fmt.Sprintf(":%d", cfg.Server.AgentPort)

    adminServer := &http.Server{
        Addr:         adminAddr,
        Handler:      clientRouter,
        ReadTimeout:  300 * time.Second,
        WriteTimeout: 300 * time.Second,
        IdleTimeout:  120 * time.Second,
    }

    go func() {
        log.Printf("admin server starting on %s", adminAddr)
        if err := adminServer.ListenAndServeTLS(cfg.TLS.Cert, cfg.TLS.Key); err != http.ErrServerClosed {
            log.Fatalf("admin server error: %v", err)
        }
    }()

    // Graceful shutdown
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    log.Println("shutting down...")
    shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    adminServer.Shutdown(shutdownCtx)
    log.Println("server stopped")
}
```

Add import: `"fmt"`.

- [ ] **Step 2: Verify compilation**

```bash
cd server
go build ./cmd/server/
```

- [ ] **Step 3: Commit**

```bash
git add cmd/server/
git commit -m "feat: implement application entry point with chi router"
```

---

## Task 13: JSON Data Migration Tool

**Files:**
- Create: `server/cmd/migrate-json/main.go`

- [ ] **Step 1: Implement migration tool**

```go
// server/cmd/migrate-json/main.go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/google/uuid"
    "github.com/lingqiao/server/internal/config"
    "github.com/lingqiao/server/internal/domain"
    "github.com/lingqiao/server/internal/repository/postgres"
)

// Old JSON structures matching the current storage format
type OldCard struct {
    Code        string     `json:"code"`
    MachineID   string     `json:"machine_id,omitempty"`
    AgentID     string     `json:"agent_id,omitempty"`
    CreatedAt   time.Time  `json:"created_at"`
    ActivatedAt *time.Time `json:"activated_at,omitempty"`
    ExpiresAt   time.Time  `json:"expires_at"`
    Status      string     `json:"status"`
    Note        string     `json:"note,omitempty"`
    MaxSessions int        `json:"max_sessions"`
}

type OldData struct {
    Cards     map[string]*OldCard    `json:"cards"`
    Sessions  map[string]interface{} `json:"sessions"`
    Blacklist map[string]interface{} `json:"blacklist"`
    Agents    map[string]interface{} `json:"agents"`
}

func main() {
    if len(os.Args) < 2 {
        log.Fatal("usage: migrate-json <path-to-data.json>")
    }
    jsonPath := os.Args[1]

    cfg, err := config.Load()
    if err != nil {
        log.Fatalf("load config: %v", err)
    }

    ctx := context.Background()
    pool, err := postgres.NewPool(ctx, cfg.Database.URL)
    if err != nil {
        log.Fatalf("connect to database: %v", err)
    }
    defer pool.Close()

    data, err := os.ReadFile(jsonPath)
    if err != nil {
        log.Fatalf("read json file: %v", err)
    }

    var oldData OldData
    if err := json.Unmarshal(data, &oldData); err != nil {
        log.Fatalf("parse json: %v", err)
    }

    cardRepo := postgres.NewCardRepo(pool)

    migrated := 0
    for _, oldCard := range oldData.Cards {
        status := domain.CardStatus(oldCard.Status)
        var durHours int
        if !oldCard.ExpiresAt.IsZero() && !oldCard.ActivatedAt.IsZero() {
            durHours = int(oldCard.ExpiresAt.Sub(*oldCard.ActivatedAt).Hours())
        } else {
            durHours = 24 * 30 // default 30 days
        }

        card := &domain.Card{
            ID:          uuid.New(),
            Code:        oldCard.Code,
            Duration:    durHours,
            Status:      status,
            Note:        oldCard.Note,
            MaxSessions: oldCard.MaxSessions,
            MachineFP:   oldCard.MachineID,
            CreatedAt:   oldCard.CreatedAt,
            ActivatedAt: oldCard.ActivatedAt,
            ExpiresAt:   &oldCard.ExpiresAt,
        }

        if oldCard.AgentID != "" {
            agentUUID, err := uuid.Parse(oldCard.AgentID)
            if err == nil {
                card.AgentID = &agentUUID
            }
        }

        if err := cardRepo.Create(ctx, card); err != nil {
            log.Printf("WARN: skip card %s: %v", oldCard.Code, err)
            continue
        }
        migrated++
    }

    fmt.Printf("migrated %d cards\n", migrated)
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd server
go build ./cmd/migrate-json/
```

- [ ] **Step 3: Commit**

```bash
git add cmd/migrate-json/
git commit -m "feat: add JSON to PostgreSQL data migration tool"
```

---

## Task 14: Integration Test and Verification

**Files:**
- Create: `server/internal/handler/client_test.go`

- [ ] **Step 1: Write integration test for client API**

```go
// server/internal/handler/client_test.go
package handler_test

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/lingqiao/server/internal/handler"
    "github.com/lingqiao/server/internal/repository/memory"
    "github.com/lingqiao/server/internal/service"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) (*chi.Mux, *service.CardService) {
    t.Helper()
    cardRepo := memory.NewCardRepo()
    sessionRepo := memory.NewSessionRepo()
    machineRepo := memory.NewMachineRepo()
    auditRepo := memory.NewAuditRepo()
    userRepo := memory.NewUserRepo()

    cardSvc := service.NewCardService(cardRepo, sessionRepo, machineRepo, auditRepo)
    authSvc := service.NewAuthService(userRepo, "test-jwt-secret")

    clientHandler := handler.NewClientHandler(cardSvc, authSvc)

    mux := chi.NewRouter()
    mux.Post("/api/v1/activate", clientHandler.HandleActivate)
    mux.Post("/api/v1/heartbeat", clientHandler.HandleHeartbeat)
    mux.Post("/api/v1/deactivate", clientHandler.HandleDeactivate)

    return mux, cardSvc
}

func TestClientAPI_FullFlow(t *testing.T) {
    mux, cardSvc := setupTestServer(t)
    ctx := context.Background()

    // Generate a card first
    card, err := cardSvc.Generate(ctx, 24*30, "test", 1, nil)
    require.NoError(t, err)

    // Test activate
    activateBody, _ := json.Marshal(map[string]string{
        "card":           card.Code,
        "machine_id":     "machine-123",
        "fingerprint":    "fp-abc",
        "client_version": "v2.1.13",
    })
    req := httptest.NewRequest("POST", "/api/v1/activate", bytes.NewReader(activateBody))
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    mux.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)
    var activateResp map[string]interface{}
    json.Unmarshal(rec.Body.Bytes(), &activateResp)
    assert.Equal(t, "ok", activateResp["status"])
    assert.NotEmpty(t, activateResp["session_token"])

    // Test heartbeat
    heartbeatBody, _ := json.Marshal(map[string]string{
        "session_token":  activateResp["session_token"].(string),
        "machine_id":     "machine-123",
        "client_version": "v2.1.13",
    })
    req = httptest.NewRequest("POST", "/api/v1/heartbeat", bytes.NewReader(heartbeatBody))
    req.Header.Set("Content-Type", "application/json")
    rec = httptest.NewRecorder()
    mux.ServeHTTP(rec, req)

    assert.Equal(t, http.StatusOK, rec.Code)
    var heartbeatResp map[string]interface{}
    json.Unmarshal(rec.Body.Bytes(), &heartbeatResp)
    assert.Equal(t, "ok", heartbeatResp["status"])
}
```

- [ ] **Step 2: Run the integration test**

```bash
cd server
go test ./internal/handler/ -run TestClientAPI_FullFlow -v
```

Expected: PASS.

- [ ] **Step 3: Run all tests**

```bash
cd server
go test ./... -v -count=1
```

Expected: all tests PASS.

- [ ] **Step 4: Verify full build**

```bash
cd server
go build ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/handler/client_test.go
git commit -m "feat: add integration test for client API full flow"
```

---

## Task 15: Cleanup and Final Verification

- [ ] **Step 1: Remove old server files**

```bash
cd server
# Keep backups of old files
mkdir -p _old
mv main.go _old/
mv card.go _old/
mv handler.go _old/
mv admin.go _old/
mv agent.go _old/
mv auth.go _old/
mv config.go _old/
mv storage.go _old/
mv payload.go _old/
mv update.go _old/
mv middleware.go _old/
mv httputil.go _old/
```

- [ ] **Step 2: Run all tests one final time**

```bash
cd server
go test ./... -v -count=1
```

- [ ] **Step 3: Verify Docker Compose environment**

```bash
cd server
docker compose up -d
sleep 3
DATABASE_URL="postgres://postgres:lingqiao_dev@localhost:5432/lingqiao?sslmode=disable" make migrate-up
docker compose down
```

- [ ] **Step 4: Final commit**

```bash
git add -A
git commit -m "refactor: complete server restructure — modular architecture with PostgreSQL"
```

---

## Spec Coverage Checklist

| Spec Requirement | Task |
|---|---|
| PostgreSQL schema | Task 3 |
| Domain models | Task 2 |
| Repository interfaces | Task 4 |
| Card repository | Task 5 |
| Session/User/Machine/Audit/Cache repos | Task 6 |
| Config management | Task 7 |
| Card service (generate, activate, heartbeat) | Task 8 |
| Auth service (HMAC, JWT) | Task 9 |
| HTTP handlers | Task 10 |
| Middleware (auth, rate limit, CORS, logging) | Task 11 |
| Application entry point | Task 12 |
| JSON data migration | Task 13 |
| Integration tests | Task 14 |
| Docker Compose | Task 1 |
| CI/CD (Makefile) | Task 1 |
