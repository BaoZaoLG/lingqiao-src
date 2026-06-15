# Lingqiao Business Services Phase 2 Batch 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the new update metadata service into the existing update handlers and add an audit recorder bridge for card manager business events.

**Architecture:** Preserve existing HTTP routes and JSON file names. Use compatibility wrappers in the `main` package so handlers keep their current shape while business logic starts depending on internal services.

**Tech Stack:** Go 1.25, standard library tests, existing `internal/storage`, `internal/updates`, and `internal/audit` packages.

---

## Task 1: Preserve Update Metadata File Compatibility

- [ ] Add a failing test proving `internal/updates.MetadataStore.Save` writes `info.json` when backed by a JSON store rooted at `data/updates`.
- [ ] Update `internal/updates.MetadataStore` to use storage key `info`.
- [ ] Run `go test ./internal/updates`.

## Task 2: Wire Update Metadata Service Into Existing Handlers

- [ ] Add a failing main-package test proving `saveCurrentUpdateForTest` persists metadata readable by `getCurrentUpdate`.
- [ ] Refactor `update.go` to alias `UpdateInfo` to `internal/updates.UpdateInfo`.
- [ ] Add a package-level metadata store rooted at `data/updates`.
- [ ] Replace direct `data/updates/info.json` write/read paths with metadata store calls.
- [ ] Keep upload/download route behavior and response fields unchanged.
- [ ] Run `go test ./...`.

## Task 3: Audit Recorder Bridge

- [ ] Add a failing main-package test proving `CardManager` mirrors bulk operation audit entries into an optional `internal/audit.Recorder`.
- [ ] Add an optional audit recorder field and setter on `CardManager`.
- [ ] Add a helper that appends existing `AuditEntry` and mirrors to `audit.Event`.
- [ ] Wire the helper into `BulkUpdateCardsDetailed`.
- [ ] Run `go test ./...`.

## Verification

- [ ] Run `gofmt` on touched Go files.
- [ ] Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./...
```

## Constraints

- Do not change external HTTP routes.
- Do not change `data/updates/info.json`.
- Do not enhance injection, stealth, bypass, or covert delivery behavior.
- Do not commit because the workspace root is not a git repository.
