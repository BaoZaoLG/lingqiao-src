# Lingqiao Java Server

Spring Boot implementation of the Lingqiao backend. The production migration
completed on 2026-07-31.

## Production architecture

- Nginx terminates TLS on `443`, `48901`, and `38472`.
- Legacy `/admin/api/*`, `/api/v1/*`, and agent `/api/*` paths are rewritten
  to Java compatibility routes.
- Java is the only business backend and listens on loopback `127.0.0.1:49001`.
- The Go service is disabled and inactive. Its binary and service definition are
  retained only for a time-limited, tested rollback.
- Java owns all reads and writes to the existing SQLite and file stores.
- Flyway remains disabled; the existing production schema is used directly.

## Run

```powershell
cd server-java
..\.tools\apache-maven-3.9.11\bin\mvn spring-boot:run
```

Health:

```text
GET http://127.0.0.1:49001/java/api/health
```

## Test

```powershell
cd server-java
..\.tools\apache-maven-3.9.11\bin\mvn test
```

## Production deployment

Linux deployment assets are under `deploy/`:

- `install.sh` installs the systemd unit and operational scripts without
  overwriting an existing environment file.
- `preflight.sh` checks Java, services, ports, configuration, disk space, and
  SQLite integrity.
- `deploy.sh` installs a checksummed versioned JAR, atomically changes
  `current.jar`, and automatically restores the previous artifact if health
  verification fails.
- `verify.sh` confirms loopback binding, Go availability, every shadow module
  enabled in the environment file, and that every cutover/write flag remains off.
- `rollback.sh` swaps `current.jar` and `previous.jar`, then verifies the restored
  process.
- `configure-go-shadow.sh` adds every Go forwarding flag with a safe `false`
  value, preserving the previous environment file.
- `deploy-go.sh`, `verify-go.sh`, and `rollback-go.sh` manage the cutover-capable
  Go gateway with the same checksum, health-gate, and automatic rollback rules.
- `parity-shadow.sh` runs the shared read-only route contract against Go and Java,
  using a five-minute database session that is removed automatically.
- `backup.sh` creates a consistent, checksummed runtime backup while both writers
  are stopped. `--dry-run` performs only its preflight.
- `cutover-to-java-production.sh` takes a stopped-writer backup, enables all Java
  reads and writes, installs the compatibility gateway, disables Go, and rolls
  the control plane back automatically if verification fails.
- `verify-java-production.sh` verifies service ownership, all feature flags,
  legacy ports, web panels, public APIs, and both databases.
- `rollback-to-go.sh` restores the saved control plane without rewinding data.
- `production-write-smoke.py` validates representative Java writes against a
  disposable consistent copy of production data.
- `parity-production-read.py` and `parity-agent-portal.py` cover dynamic,
  authenticated routes using temporary sessions that are removed automatically.

Production verification:

```bash
/opt/lingqiao-java/bin/verify-java-production.sh
```

## Current Modules

| Module | Status |
| --- | --- |
| common response/error model | implemented |
| `/java/api/health`, `/java/api/cutover` | implemented |
| exam / audit / scripts | read/write migration paths implemented, default off |
| cards / sessions / agents | admin read/write contracts implemented, default off |
| releases / payloads | read/write sidecar contracts implemented, default off |
| chat / blacklist / announcement | read/write contracts implemented, default off |
| client API | activation, heartbeat, download, update, chat and exam report implemented |
| agent portal | auth, profile, dashboard and card operations implemented |
| Go cutover proxy | module-scoped read/write forwarding, default off |
| write ownership | Java owns all production writes |

## Rollback

Java artifact rollback:

```bash
/opt/lingqiao-java/bin/rollback.sh
```

Emergency control-plane rollback:

```bash
/opt/lingqiao-java/bin/rollback-to-go.sh
```

This restores the saved Nginx, Java environment, and systemd configuration,
reenables Go, and retains all data written since cutover.
