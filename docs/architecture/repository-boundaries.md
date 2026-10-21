# Repository Boundaries

Status: active governance baseline  
Last reviewed: 2026-07-15

## Owned Source And Documentation

| Path | Ownership |
| --- | --- |
| `.github/` | CI and release workflow configuration. |
| `client/` | Windows Qt client, injector, Hook runtime, and client tests. |
| `server/` | Go production backend and embedded Web application. |
| `server-java/` | Java compatibility sidecar; not a production replacement by default. |
| `packages/` | Reusable project packages, currently including the AutoExam source package. |
| `e2e_test/` | Cross-process compatibility and end-to-end tests. |
| `installer/` | WiX installer inputs and packaging scripts. |
| `scripts/` | Quality, build, regression, and release orchestration. |
| `docs/` | Architecture, ADR, API, operations, security, handoff, and project-management material. |
| `cef/`, `chromium/` | Native client dependencies required by the current CMake layout. |

## Generated Or Runtime State

| Path / pattern | Classification | Governance rule |
| --- | --- | --- |
| `build*/`, `client/**/Release/` | CMake/MSBuild output | Reproducible output; never use as source of truth. |
| `dist/`, `server/web-dist/` | Release/Web output | Rebuild from source; only `.gitkeep` is tracked in `server/web-dist`. |
| `data/`, `server/data/` | Runtime databases, files, packages, and secrets | Never commit; back up as runtime state. |
| `certs/`, `server/certs/` | TLS certificates and private keys | Never commit; manage as deployment secrets. |
| `*.log`, `server.log` | Runtime logs | Never commit. |
| `server/.gocache/` | Go build cache | Reproducible local cache. |
| `server/injector-server*`, `server/*lingqiao-server*.exe` | Locally built server binaries | Rebuild from Go source; do not treat as repository inputs. |
| `.superpowers/` | Local planning/tool state | Local-only state; not part of the product architecture. |

The ignore rules intentionally do not delete existing files. Existing modified and untracked paths are protected user or parallel-session work until their owner explicitly disposes of them.

## Protected Unclassified Root Artifacts

The initial inventory also found local or externally sourced artifacts such as `management.html`, `mihomo.gz`, proxy configuration/data, patch helpers, `build-status-review/`, and unusually named server executables. Their product ownership is not proven. Session 01 will not delete or move them; later queue tasks must establish references, build use, runtime use, and provenance before classification.

## Build And Runtime Entrypoints

| Entrypoint | Output / effect | Boundary note |
| --- | --- | --- |
| `build.bat` | Win32 Release `CefHook.dll` and `CX_LingQiao.exe` by default | `BUILD_DIR`, `QT_PREFIX_PATH`, `CMAKE_GENERATOR`, and `CMAKE_PLATFORM` are overridable. |
| `scripts/check.ps1` | Web, Go, E2E, vulnerability, and CMake configure gates | It configures C++ in `build/` but does not compile the C++ test or release targets. |
| `server/main.go` | Go admin/client and agent HTTPS servers | Production composition root and route owner. |
| `server/web/package.json` | Vite admin/agent assets | Output is embedded from `server/web-dist`. |
| `server-java/pom.xml` | Java sidecar | Must use a separate port and default to no writes. |
| `packages/autoexam/build.ps1` | AutoExam derived artifacts | `packages/autoexam/src/autoexam.js` remains the source entry. |
| `scripts/release.ps1` | Release artifact orchestration | Must consume verified client/Web/installer outputs. |

## Governance Rules

1. Do not perform a top-level directory migration while module boundaries and parallel sessions are still changing.
2. New production code belongs in the existing owning module; root-level compatibility handlers should shrink over time.
3. Runtime paths must be configurable and kept separate from source paths.
4. Generated artifacts must document their source and regeneration command.
5. Java migration is sidecar-first and cannot silently create a second writer.
6. Deletion requires evidence across source references, build files, tests, dynamic loading, scripts, documentation, and release inputs.
