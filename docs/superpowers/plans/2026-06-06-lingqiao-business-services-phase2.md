# Lingqiao Business Services Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Start Phase 2 by extracting testable business helpers for card bulk operations, invites, and update metadata while preserving existing HTTP APIs.

**Architecture:** Add small internal packages for business result models and services, then bridge existing `main` package functions to them through compatibility wrappers. The current JSON files, handler routes, and response shapes remain compatible.

**Tech Stack:** Go 1.25, standard library tests, existing `internal/storage` package, existing `CardManager` compatibility layer.

---

## File Structure

- Create: `_src_tmp/injector-server/internal/cards/bulk.go`
  - Defines bulk card actions, per-item results, aggregate counts, and action validation.
- Create: `_src_tmp/injector-server/internal/cards/bulk_test.go`
  - Tests result aggregation and action validation.
- Modify: `_src_tmp/injector-server/card.go`
  - Adds `BulkUpdateCardsDetailed` and keeps `BulkUpdateCards` as a compatibility wrapper.
- Modify: `_src_tmp/injector-server/card_test.go`
  - Tests per-item bulk operation outcomes.
- Create: `_src_tmp/injector-server/internal/agents/invites.go`
  - Implements invite code service over `storage.Store`.
- Create: `_src_tmp/injector-server/internal/agents/invites_test.go`
  - Tests invite create, use, exhaustion, delete, and compatibility state.
- Modify: `_src_tmp/injector-server/agent.go`
  - Aliases `InviteCode` to the internal type and routes global invite functions through the service.
- Create: `_src_tmp/injector-server/internal/updates/metadata.go`
  - Implements update metadata store, version validation, safe filename, and SHA lookup.
- Create: `_src_tmp/injector-server/internal/updates/metadata_test.go`
  - Tests update metadata save/load and version validation.

## Task 1: Card Bulk Result Model

- [ ] Write failing tests in `internal/cards/bulk_test.go` for valid actions, invalid actions, and result count aggregation.
- [ ] Run `go test ./internal/cards` and verify it fails because the package is missing.
- [ ] Implement `internal/cards/bulk.go` with `BulkAction`, `BulkItemResult`, `BulkResult`, `ValidateBulkAction`, and `AddItem`.
- [ ] Run `go test ./internal/cards` and verify it passes.

## Task 2: CardManager Detailed Bulk Operations

- [ ] Write failing tests in `card_test.go` for `BulkUpdateCardsDetailed`, covering successful update, missing card skipped, and invalid action failure.
- [ ] Run `go test .` and verify it fails because `BulkUpdateCardsDetailed` is missing.
- [ ] Implement `BulkUpdateCardsDetailed` in `card.go` using `internal/cards.BulkResult`.
- [ ] Keep `BulkUpdateCards` as a wrapper returning the legacy `(affected int, err error)` result.
- [ ] Run `go test ./...` and verify all tests pass.

## Task 3: Invite Service Extraction

- [ ] Write failing tests in `internal/agents/invites_test.go` for create, use, max-use exhaustion, and delete.
- [ ] Run `go test ./internal/agents` and verify it fails because invite service is missing.
- [ ] Implement `internal/agents/invites.go` with `InviteCode`, `InviteService`, `List`, `Create`, `Delete`, and `ValidateAndUse`.
- [ ] Refactor `agent.go` global invite functions to call a default `InviteService` backed by `NewJSONStorage("data")`.
- [ ] Run `go test ./...` and verify all tests pass.

## Task 4: Update Metadata Service

- [ ] Write failing tests in `internal/updates/metadata_test.go` for version validation, safe filename, save/load, and SHA lookup.
- [ ] Run `go test ./internal/updates` and verify it fails because metadata service is missing.
- [ ] Implement `internal/updates/metadata.go` with `UpdateInfo`, `MetadataStore`, `ValidateVersion`, `SafeFilename`, `Save`, `Load`, and `SHAForVersion`.
- [ ] Keep `update.go` behavior compatible for this phase; deeper upload handler refactor is deferred to the next Phase 2 batch.
- [ ] Run `go test ./...` and verify all tests pass.

## Verification

- [ ] Run `gofmt` on all touched Go files.
- [ ] Run:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./...
```

- [ ] Confirm every package passes.

## Constraints

- Do not change external HTTP routes.
- Do not change current JSON file names.
- Do not enhance injection, stealth, bypass, or covert delivery behavior.
- Do not commit because `C:\Users\Li\Downloads\Lingqiao_src` is not currently a git repository.
