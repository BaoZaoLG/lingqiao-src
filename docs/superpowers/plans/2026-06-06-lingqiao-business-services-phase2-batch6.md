# Lingqiao Business Services Phase 2 Batch 6 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract card activation and heartbeat session rules into `internal/cards` while preserving current `CardManager` API behavior.

**Architecture:** Add a pure session lifecycle service that receives typed card/session state and returns updated state. `CardManager` keeps locking, persistence, token generation, and JSON models.

**Tech Stack:** Go 1.25, existing `internal/cards` package, existing `CardManager` tests.

---

## Task 1: Session Lifecycle Service

- [ ] Add failing tests for machine binding, max active sessions, first activation billing start, expired session rejection, card invalidation, and heartbeat renewal.
- [ ] Implement `internal/cards/session.go` with `Session`, `ActivationInput`, `HeartbeatInput`, `SessionService`, `Activate`, and `Heartbeat`.
- [ ] Run `go test ./internal/cards`.

## Task 2: CardManager Wiring

- [ ] Refactor `ActivateCard` and `Heartbeat` to delegate business decisions to `internal/cards.SessionService`.
- [ ] Keep `CardManager` responsible for locks, maps, token generation, persistence, and audit.
- [ ] Preserve existing session duration, card expiration behavior, and error messages.
- [ ] Run `go test ./...`.

## Verification

- [ ] Run `gofmt` on touched Go files.
- [ ] Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./...
```

## Constraints

- Do not change external HTTP routes.
- Do not change existing `Card` or `Session` JSON format.
- Do not change HMAC, update, download, or payload behavior.
- Do not enhance injection, stealth, bypass, or covert delivery behavior.
- Do not commit because the workspace root is not a git repository.
