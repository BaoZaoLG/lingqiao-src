# Java Cutover Runbook

Progressive Go → Java ownership transfer. Every step is reversible by changing one
environment variable; no code change or redeploy is needed to roll back.

## Preconditions

- Java sidecar builds and its tests pass: `cd server-java && mvn test`.
- Go builds and its tests pass: `cd server && go test ./...`.
- Both processes read the **same** SQLite database file (`DATA_DIR`).
- Go stays running throughout. It is the rollback path.

## Ports

| Process | Port | Role |
| --- | --- | --- |
| Go admin | `48901` | production entrypoint |
| Go agent portal | `38472` | production entrypoint |
| Java sidecar | `49001` | receives forwarded traffic only |

Java must not bind `48901` or `38472`. Cutover works by Go forwarding to Java, not by
Java taking the port.

## Step 0 — Start Java in shadow mode

Enable a module's read path on the Java side, but leave every cutover flag off. Java is
reachable but no production traffic reaches it.

```bash
JAVA_SERVER_PORT=49001
JAVA_DATA_DIR=/path/to/data                                # same DATA_DIR as Go
JAVA_DB_URL=jdbc:sqlite:file:/path/to/data/lingqiao.db?mode=ro
JAVA_WRITE_DB_URL=jdbc:sqlite:file:/path/to/data/lingqiao.db?mode=ro
JAVA_EXAM_ENABLED=true
JAVA_AUDIT_ENABLED=true
JAVA_SCRIPTS_ENABLED=true
```

The admin token and HMAC secret are read from the shared data directory, so pointing
`JAVA_DATA_DIR` at Go's `DATA_DIR` is what makes both backends accept the same tokens.
Shared module repositories and admin-token validation use `JAVA_WRITE_DB_URL` even
during read-only shadow operation. Keep its SQLite URI in `mode=ro` and keep the
systemd data mount read-only. Before enabling the first Java write flag, take a
consistent backup, remove `mode=ro`, and explicitly grant the service write access.

Confirm:

```bash
curl -s http://127.0.0.1:49001/java/api/health
curl -s http://127.0.0.1:49001/java/api/cutover
```

`health` must report the intended shadow modules as enabled; `cutover` must report
every forwarding flag `false`.

The production systemd templates are kept in `server-java/deploy/`. The service binds
only to loopback, constrains Java memory, and mounts the Go data directory read-only.
Keep `JAVA_CUTOVER_ENABLED=false` throughout shadow verification.

Install the operational bundle once, then deploy only versioned artifacts:

```bash
cd /tmp/lingqiao-java-deploy
./install.sh ./lingqiao-java.env
/opt/lingqiao-java/bin/preflight.sh --shadow
/opt/lingqiao-java/bin/deploy.sh /tmp/lingqiao-server-java.jar <release-id>
/opt/lingqiao-java/bin/verify.sh --shadow
```

`install.sh` preserves an existing `/etc/lingqiao-java/lingqiao-java.env`.
`deploy.sh` records the former `current.jar` as `previous.jar`; a failed restart or
verification automatically restores it.
`verify.sh` derives its expected module list from every
`JAVA_<MODULE>_ENABLED=true` entry, so enabling a new shadow module automatically
adds it to the deployment health gate. `EXPECTED_MODULES` may still be supplied for
an explicit one-off check.

The production Go binary must contain `JAVA_CUTOVER_ENABLED` before any forwarding
flag can work. Install a cutover-capable gateway with all flags disabled:

```bash
/opt/lingqiao-java/bin/configure-go-shadow.sh
/opt/lingqiao-java/bin/deploy-go.sh /tmp/injector-server <release-id>
/opt/lingqiao-java/bin/verify-go.sh --shadow
```

`deploy-go.sh` refuses binaries without the cutover marker and refuses to run if any
forwarding flag is already true. Its rollback command is
`/opt/lingqiao-java/bin/rollback-go.sh`.

## Step 1 — Compare responses before forwarding anything

`server/cmd/paritycheck` issues the same **read-only** request to both backends and diffs
the JSON. It never calls a write route, so it is safe against production.

```bash
cd server
go run ./cmd/paritycheck \
  -go   http://127.0.0.1:48901 \
  -java http://127.0.0.1:49001 \
  -token "$ADMIN_TOKEN" \
  -manifest ../docs/api/java-cutover-contracts.json \
  -module exam
```

