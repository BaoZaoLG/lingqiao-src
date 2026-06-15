# Lingqiao Business Services Phase 2 Batch 5 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract basic card lifecycle rules into `internal/cards` and wire existing `CardManager` generation/update methods through that service without changing JSON shape or HTTP routes.

**Architecture:** Keep the existing `main.Card` persistence model. Add an internal lifecycle service with a compatible card model and converter functions at the `CardManager` boundary.

**Tech Stack:** Go 1.25, existing `internal/cards` package, existing `CardManager` tests.

---

## Task 1: Card Lifecycle Service

- [ ] Add failing tests for card creation, status update, extension reactivation, and detail update max-session clamp.
- [ ] Implement `internal/cards/lifecycle.go` with `Card`, `Status`, `LifecycleService`, `Create`, `UpdateStatus`, `Extend`, and `UpdateDetails`.
- [ ] Run `go test ./internal/cards`.

## Task 2: CardManager Lifecycle Wiring

- [ ] Refactor `GenerateCard`, `UpdateCardStatus`, `ExtendCard`, and `UpdateCardDetails` to call `internal/cards.LifecycleService`.
- [ ] Preserve existing `Card` JSON fields and audit action names.
- [ ] Keep activation and heartbeat logic unchanged in this batch.
- [ ] Run `go test ./...`.

## Verification

- [ ] Run `gofmt` on touched Go files.
- [ ] Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./...
```

## Constraints

- Do not change external HTTP routes.
- Do not change existing `Card` JSON format.
- Do not change activation, heartbeat, or session concurrency rules in this batch.
- Do not enhance injection, stealth, bypass, or covert delivery behavior.
- Do not commit because the workspace root is not a git repository.
