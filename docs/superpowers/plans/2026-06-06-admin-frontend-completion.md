# Admin Frontend Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restore the full admin business surface in the new Vite/TypeScript admin frontend.

**Architecture:** Keep the existing Go admin APIs and Vite multi-page frontend. Expand the admin HTML with the missing pages and modals, then wire them in TypeScript through the shared `ApiClient`, `AppShell`, and `ToastHost`.

**Tech Stack:** Go backend, Vite, TypeScript, plain DOM APIs, shared CSS.

---

### Task 1: Restore Admin Pages

**Files:**
- Modify: `_src_tmp/injector-server/web/admin/index.html`

- [ ] Add nav items and pages for sessions, machines, blacklist, announcement, password, invites, and richer updates.
- [ ] Add modals for import, edit card, extend card, blacklist, machine detail, agent cards, and reset agent password.

### Task 2: Wire Business Actions

**Files:**
- Modify: `_src_tmp/injector-server/web/admin/src/main.ts`

- [ ] Add loaders for all restored pages.
- [ ] Add card edit, extend, import, CSV/JSON export.
- [ ] Add sessions force logout, machines detail, blacklist add/remove.
- [ ] Add announcement and version push save/clear/upload.
- [ ] Add password change.
- [ ] Add full agent and invite management.

### Task 3: Verify and Deploy

**Commands:**
- `npm run check`
- `$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'; go test ./...`
- Linux build and deploy to `/opt/injector-server` after backup.

**Note:** This workspace is not a git repository, so commit steps are not applicable.

### Task 4: Productized Admin Upgrade

**Files:**
- Modify: `_src_tmp/injector-server/admin.go`
- Modify: `_src_tmp/injector-server/main.go`
- Modify: `_src_tmp/injector-server/web/admin/index.html`
- Modify: `_src_tmp/injector-server/web/admin/src/main.ts`
- Modify: `_src_tmp/injector-server/web/shared/src/styles.css`
- Test: `_src_tmp/injector-server/admin_ops_test.go`

- [ ] Add an operations overview API with today metrics, expiring cards, risky machines, agent leaderboard, and recent audit.
- [ ] Add advanced card filters for bound state, agent, expiration window, and max session count.
- [ ] Extend audit query with keyword and time range filters.
- [ ] Rework the admin frontend into an operations cockpit with high-signal panels, advanced cards, risk workflows, richer announcements/version push, searchable audit, and improved table interactions.
- [ ] Run frontend check, Go tests, build Linux binary, deploy, and perform review.
