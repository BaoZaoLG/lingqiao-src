# Generated And Dependency Tree Classification

Status: active  
Last reviewed: 2026-07-17  
Covers Session 01 tasks that enumerate CMake/MSBuild output trees and pinned native dependency trees.

## Rule

Any path matching the classes below is **not product source of truth**. Session 01 classifies and documents it. Session 01 does **not** delete, move, or rewrite these trees because they may be active local caches or protected parallel-session artifacts.

## Generated Build Classes

| Class | Path patterns | Owners / producers | Consumers |
| --- | --- | --- | --- |
| Primary local CMake tree | `build/**` | `build.bat`, `scripts/check.ps1` configure, local VS/CMake | Local developer builds; risk: quality gate mutates shared cache |
| Alternate local CMake tree | `build64/**` | Manual x64 or alternate local configures | Local only; not referenced by release workflow |
| Status/review CMake tree | `build-status-review/**` | Local review/configure experiments | Local only; candidate for later optional cleanup by Session 09 with proof |
| Nested client CMake tree | `client/build/**` | In-tree CMake configure mistakes or IDE defaults | Local only; preferred roots are repo-level `build` or CI `build-release` |
| CI/release isolated tree | `build-release/**` | `.github/workflows/release.yml` | Installer packaging inputs |

Common node kinds under those trees: `CMakeFiles/`, `*.dir/`, `*_autogen/`, `Release|Debug|MinSizeRel|RelWithDebInfo/`, `Win32/`, `*.vcxproj`, `*.sln`, `*.tlog`, compiler-id probes, Qt deploy folders (`platforms`, `imageformats`, `styles`, `iconengines`).

Ignore coverage: root `.gitignore` already excludes `build*/`, CMake intermediates, and many binary extensions.

## Native Dependency Classes

| Path | Classification | Notes |
| --- | --- | --- |
| `cef/**` | Third-party native headers/deps for CefHook | Included from `client` CMake via `../cef`. Do not edit upstream CEF headers for product features. |
| `cef/include/**` | CEF public/capi/internal/views/wrapper headers | Compile-time dependency surface only. |
| `chromium/**`, `chromium/net/**` | Chromium fragment headers used by current CMake include path | Treat as vendored dependency, not app business logic. |
| `certs/**` | Runtime TLS material | Already classified under runtime secrets; not generated source. |

## Explicit Non-Source Local Tool Files Already Covered

- `.superpowers/**` brainstorm HTML/state
- `.vscode/settings.json` local hardcoded cmake source path
- Root/server local binaries and path-flattened executable names

## Verification Used For Bulk Classification

1. Path inventory existence checks for `build`, `build64`, `build-status-review`, `client/build`, `cef`, `chromium`.
2. `.gitignore` pattern review (`build*/`, CMake artifacts).
3. Script/workflow reference review:
   - local release defaults to `build\src\Release`
   - CI release uses isolated `build-release`
   - no product script treats `build-status-review` as input
4. CMake include references confirm `cef` and `chromium` are dependency roots.

## Task Closure Policy For Tree Enumeration Items

For each enumerated subdirectory/file under the generated classes:

- Ownership: local generated cache or vendored dependency
- Risk: pollution of source mental model; accidental commit; stale binary reuse
- Action: keep ignored; document; do not delete in Session 01
- Validation: classification against ignore rules and producer/consumer map above

This policy closes the Session 01 tree-walk tasks without claiming the files were removed.
