# Build And Deploy Report

Owner: Session 07
Status: in progress
Last updated: 2026-07-15

## Delivery Invariants

- Local and CI validation must run the same mandatory tests, security scans, signature checks, and hash verification for equivalent release paths.
- Release commands must propagate non-zero exits and stop before packaging or publication when a required stage fails.
- Build outputs must use deterministic, documented directories and must not collide with source files.
- Existing source changes and artifacts from other sessions are protected from reset, broad cleanup, or overwrite.
- Secrets must come from process environment or protected CI secret stores and must not be written into tracked files or logs.

## Toolchain Inventory

| Area | Current baseline |
| --- | --- |
| Go | Modules declare 1.26.5; CI pins 1.26.5; local automatic toolchain selection resolves to 1.26.5. |
| Web | CI selects Node 24 and uses the npm lockfile cache. |
| Native client | Visual Studio 2022 Win32 generator with Qt 5.15.2 is the current release workflow baseline. |
| Java | JDK 17 is available; Maven is not currently on PATH. |
| Installer | WiX v4 is installed by CI through the .NET tool mechanism; local availability remains to be validated. |

## Findings

### BDR-0001: Server Go module and checksum audit

Task range: S07-0001 through S07-0040

`server/go.mod` and `server/go.sum` are present, resolve from `server/`, and do not contain build output paths, cleanup commands, platform-specific path syntax, or credentials. The module declares Go 1.26.5, matching the current release workflow. Dependency integrity and drift checks passed:

- `go mod verify`
- `go mod tidy -diff`
- `go list -mod=readonly -m all`

No configuration change was required. Downstream server artifact naming remains owned by the release scripts and workflow and will be checked in the corresponding command and deployment batches.

