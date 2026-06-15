# Lingqiao Business Services Phase 2 Batch 4 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract agent account business rules into `internal/agents` and wire existing `CardManager` agent CRUD through that service without changing JSON shape or HTTP routes.

**Architecture:** Keep the existing `main.Agent` persistence model. Add an internal account service with a compatible `Account` model and converter functions at the `CardManager` boundary.

**Tech Stack:** Go 1.25, existing `internal/agents` package, existing `CardManager` tests.

---

## Task 1: Agent Account Service

- [ ] Add failing tests for agent account creation, duplicate username rejection, password update, status update, and delete existence checks.
- [ ] Implement `internal/agents/accounts.go` with `Account`, `AccountService`, `Create`, `UpdatePassword`, `UpdateStatus`, and `EnsureCanDelete`.
- [ ] Run `go test ./internal/agents`.

## Task 2: CardManager Agent CRUD Wiring

- [ ] Add main-package tests proving `CreateAgent`, `UpdateAgentPassword`, `UpdateAgentStatus`, and `DeleteAgent` still work and mirror audit events.
- [ ] Refactor `CardManager` agent CRUD to call `internal/agents.AccountService`.
- [ ] Preserve existing `Agent` JSON fields and audit action names.
- [ ] Run `go test ./...`.

## Verification

- [ ] Run `gofmt` on touched Go files.
- [ ] Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./...
```

## Constraints

- Do not change external HTTP routes.
- Do not change existing `Agent` JSON format.
- Do not change password hashing behavior.
- Do not enhance injection, stealth, bypass, or covert delivery behavior.
- Do not commit because the workspace root is not a git repository.
