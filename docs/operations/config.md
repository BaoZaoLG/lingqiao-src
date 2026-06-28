# Lingqiao Configuration

Server configuration should come from environment variables.

| Variable | Purpose |
| --- | --- |
| `PORT` | Admin and client API port. |
| `AGENT_PORT` | Agent panel port. |
| `DATA_DIR` | Runtime JSON/SQLite data directory. |
| `SESSION_TTL` | Client/admin/agent session lifetime. |
| `ADMIN_PASSWORD` | Initial admin password. |
| `HMAC_SECRET` | Client HMAC root secret. |
| `ADMIN_ORIGIN` | Optional admin CORS origin. |
| `AGENT_ORIGIN` | Optional agent CORS origin. |
| `CERT_HOST` | Self-signed certificate host. |

Do not hardcode production secrets in source files.
