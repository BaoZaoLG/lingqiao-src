# Client Session Controller Design

Date: 2026-07-02

## Goal

Move the first C-side upgrade step from `MainWindow + impl_*.h + workers` toward `UI -> Controller -> Service/API` without changing the existing network protocol, HMAC signing, certificate pinning, local session format, DLL output path, or injection behavior.

The first implementation unit is:

1. Centralize client activation, heartbeat, and deactivate requests through `ApiClient`.
2. Add a stable `ApiError` model and parser.
3. Introduce `SessionController` for activation, restore validation, heartbeat, deactivate, expiry, and pending deactivate coordination.
4. Keep `MainWindow` responsible for UI rendering and legacy state bridging only.

This stage intentionally does not migrate update downloads, chat workflows, DLL download, script fetch, or injection.

## Current State

The client currently has these relevant pieces:

- `client/src/services/api_client.h/.cpp` wraps `HttpPostJson` and `WinHttpGetSigned`, but only exposes generic `postJson` and `getSigned`.
- `client/src/network/workers.h` owns activation and heartbeat workers that call `HttpPostJson` directly.
- `client/src/ui/impl_session.h` owns persistence, restore heartbeat, pending deactivate flushing, reset logic, and direct deactivate/heartbeat HTTP calls.
- `client/src/ui/impl_network.h` owns heartbeat scheduling and heartbeat-failure retry behavior.
- `client/src/ui/impl_actions.h` wires activation worker output into UI state.
- `client/src/controllers/update_controller.h/.cpp` already exists but only checks update manifests; full update download/install is out of scope for this stage.

The worktree also contains unrelated server changes. This design keeps the first implementation scoped to client files and documentation.

## Design Summary

Use `ApiClient` as the only session HTTP entry point. `SessionController` owns the session state machine and async orchestration. `MainWindow` subscribes to controller callbacks and updates widgets, labels, timers, tray state, chat polling, update checks, and DLL preparation exactly as it does today.

Target first-stage flow:

```text
MainWindow button/timer
  -> SessionController
     -> ApiClient activate/heartbeat/deactivate
        -> existing HttpPostJson/WinHttpGetSigned
     -> parse session response or ApiError
  -> MainWindow event handler updates UI
```

`network/workers.h` should stop being the owner of activation and heartbeat after this stage. It can remain in the tree temporarily if other code still includes it, but session UI should not instantiate `ActivateWorker` or `HeartbeatWorker`.

## ApiClient Changes

Add typed request/response structs in `client/src/services/api_client.h`:

- `ApiError`
- `ApiErrorCategory`
- `ApiResult<T>` or a small equivalent typed response wrapper
- `ActivationResponse`
- `HeartbeatResponse`
- `DeactivateResponse`
- `UpdateHint`

Add methods:

- `activate(card, machineId, fingerprint, clientVersion)`
- `heartbeat(sessionToken, machineId, clientVersion)`
- `deactivate(sessionToken, machineId)`

Keep existing generic methods:

- `postJson(path, body)`
- `getSigned(path, sessionToken, machineId, cardCode)`

The generic methods stay available for update, chat, DLL, and transition code until later stages migrate those paths.

`ApiClient` should still use:

- `SERVER_HOST`
- `SERVER_PORT`
- `g_pathAct`
- `g_pathHb`
- `g_pathDeact`
- `HttpPostJson`
- existing request JSON field names, including `client_id`

No protocol-level behavior changes are allowed in this stage.

## ApiError Model

`ApiError` fields:

- `category`: `None`, `Network`, `Http`, `Business`, `Parse`
- `code`: stable machine-readable code when available
- `status`: HTTP status code, or `0` for network failures
- `detail`: user-displayable detail
- `rawMessage`: original transport or server message

Parsing order:

1. If `HttpResponse.error` is non-empty or `statusCode == 0`, return `Network`.
2. If body is valid JSON and contains RFC 9457-style fields, read `code`, `detail`, and `status`.
3. If body is valid legacy JSON, read `detail` or `message`.
4. If HTTP status is non-2xx and no useful body exists, synthesize `HTTP_<status>` with a stable default detail.
5. If a 2xx response body is malformed, return `Parse`.

Compatibility mappings:

- `401` maps to session invalid or signature failure depending on endpoint context.
- `403` maps to business failure. Use `code` when present; otherwise preserve legacy detail text for display.
- `429` maps to rate limit.
- `5xx` maps to server unavailable.
- Version rejection remains compatible with the existing UI by carrying download URL and sha256 when the server includes them.

MainWindow may still display Chinese `detail`, but controller logic should prefer `code`, `status`, and `category`.

## SessionController

Add:

```text
client/src/controllers/session_controller.h
client/src/controllers/session_controller.cpp
```

State enum:

- `inactive`
- `activating`
- `active`
- `heartbeat_failed`
- `expired`
- `deactivated`

Responsibilities:

- Activate a card with machine ID, fingerprint, and client version.
- Persist and restore session data using the existing `QSettings` keys and XOR-with-fingerprint helper behavior.
- Flush pending deactivate tokens using the existing pending token keys.
- Validate restored sessions through heartbeat without blocking the UI thread.
- Run heartbeat requests and track consecutive failures.
- Preserve the current behavior of attempting re-activation after three heartbeat failures when a saved card is available.
- Deactivate the server session on explicit logout or expiry reset.
- Clear environment variable `INJECTOR_SESSION_TOKEN` when the session becomes inactive.
- Emit update hints from activation/heartbeat responses for legacy UI compatibility.

Callbacks/events:

