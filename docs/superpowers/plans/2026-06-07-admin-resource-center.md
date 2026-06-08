# Admin Resource Center Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade script and update management from single active files into selectable repositories, improve admin usability, and clean stale local build artifacts.

**Architecture:** Server keeps durable resource manifests under `data/scripts` and `data/updates`. Admin APIs list, create, activate, and delete resources; client APIs continue to serve only the active script/update. The frontend adds repository-style management pages while preserving existing activation and download contracts.

**Tech Stack:** Go HTTP server, TypeScript Vite admin, CMake/Win32 client build.

---

### Task 1: Script Repository
- [ ] Add tests for multiple scripts, active selection, and delete protection.
- [ ] Implement script index persistence, migration from `active.json`, admin actions, and active client download.
- [ ] Verify with `go test ./...`.

### Task 2: Update Repository
- [ ] Add tests for retaining uploaded packages, listing versions, selecting active package, and delete protection.
- [ ] Implement update manifest list, active selection, and admin actions.
- [ ] Verify with `go test ./...`.

### Task 3: Admin UI
- [ ] Upgrade scripts page to list/select/edit/delete scripts.
- [ ] Upgrade updates page to list/select/delete packages and show policy clearly.
- [ ] Add system resource summary.
- [ ] Verify with `npm run check`.

### Task 4: Cleanup And Build
- [ ] Identify generated build/cache directories inside workspace.
- [ ] Remove only verified build/cache artifacts.
- [ ] Reconfigure/rebuild server, admin web, DLL, and client.
- [ ] Report artifacts and optional deployment steps.
