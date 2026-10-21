# Java Production Cutover Plan

## Goal

Enable progressive production ownership transfer from Go to Java without a big-bang rewrite.

## Current State

- Go remains the production entrypoint (`server/main.go`).
- Java runs as sidecar on `127.0.0.1:49001` by default.
- Feature modules are flag-gated under `lingqiao.sidecar.*`.
- Cutover flags are available under `lingqiao.cutover.*` and default to `false`.

## Cutover Order

1. health/audit/exam read
2. scripts read
3. cards/sessions admin read
4. agents admin read
5. releases/payloads only after golden-vector verification
6. write paths only after dual-run parity is green

## Operator Procedure

1. Start Java sidecar.
2. Enable module flags (`JAVA_*_ENABLED=true`) for shadow/read paths.
3. Verify `/java/api/health` and `/java/api/cutover`.
4. Compare Go/Java responses for the same admin query.
5. Flip only one `JAVA_CUTOVER_*` flag at a time via reverse proxy/gateway.
6. Keep Go online for immediate rollback.

## Rollback

1. Set the corresponding `JAVA_CUTOVER_*` flag back to `false`.
2. Route traffic back to Go.
3. Leave Java process running for diagnosis or stop it.

## Non-Goals For This Stage

- Replacing Go process lifecycle
- Automatic write dual-commit
- Silent schema ownership transfer via Flyway