- `onActivated(sessionToken, cardExpiresAt)`
- `onActivationFailed(ApiError)`
- `onHeartbeatOk(cardExpiresAt)`
- `onHeartbeatFailed(ApiError, failureCount)`
- `onSessionExpired(reason)`
- `onDeactivated()`
- `onUpdateHint(UpdateHint)`

The controller can be implemented with Qt signals/slots if it inherits `QObject`; otherwise use `std::function` callbacks. Prefer `QObject` because the surrounding client already uses Qt eventing and queued UI dispatch.

## MainWindow Boundary

MainWindow remains responsible for:

- Reading `m_cardInput`.
- Calling `SessionController::activate`, `tryRestoreSession`, `deactivate`, and heartbeat start/stop methods.
- Updating buttons, labels, tray icon, connection dot, expiry label, and status text.
- Starting/stopping chat polling after controller events.
- Calling existing `downloadDllAsync`, `fetchBalance`, and `CheckEnterpriseUpdateAsync` after activation or restored heartbeat succeeds.
- Keeping `m_activated`, `m_sessionToken`, `m_machineID`, and `m_cardExpiresAt` as transitional UI-facing fields until later stages remove them.

MainWindow should stop owning:

- Direct `HttpPostJson` calls for activation, heartbeat, and deactivate.
- Pending deactivate HTTP flushing.
- Restore heartbeat HTTP request construction.
- Heartbeat failure retry policy.

## Data Flow Details

### Activation

1. UI reads card input and machine fingerprint.
2. `SessionController::activate(card)` sets state to `activating`.
3. Controller calls `ApiClient::activate`.
4. On success, controller stores token, machine ID, card expiry, card code, sets `INJECTOR_SESSION_TOKEN`, saves session, and emits `onActivated`.
5. MainWindow updates UI, starts chat, downloads DLL, fetches balance, checks update, and starts heartbeat.
6. On failure, controller emits `onActivationFailed(ApiError)`; MainWindow restores the activation button and displays the error.

### Restore

1. Controller flushes pending deactivations.
2. Controller reads current `QSettings` keys.
3. If local expiry is already past, controller clears local state and queues/executes deactivate.
4. Otherwise controller performs an async heartbeat.
5. On success, controller emits `onActivated` or a restore-specific success event with the restored token.
6. On failure, controller clears saved session and emits `onSessionExpired`.

### Heartbeat

1. MainWindow or controller timer triggers `heartbeat`.
2. Controller ignores the request if inactive or another heartbeat is in progress.
3. On success, reset failure count and emit `onHeartbeatOk`.
4. On network/server transient failure, increment failure count and emit `onHeartbeatFailed`.
5. After three consecutive transient failures, attempt re-activation using the saved card code, preserving existing behavior.
6. On explicit invalid/expired session, clear local state and emit `onSessionExpired`.

### Deactivate

1. UI calls controller deactivate.
2. Controller sends server deactivate if token and machine ID exist.
3. Network or `5xx` failures preserve the pending deactivate queue behavior.
4. Controller clears local session state and emits `onDeactivated`.

## Testing Strategy

Add focused client tests before production code changes.

If no client test target exists, add a small Qt/Core-compatible test target in `client/src/CMakeLists.txt` that can run without launching the GUI.

First tests:

- `ApiError` parses RFC 9457 `code/detail/status`.
- `ApiError` parses legacy `message/detail`.
- `ApiError` maps network failures and empty HTTP errors.
- Activation response parser extracts `session_token`, `card_expires_at`, and legacy `update`.
- Heartbeat response parser extracts `card_expires_at` and rejects malformed JSON.
- Pending deactivate policy keeps tokens for network or `5xx` failures and removes tokens after client-side success or non-retryable server response.

Build verification after implementation:

```powershell
cmake -S client -B build -G "Visual Studio 17 2022" -A Win32 -DENABLE_HOOK=ON
cmake --build build --config Release
```

Manual verification:

- New card activation succeeds.
- Restart restores session successfully.
- Heartbeat invalid session clears local state.
- Three transient heartbeat failures still attempt re-activation.
- Explicit deactivate clears local session and keeps pending token behavior on retryable network/server failure.
- DLL download and injection button state remain unchanged from the user perspective.

## Rollback Strategy

The implementation should be structured so rollback can be file-local:

1. Keep generic `ApiClient::postJson` and `getSigned`.
2. Keep old UI helper methods until the new controller path is fully wired.
3. Migrate activation first, then restore/deactivate, then heartbeat loop.
4. Do not delete `network/workers.h` in this stage unless reference scans prove it is unused.
5. If a regression appears, route MainWindow back to the old worker/helper path while keeping parser tests and typed `ApiClient` code for investigation.

## Non-Goals

- No service-side API changes.
- No top-level directory migration.
- No update download/install controller migration.
- No chat controller migration.
- No DLL/script service migration.
- No injection controller migration.
- No change to HMAC, certificate pinning, WinHTTP implementation, local card/session keys, or DLL output path.

## Acceptance Criteria

- `client/src/ui/impl_*.h` no longer directly performs activation, heartbeat, or deactivate `HttpPostJson` calls.
- `ApiClient` exposes typed activation, heartbeat, and deactivate methods.
- `ApiError` parsing covers network, HTTP, business, and parse failures.
- `SessionController` owns session state, restore validation, heartbeat failure handling, and pending deactivate flushing.
- Existing activation, restore, heartbeat, deactivate, DLL preparation, and UI enablement behavior is preserved.
- Client tests cover parser and controller-adjacent pure logic.
- Release build succeeds with the existing CMake command.
