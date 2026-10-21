# Java Migration Progress

## Status

Go remains the production process and rollback owner. Java runs as a sidecar and can
receive selected admin / agent-portal / client routes through Go's cutover proxy. All
cutover flags default to `false`.

## Module Coverage

| Module | Read | Write | Read cutover | Write cutover |
| --- | --- | --- | --- | --- |
| exam | yes (records, stats, CSV export) | yes (HMAC report + session-only collect) | yes | yes |
| cards | yes (effective status, filters, CSV export) | yes (bulk + CSV import) | yes | yes |
| authorization control plane | yes (lifecycle + active sessions) | yes (preview + single/bulk actions) | yes | yes |
| blacklist | yes | yes | yes | yes |
| sessions | yes | yes | yes | yes |
| agents | yes (metrics CSV, machine/card ownership, Go ordering) | yes | yes | yes |
| scripts | yes | yes | yes | yes |
| announcement | yes | yes | yes | yes |
| chat | yes | yes | yes | yes |
| audit | yes (JSON + filtered CSV) | service layer | yes | not exposed |
| releases | yes (packages + event metrics + public key + client update/download) | yes | yes | yes |
| payloads | yes | yes | yes | yes |
| admin auth / users / invites / dashboard | yes | yes | yes | yes |
| client API (19 business routes) | yes | yes | per-module | per-module |
| agent portal | yes | yes (login, register, password, card generation) | yes | yes |
| web panels (`admin` / `agent` / `assets`) | yes | n/a | not proxied | n/a |

Shared route contract: `docs/api/java-cutover-contracts.json` (85 routes with permissions/auth class, validated by
both Go and Java tests).

## Cross-language Golden Vectors

| Vector | File | Status |
| --- | --- | --- |
| Card code alphabet / format / validation | `docs/api/cardcode-golden-vectors.json` | byte-identical |
| Release manifest Ed25519 signature | `docs/api/release-manifest-golden-vectors.json` | byte-identical |
| Payload key exchange + DLL cipher | `docs/api/payload-crypto-golden-vectors.json` | byte-identical |
| Manifest HMAC | `docs/api/manifest-hmac-golden-vectors.json` | byte-identical |
| DLL transport framing | `docs/api/dll-transport-golden-vectors.json` | byte-identical |
| Release rollout bucketing | `docs/api/release-rollout-golden-vectors.json` | byte-identical |
| Chat identity / nickname rules | `docs/api/chat-identity-golden-vectors.json` | byte-identical |

## Security Posture

- Every business controller authenticates (`requireAdmin` with a permission code, HMAC +
  session or active session token for client ingestion, or `auth.requireSession` for the agent portal).
- `ControllerAuthCoverageTest` fails the build if a new controller skips authentication.
  Allow-list is 4 entries: health, cutover, client announcement, web assets.
- Agent portal reuses Go's session model: `agent_` token prefix, SHA-256 token index,
  shared `agent_sessions.json` / `agent_token_map.json` persistence,
  bcrypt-with-legacy-upgrade password check, 5-per-15min login and 3-per-hour
  registration/password-change rate limits, disabled-agent rejection on every request.
- Agent self-registration consumes the invite code and inserts the agent in one
  transaction, so an exhausted or invalid invite can never leave an orphan account.
- File-backed modules normalize paths and reject traversal (`..`, `\`, NUL).
- Write modules use a separate writable datasource that is only created when a write
  flag is enabled.

## Verification

- Go: `GOTOOLCHAIN=local go test ./...` — pass.
- Java: `mvn test` — `Tests run: 313, Failures: 0, Errors: 0`.
- E2E: `GOTOOLCHAIN=local go test ./...` in `e2e_test` — pass.

## Operator Procedure

1. Start the Java sidecar on `127.0.0.1:49001`.
2. Enable a module (`JAVA_<MODULE>_ENABLED=true`).
3. For writes, also enable `JAVA_<MODULE>_WRITE_ENABLED=true`.
4. Enable Go cutover (`JAVA_CUTOVER_ENABLED=true`).
5. Enable one module cutover (`JAVA_CUTOVER_<MODULE>=true`), and for writes
   `JAVA_CUTOVER_<MODULE>_WRITE=true`.
6. Verify `/java/api/health` and `/java/api/cutover`, then compare responses with Go.
7. Roll back by clearing the module cutover flag.

Agent portal specifically: `JAVA_AGENT_PORTAL_ENABLED=true`, then
`JAVA_CUTOVER_AGENT_PORTAL=true` (dashboard, info, card list) and
`JAVA_CUTOVER_AGENT_PORTAL_WRITE=true` (login, register, password change, single and
batch card generation).

## Web Panel Hosting

Go embeds `web-dist` into the binary. Java serves the same tree from disk via
`JAVA_WEB_DIST_DIR` (default `../server/web-dist`), so the frontend can be updated
without repackaging the JAR — at the cost of shipping the directory alongside the JAR.
Panels are unauthenticated by design, matching Go; they authenticate through the API.

## Timestamp Compatibility

SQLite `DATETIME` columns are the main cross-language hazard, in both directions.

Reads: Go writes RFC3339 text, but the JDBC driver only parses its own
`yyyy-MM-dd HH:mm:ss.SSS` form and throws on anything else. Every Java read path goes
through `common/jdbc/TimestampReader`, which tries the driver first and falls back to
`Instant.parse`.

Writes: binding a `java.sql.Timestamp` makes the driver store **epoch millis**
(`1782909296000`). Go's `parseDBTime` / `parseTime` only try text layouts, so such a row
decodes as the zero time — a silent data corruption, not an error. Every Java write path
goes through `common/jdbc/TimestampWriter.format`, which emits `time.RFC3339Nano` exactly
like Go. `TimestampWriterUsageTest` fails the build if production code binds a
`Timestamp` or calls `setTimestamp`.

## Cutover Tooling

`server/cmd/paritycheck` diffs Go and Java responses for the same read-only route,
driven by the shared manifest. It never calls a write route, so it is safe to run against
production. `docs/project-management/JAVA_CUTOVER_RUNBOOK.md` is the step-by-step
procedure; it is the document to follow when enabling a module.

## Permission Parity

Go registers each admin route with an explicit permission code, and several are
counter-intuitive: `GET /admin/api/releases`, `/admin/api/scripts` and
`/admin/api/payloads` are all guarded by `*.write`, and `/admin/api/blacklist` uses
`system.read` for both GET and POST. The Java sidecar must check the same code — using
the "obvious" read permission would let `operator` reach release data it cannot see in
Go today.

The shared manifest (`docs/api/java-cutover-contracts.json`, version 14) now carries a
`permission` field per admin route. `TestManifestPermissionsMatchGoRoutes` pins it
against Go's registrations and `PermissionParityTest` pins it against the Java
controllers, so a drift on either side fails the build.

## Response Contract

The admin and agent panels are shared between both backends, so Java responses must
carry exactly the fields the Go handlers send — no extras. Eight controllers used to add
a `read_only: true` marker that exists in no Go response; it has been removed, and
`ResponseFieldDriftTest` fails the build if it comes back.

## Guardrails Still In Force

- Do not replace `server/main.go`.
- Java does not bind Go production ports by default.
- Go auth/permission middleware runs before any forwarding.
- Read routes accept only `GET`/`HEAD`; write routes accept only their declared method.
- No Flyway schema takeover; no dual-write.

## Known Gaps

- Audit has no HTTP write endpoint by design; it is a service-layer dependency only.
- Java has not yet been run as the sole listener on `48901`/`38472`; that step needs a
  dual-run parity drill against live traffic before Go can be retired.
