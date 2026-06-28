# Lingqiao Backup And Restore

## Backup

Stop the server or put it in maintenance mode before backup.

Back up:

- `server/data/`
- release package files under `DATA_DIR/updates/`
- release database if present
- TLS certificates if reused
- release signing keys if stored outside `DATA_DIR`

## Restore

1. Stop the server.
2. Restore `DATA_DIR`.
3. Verify file permissions.
4. Start the server.
5. Open admin panel and verify cards, releases, agents, and audit entries.
6. Run a test client activation and heartbeat.
