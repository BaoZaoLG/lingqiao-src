# Lingqiao Server

## Frontend Build (Required First)

The modern admin and agent panels live in `web/` and are built with Vite + TypeScript.
Because the Go server uses `go:embed` to serve the static frontend, you MUST build the web assets before running or testing the Go server.

```powershell
cd web
npm install
npm run check
```

The build output is written to `web-dist/` and embedded by the Go server. Admin is served from `/admin/`, agent is served from `/`, and shared assets are served from `/assets/`.

## Test

PowerShell:

```powershell
$env:GOCACHE="$PWD\.gocache"
go test ./...
```

## Run

```powershell
$env:PORT='48901'
$env:AGENT_PORT='38472'
$env:DATA_DIR='data'
go run .
```

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `PORT` | `48901` | Admin and client API HTTPS port |
| `AGENT_PORT` | `38472` | Agent panel HTTPS port |
| `DATA_DIR` | `data` | Persistence directory (holds `lingqiao.db` and old `data.json`) |
| `SESSION_TTL` | `4h` | Default platform session TTL for new internal services |
| `ADMIN_PASSWORD` | generated | Initial admin password when no password hash exists |
| `HMAC_SECRET` | generated | Client API HMAC secret |
| `ADMIN_ORIGIN` | empty | Optional allowed admin CORS origin |
| `AGENT_ORIGIN` | empty | Optional allowed agent CORS origin |
| `CERT_HOST` | `127.0.0.1` | Hostname/IP used for generated self-signed TLS certificates |

## Notes

The backend has been migrated to use **SQLite** as the default persistence engine (`data/lingqiao.db`). Legacy `data.json` files will be automatically imported upon first startup if the SQLite database does not exist.

Credentialed admin and agent CORS defaults to same-origin only. Set `ADMIN_ORIGIN` or `AGENT_ORIGIN` only when a separate trusted browser origin is required.
