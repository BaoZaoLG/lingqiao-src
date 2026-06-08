# Versioned JS Module Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the stable CEF lifecycle DLL base from business JS by serving versioned JS modules from the server.

**Architecture:** The server owns script versions in `data/scripts` and exposes authenticated client/admin APIs. The DLL fetches the active JS module during CEF `OnLoadEnd`, verifies SHA-256, injects it, and falls back to embedded JS when the server path is unavailable. The V8HOOK branch stays isolated and is not referenced by the上线 CEF path.

**Tech Stack:** Go HTTP server, Vite TypeScript admin panel, WinHTTP/C++ DLL.

---

### Task 1: Server Script Module API

**Files:**
- Create: `injector-server/script_module.go`
- Create: `injector-server/script_module_test.go`
- Modify: `injector-server/main.go`

- [ ] Add failing tests for admin save/get and client token-gated download.
- [ ] Implement script metadata, SHA-256 hashing, version history, and active script persistence.
- [ ] Register `/api/v1/script` and `/admin/api/script`.
- [ ] Run `go test ./...`.

### Task 2: DLL Script Loader

**Files:**
- Create: `src/script_loader.h`
- Modify: `src/thread.cpp`

- [ ] Add a small loader that reads server host/port from compile-time config and gets `/api/v1/script`.
- [ ] Verify `sha256(content)` matches the server metadata.
- [ ] Inject remote JS first and fall back to embedded JS when fetch/verification fails.
- [ ] Build `CefHook` Release.

### Task 3: Admin Script UI

**Files:**
- Modify: `injector-server/web/admin/index.html`
- Modify: `injector-server/web/admin/src/main.ts`

- [ ] Add sidebar entry and `脚本模块` page.
- [ ] Add current version/SHA/updated time display.
- [ ] Add text editor, version field, save button, and reload button.
- [ ] Run `npm run check` and `npm run build`.

### Task 4: Final Verification

**Files:**
- Verify generated `injector-server/web-dist`
- Verify generated `build-hook-log/src/Release/CefHook.dll`

- [ ] Run `go test ./...`.
- [ ] Run `npm run check`.
- [ ] Run `npm run build`.
- [ ] Run `cmake --build _src_tmp\build-hook-log --config Release --target CefHook`.
- [ ] Report changed files and known deployment steps.
