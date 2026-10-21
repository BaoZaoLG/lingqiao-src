# Session 01 Parallel Handoff

Updated: 2026-07-17 Asia/Shanghai  
Owner: Session 01 (global control / architecture / repository governance)

## Supersedes

This handoff supersedes outdated cleanliness claims in `docs/PROJECT_HANDOFF_REPORT.md` (2026-06-28). Historical handoff content remains readable background only.

## Current Production Shape

- Production backend entry: `server/main.go` on ports from `PORT`/`AGENT_PORT`.
- Web admin/agent: Vite MPA under `server/web`, embedded from `server/web-dist`.
- Client: Qt Injector + CefHook via `build.bat` / CMake.
- Java: `server-java` sidecar only; default not in traffic path.
- AutoExam source: `packages/autoexam/src/autoexam.js` (root `AutoExam.js` is an identical compatibility copy).

## Active Parallel Session Ownership

| Session | Owns | Do not silently change from other sessions |
| --- | --- | --- |
| 01 | Repo boundaries, architecture docs, coordination | Product business rules |
| 02 | HTTP contracts, handlers, OpenAPI | DB schema |
| 03 | Service/domain semantics | Public route shapes without notice |
| 04 | Schema/migrations/repositories | Auth protocol |
| 05 | Security controls and secret handling | Unrelated UX refactors |
| 06 | Web pages and frontend UX | Backend contracts |
| 07 | Build/CI/release scripts | Runtime business logic |
| 08 | Tests and quality gates | Production shortcuts that weaken tests |
| 09 | Dead/duplicate code cleanup with proof | Runtime data deletion |
| 10 | Java sidecar progressive migration | Go production cutover by default |

## Highest Current Risks

1. Client session TTL still hard-coded to 30 minutes in `main.go` while `SESSION_TTL` defaults to 4h.
2. TLS cert path ignores `DATA_DIR`; root and `server/certs` both exist.
3. OpenAPI covers only three client endpoints.
4. Local runtime secrets and server binaries are present on disk; ignored but not cleaned.
5. `ARCHITECTURE_V2_PROPOSAL.md` Chinese text is encoding-corrupted.
6. Quality gate configures C++ into shared `build/` without compiling tests/release targets.

## Resume Points

See `project-management/MASTER_EXECUTION_PROGRESS.md` and each `project-management/SESSION_*_PROGRESS.md`.
