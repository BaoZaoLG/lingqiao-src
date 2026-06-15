# Lingqiao Business Services Phase 2 Batch 3 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Route existing `CardManager` audit writes through one helper so the legacy JSON audit log and the new `internal/audit.Recorder` stay in sync.

**Architecture:** Keep `AuditEntry` and the current JSON persistence format. Replace direct `cm.auditLog = append(...)` writes with `cm.appendAuditLocked(...)` while the caller already holds `cm.mu`.

**Tech Stack:** Go 1.25, existing `internal/audit` recorder, existing `CardManager` tests.

---

## Task 1: Add Audit Mirror Tests

- [ ] Add failing tests for card generation, activation, agent creation, and blacklist changes mirroring to `internal/audit.Recorder`.
- [ ] Run `go test .` and verify tests fail where direct audit append sites do not mirror to recorder.

## Task 2: Refactor Audit Append Sites

- [ ] Replace direct `cm.auditLog = append(cm.auditLog, AuditEntry{...})` calls in `card.go` with `cm.appendAuditLocked(AuditEntry{...})`.
- [ ] Keep persistence behavior unchanged.
- [ ] Run `go test ./...`.

## Verification

- [ ] Run `gofmt` on touched Go files.
- [ ] Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./...
```

## Constraints

- Do not change external HTTP routes.
- Do not change existing audit JSON format.
- Do not enhance injection, stealth, bypass, or covert delivery behavior.
- Do not commit because the workspace root is not a git repository.
