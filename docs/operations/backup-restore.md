# Lingqiao Backup And Restore

## Backup

The production deployment stores mutable state under
`/opt/injector-server/data`, TLS material under `/opt/injector-server/certs`, and
service configuration under `/etc/injector-server` and `/etc/lingqiao-java`.
SQLite, JSON compatibility files, payload chunks, scripts, release packages, and
signing material form one rollback unit.

Before a Java write cutover, use the checked-in operational script:

```bash
/opt/lingqiao-java/bin/backup.sh --dry-run
/opt/lingqiao-java/bin/backup.sh --consistent
```

Consistent mode:

1. Records which Go and Java services were active.
2. Stops Java and then Go.
3. Runs SQLite `quick_check` on both databases.
4. Archives data, certificates, environment files, systemd units, and Java artifact
   pointers with mode `0600`.
5. Writes a SHA-256 manifest and verifies the archive can be listed.
6. Restarts Go and Java in that order and verifies both resumed.

Store `/opt/lingqiao-backups/*.tar.gz` and its matching `.sha256` outside the server
before a destructive schema or write-ownership change. The archive contains secrets.

## Restore

1. Verify the archive against its `.sha256` manifest.
2. Disable Go forwarding and stop Java, then stop Go.
3. Preserve the failed state separately for diagnosis.
4. Restore the complete rollback unit, not only `lingqiao.db`.
5. Restore ownership and modes; environment and backup files must remain `0600`.
6. Start Go first and run SQLite `quick_check`.
7. Start Java only in shadow mode and run `preflight.sh --shadow` and
   `verify.sh --shadow`.
8. Open the admin panel and verify cards, releases, agents, audit entries, and
   timestamps.
9. Run a test client activation and heartbeat before re-enabling any forwarding.
