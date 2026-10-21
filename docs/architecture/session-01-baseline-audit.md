# Session 01 Baseline Audit

Status: batch S01-0001 through S01-0050 complete  
Last reviewed: 2026-07-17 Asia/Shanghai

This document is the execution evidence for the first Session 01 batch. It does not replace the queue; it records verified ownership, entry points, and handover risks.

## Root Boundary Map

| Classification | Paths |
| --- | --- |
| Source | `client/`, `server/`, `server-java/`, `packages/`, `e2e_test/`, `installer/`, `scripts/`, `docs/`, `.github/` |
| Native dependencies | `cef/`, `chromium/` |
| Generated / build | `build/`, `build64/`, `build-status-review/`, `dist/`, `server/web-dist/` |
| Runtime data / secrets | `data/`, `server/data/`, `certs/`, `server/certs/`, `*.log` |
| Local tool state | `.superpowers/`, `.vscode/`, `.agents/` |
| Protected unclassified root artifacts | `management.html`, `mihomo.gz`, `clash_sub.yaml`, `proxies.json`, `patch*.py`, local server executables under `server/` |

Governance baseline: `docs/architecture/repository-boundaries.md`.

## Entry Points Verified

| ID range | Entry | Finding |
| --- | --- | --- |
| S01-0001 | Root inventory | Source vs generated vs runtime boundaries are clear; local binaries and proxy/tool files remain protected unclassified assets. |
| S01-0002 | `README.md` | Current README covers client, Go, web, Java sidecar, AutoExam, installer, scripts, and check gates. |
| S01-0003 | `.gitignore` | Covers build outputs, web-dist contents, data, certs, logs, server binaries, and local tool dirs. Runtime secrets currently present on disk are ignored, not deleted. |
| S01-0004 | `build.bat` | Defaults to Win32 Release via VS 2022; overridable `BUILD_DIR`, Qt path, generator, platform. Builds `CefHook` and `Injector`. |
| S01-0005 | `scripts/check.ps1` | Gates web check, Go tests, E2E, govulncheck, and CMake configure into shared `build/`. Does not compile C++ targets. Recorded as CM-S01-0003. |
| S01-0006 | `server/go.mod` | Module `github.com/lingqiao/server`, Go 1.26.5, SQLite via `modernc.org/sqlite`, crypto via `golang.org/x/crypto`. |
| S01-0007 | `server/main.go` | Oversized composition root: migrate, DI, dual servers, routes, cleanup task, embed `web-dist`. |
| S01-0008 | `internal/config` | Env matrix: `PORT`, `AGENT_PORT`, `DATA_DIR`, `SESSION_TTL`. |
| S01-0009 | `server/config.go` | TLS always uses cwd-relative `certs/server.crt|key`; `CERT_HOST` defaults to `127.0.0.1`. Recorded as CM-S01-0002. |
| S01-0010 | `server/db.go` | Schema and JSON-to-SQLite migration live in main package tables for cards, sessions, blacklist, audit, agents, invites, exam. |
| S01-0011 | `server/handler.go` | Root `APIHandler` remains a compatibility hotspot for client API, HMAC, DLL, announcements. |
| S01-0012 | `server/middleware.go` | Client CORS is `*`; admin/agent use credentialed explicit origins; CSP and body limits applied. |
| S01-0013 | `server/admin.go` | Root `AdminHandler` still owns many admin routes while `features/admin` exists underneath. |
| S01-0014/0015 | `internal/auth` | HMAC and admin/session auth packages exist; no package tests yet. Recorded as CM-S01-0004. |
| S01-0016 | `internal/httpapi` | Shared response/security/rate-limit helpers with partial tests. |
| S01-0017-0026 | feature modules | sessions/cards/authorizations/agents/chat/releases/payloads/scripts/exam/blacklist ownership confirmed and mapped in `server-module-map.md`. |
| S01-0027-0030 | audit/storage/releases/cardcode | Storage adapter still used by chat/payload compatibility; releases and cardcode are golden-vector candidates. |
| S01-0031-0037 | web MPA | Vite dual entry admin/agent, shared `api.ts`, output `server/web-dist`, npm `check = typecheck + build`. |
| S01-0038 | OpenAPI | Only `/api/v1/activate`, `/heartbeat`, `/update/check` documented; large admin/agent/payload/exam surface missing. |
| S01-0039 | ADR set | Modular monolith, hexagonal boundaries, release manifest protocol, SQLite storage, script single source of truth. |
| S01-0040 | config docs | `docs/operations/config.md` matches current env owners and documents the session TTL wiring gap. |
| S01-0041 | security docs | Present under `docs/security`; runtime secret lifecycle still needs Session 05 expansion. |
| S01-0042-0047 | client build and controllers | CMake targets `CefHook`, `Injector` (`CX_LingQiao`), manual injectors, `ClientTests`; session/update controllers and `api_client` own protocol compatibility. |
| S01-0048 | client config | `config.cpp` is the single definition home for shared host/path/secret globals; version/public key come from CMake definitions. |
| S01-0049 | client crypto | HMAC-SHA256, PBKDF2, AES-GCM, cert pin helpers in `crypto.h`; cross-language golden vectors required. |
| S01-0050 | CefHook entry | `dllmain.cpp` is the injected DLL entry and remains outside Java migration scope, but DLL download semantics must stay stable. |