Exit code 0 and `mismatched=0` means the module is safe to forward. Investigate any
`DIFF` line before continuing — a difference here becomes a user-visible bug after
cutover.

On production, the packaged command creates and removes its own five-minute
administrator session, so no password or raw token appears in shell history:

```bash
/opt/lingqiao-java/bin/parity-shadow.sh exam audit scripts
```

Useful flags:

- `-module <name>` — compare one module at a time.
- `-query "page=1&per_page=20"` — exercise pagination.
- `-ignore public_key` — skip a field that is legitimately environment-specific.

## Step 2 — Forward one read module

```bash
JAVA_CUTOVER_ENABLED=true
JAVA_CUTOVER_EXAM=true
```

Restart Go. `GET /admin/api/exam/records` is now served by Java; everything else is still
Go. Verify through the real admin panel, not only curl.

## Step 3 — Recommended module order

Read paths first, in increasing blast radius:

1. `EXAM`, `AUDIT` — reporting only, no client impact
2. `SCRIPTS` — file-backed, read-only
3. `CARDS`, `SESSIONS` — core admin data
4. `AGENTS` — admin data plus card aggregation
5. `CHAT`, `BLACKLIST`, `ANNOUNCEMENT` — file-backed state
6. `RELEASES`, `PAYLOADS` — only after golden vectors verified
7. `AGENT_PORTAL` — separate port, verify with `-go http://127.0.0.1:38472`
8. `ADMIN` — login, password, overview, administrator and invite control plane last

Write paths only after the module's read path has run clean for at least one full
business day:

```bash
JAVA_CARDS_WRITE_ENABLED=true          # Java side
JAVA_CUTOVER_CARDS_WRITE=true          # Go forwarding
```

Before removing SQLite `mode=ro` or granting Java write ownership:

```bash
/opt/lingqiao-java/bin/backup.sh --dry-run
/opt/lingqiao-java/bin/backup.sh --consistent
```

Record the printed archive and SHA-256 manifest in the change ticket. Do not continue
if either database fails `quick_check` or either service fails to resume.

For the final admin control-plane step, enable both Java controller families before
forwarding:

```bash
JAVA_ADMIN_AUTH_ENABLED=true
JAVA_ADMIN_OVERVIEW_ENABLED=true
JAVA_CUTOVER_ADMIN=true
JAVA_CUTOVER_ADMIN_WRITE=true
```

Client routes also require the matching Java controller family:

| Routes | Java side |
| --- | --- |
| activate / heartbeat / deactivate | `JAVA_CLIENT_WRITE_ENABLED=true` |
| DLL / script / payload / key exchange | `JAVA_CLIENT_DOWNLOAD_ENABLED=true` |
| update check / events / package | `JAVA_CLIENT_UPDATE_ENABLED=true` |
| client chat | `JAVA_CLIENT_CHAT_ENABLED=true` |
| client announcement | `JAVA_CLIENT_ANNOUNCEMENT_ENABLED=true` |

The corresponding module cutover flag still controls forwarding; state-changing client
routes additionally require that module's `JAVA_CUTOVER_<MODULE>_WRITE=true`.

## Step 4 — What to check after each flip

- Admin panel page loads and shows the same rows as before.
- `Cache-Control` on login responses is still `no-store`.
- Timestamps render as real dates, not 1970. A 1970 date means a timestamp format
  regression; roll back immediately.
- Audit entries still appear for write actions.

## Rollback

There are two independent rollback layers:

1. **Traffic rollback** — disable the module or global Go forwarding flag and restart
   Go. This returns requests to Go without changing the Java artifact.
2. **Artifact rollback** — run `/opt/lingqiao-java/bin/rollback.sh`. It atomically
   swaps `current.jar` and `previous.jar`, restarts Java, and verifies shadow safety.

```bash
JAVA_CUTOVER_<MODULE>=false
```

Restart Go. Traffic returns to the Go handler. Java can keep running for diagnosis.

Full stop:

```bash
JAVA_CUTOVER_ENABLED=false
```

Never use an artifact rollback as a substitute for traffic rollback when production
requests are already being forwarded.

## Intentional exclusions

- `/admin/api/audit` has no HTTP write endpoint in either backend; Java writes audit
  events through the shared service layer.
- Health and static web assets remain local to each process rather than being proxied.

## Retiring Go

Only after every module (read and write) has been forwarded and stable. That step changes
the process topology rather than a flag, so it needs its own plan and is deliberately not
part of this runbook.
