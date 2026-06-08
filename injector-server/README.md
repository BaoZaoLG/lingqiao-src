# Lingqiao Server

## Test

PowerShell:

```powershell
$env:GOCACHE='C:\Users\Li\Downloads\Lingqiao_src\_src_tmp\injector-server\.gocache'
go test ./...
```

## Run

```powershell
$env:PORT='48901'
$env:AGENT_PORT='38472'
$env:DATA_DIR='data'
go run .
```

## Frontend

The modern admin and agent panels live in `web/` and are built with Vite + TypeScript.

```powershell
cd web
npm install
npm run typecheck
npm run build
```

The build output is written to `web-dist/` and embedded by the Go server. Admin is served from `/admin/`, agent is served from `/`, and shared assets are served from `/assets/`.

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `PORT` | `48901` | Admin and client API HTTPS port |
| `AGENT_PORT` | `38472` | Agent panel HTTPS port |
| `DATA_DIR` | `data` | JSON persistence directory |
| `SESSION_TTL` | `4h` | Default platform session TTL for new internal services |
| `ADMIN_PASSWORD` | generated | Initial admin password when no password hash exists |
| `HMAC_SECRET` | generated | Client API HMAC secret |
| `ADMIN_ORIGIN` | empty | Optional allowed admin CORS origin |
| `AGENT_ORIGIN` | empty | Optional allowed agent CORS origin |
| `CERT_HOST` | `127.0.0.1` | Hostname/IP used for generated self-signed TLS certificates |

## Notes

Existing JSON data remains the persistence format for the platform foundation phase. Keep `data/` backed up before migration work.

Credentialed admin and agent CORS defaults to same-origin only. Set `ADMIN_ORIGIN` or `AGENT_ORIGIN` only when a separate trusted browser origin is required.

The backend has been split into internal services for config, storage, auth sessions, audit recording, cards, agents, and update metadata. The admin and agent panels now have Vite/TypeScript source under `web/`, with compiled assets embedded from `web-dist/`.