## Test Coverage Snapshot From This Batch

| Package | Go files | `*_test.go` |
| --- | ---: | ---: |
| `internal/config` | 1 | 0 |
| `internal/auth` | 2 | 0 |
| `internal/storage` | 1 | 0 |
| `features/security/blacklist` | 2 | 0 |
| `features/sessions` | 12 | 4 |
| `features/cards` | 10 | 3 |
| `features/agents` | 11 | 4 |
| `features/exam` | 3 | 1 |
| `platform/cardcode` | 2 | 1 |

## Confirmed Handoffs

- Session 02: expand OpenAPI beyond the three client paths.
- Session 03: decide real client session TTL semantics vs fixed 30 minutes.
- Session 05: CORS split, TLS cert directory policy, runtime secret lifecycle, bridge credential exposure.
- Session 07: quality-gate C++ compile isolation and toolchain alignment on Go 1.26.5.
- Session 08: add package tests for auth/storage/blacklist/config.
- Session 09: classify or dispose root local artifacts and server binary clutter only with reference proof.
- Session 10: follow migration waves in `server-module-map.md`; keep payloads/releases/auth late.

## Explicit Non-Actions

- No deletion of runtime data, certificates, build trees, or unclassified root artifacts.
- No production code rewrites in this batch beyond documentation and progress governance.
- No git commit/push/branch operations.

## Batch S01-0051 through S01-0100

Status: completed  
Evidence date: 2026-07-17

### Client / package / delivery entry points

| ID | Scope | Classification / finding |
| --- | --- | --- |
| S01-0051 | `client/src/hook/thread.cpp` | Hook runtime background thread and CEF load-handler install; client-owned, not Java migration scope. |
| S01-0052 | `client/src/hook/script_loader.h` | Script download/load path; must stay compatible with server scripts API and cert pin from `crypto.h`. |
| S01-0053 | `client/src/hook/bridge_protocol.h` | Bridge protocol v2 constants and error codes; frontend/native boundary only. |
| S01-0054 | `client/src/security/machine_fp.h` | Machine fingerprint inputs for license binding; server must preserve field semantics. |
| S01-0055/0056 | AutoExam source and root copy | SHA-256 identical (`54C14DB9...F7C8`, 86993 bytes). Source of truth remains `packages/autoexam/src/autoexam.js`; root file is compatibility/publish copy. |
| S01-0057 | `installer/` | WiX v4 skeleton: `build_installer.ps1`, `build_portable_setup.ps1`, `Bundle.wxs`, `Product.wxs`. |
| S01-0058 | `scripts/release.ps1` | Orchestrates web build, Go `dist/server/lingqiao-server.exe`, AutoExam build, C++ `build` Release, installer from `build\src\Release`. |
| S01-0059 | `e2e_test` | Separate Go module on 1.26.5; included by `scripts/check.ps1` and release workflow tests. |
| S01-0060 | `data/` | Runtime `lingqiao.db`, `releases.db`, `upload_key`, `update_manifest_seed.key`. Gitignored via `/data/`. High-risk secrets on disk. |
| S01-0061/0062 | `certs/`, `server/certs/` | Both contain `server.crt` and `server.key`. Code loads cwd `certs/`; dual directories are a deployment hazard (CM-S01-0002). |
| S01-0063 | `server/web-dist` | Embed input with `admin/`, `agent/`, `assets/`, tracked `.gitkeep` only. |
| S01-0064 | multi-build trees | `build/`, `build64/`, `build-status-review/` are local CMake outputs. Release CI uses `build-release`. Do not delete without owner request. |
| S01-0065 | server local binaries | Multiple local executables under `server/` including oddly named path-flattened artifacts. Ignored by `.gitignore`; treat as generated clutter for Session 09. |
| S01-0066/0072/0073 | `.github` / workflows | Source-owned CI. `release.yml` tags/workflow_dispatch: Node 24, Go 1.26.5, Qt 5.15.2 win32, web check, Go+e2e tests, server build, AutoExam, WiX, CMake `build-release`. |
| S01-0067/0079 | `.vscode` | Local IDE state. `settings.json` hardcodes `cmake.sourceDirectory` to `.../_src_tmp`; local-only, already ignored. |
| S01-0068/0074-0078 | `.superpowers/**` | Local agent brainstorm state under `.superpowers/brainstorm/...`; ignored and not product source. |
| S01-0069 | `docs/superpowers` | Historical plans/specs; subordinate to current SESSION queues and architecture docs. |
| S01-0070 | `docs/PROJECT_HANDOFF_REPORT.md` | Dated 2026-06-28 and claims a clean baseline without local binaries; superseded by current inventory and `docs/SESSION_01_PARALLEL_HANDOFF.md`. |
| S01-0071 | `docs/ARCHITECTURE_V2_PROPOSAL.md` | File is UTF-8 with corrupted Chinese sequences (mojibake). Structural intent still useful; encoding repair is a documentation follow-up, not a code change. |
| S01-0080-0100 | `build` and `build-status-review/**` tree nodes | Generated CMake/MSBuild output only. Ownership: local build cache. Rule: never source of truth; no recursive cleanup in Session 01. |

### Additional handoffs from this batch

- Session 07: release script uses shared `build/` and deletes `CMakeCache.txt`; CI uses isolated `build-release`. Align local release and CI build directories.
- Session 09: server binary clutter and path-flattened executable names need evidence-based cleanup later.
- Session 01 later docs batch: repair or replace mojibake `ARCHITECTURE_V2_PROPOSAL.md` with a clean UTF-8 architecture status document.

## Batch S01-0101 through S01-1180

Status: completed via generated/dependency tree classification  
Evidence: `docs/architecture/generated-and-dependency-trees.md`

- S01-0101..S01-0384, S01-0405..S01-0450, S01-0461..S01-0600, S01-0602..S01-1180: generated CMake/MSBuild trees under `build*`, `build-status-review`, `client/build`, and their file-level nodes.
- S01-0385..S01-0399: `cef/**` third-party include dependency tree used by client CMake.
- S01-0400: `certs` runtime secret directory (reconfirmed ignored; dual with `server/certs`).
- S01-0401..S01-0403: `chromium/**` vendored include dependency tree.
- S01-0404: `client/` product source root ownership confirmed.
- S01-0451: release workflow pinned toolchain and isolated `build-release`.
- S01-0452: `.gitignore` already covers build/runtime/secret classes used in this batch.
- S01-0453..S01-0458: `.superpowers` local brainstorm HTML/state only.
- S01-0459: `.vscode/settings.json` hardcodes `_src_tmp` path; local-only.
- S01-0460: `AutoExam.js` identical compatibility copy.
- S01-0601: `build.bat` reconfirmed as Win32 Release client entry.

No deletions performed.

## Batch S01-1181 through S01-1200

Status: completed

| ID | Finding |
| --- | --- |
| S01-1181 | Client `api_client` -> Go `internal/auth` HMAC -> root/feature handlers. Highest Java compatibility risk. |
| S01-1182 | `session_controller` -> `features/sessions` activation/heartbeat/deactivate semantics. |
| S01-1183 | `update_controller` -> `features/releases` + `internal/releases` manifest/signature flow. |
| S01-1184 | `script_loader` -> `features/scripts` download/active script path. |
| S01-1185 | Payload key exchange and encrypted chunk/DLL delivery couple client runtime to `features/payloads`. |
| S01-1186 | Web deploy chain: npm build -> `server/web-dist` -> Go embed; Java must choose static-resource strategy later. |
| S01-1187 | Client CMake outputs feed installer WiX via `scripts/release.ps1`. |
| S01-1188 | Data chain: `DATA_DIR/lingqiao.db` opened by `server/db.go` and feature repositories. |
| S01-1189 | Admin auth chain: login token/cookie -> middleware/auth helpers -> permission-gated feature handlers. |
| S01-1190 | Agent auth chain uses separate listener/handlers/services; must not merge with admin context. |
| S01-1191 | `server/main.go` CLI `--generate-card` path exists beside HTTPS servers. |
| S01-1192 | `APIHandler.SessionCleanupTask` is the client-session reaper scheduler started from composition root. |
| S01-1193 | Env matrix documented in `docs/operations/config.md`; client TTL wiring gap remains CM-S01-0001. |
| S01-1194 | Dual admin surface: root `server/admin*.go` compatibility handlers + `internal/features/admin`. |
| S01-1195 | AutoExam dual path: package source authoritative, root copy identical. |
| S01-1196 | `build-status-review` is disposable generated cache candidate, not product module. |
| S01-1197 | Flattened/odd server binary names under `server/` are local build clutter, ignored, cleanup deferred to Session 09 with proof. |
| S01-1198 | Installer migration check: WiX scripts are present; signing env vars optional; no Session 01 code change. |
| S01-1199 | Docs migration check: architecture/module/config/handoff docs now form the Session 01 governance baseline. |
| S01-1200 | Cards migration check: `features/cards` is foundational domain; Java must stay read-only/shadow until lifecycle contracts pass. |
