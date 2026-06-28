# Lingqiao Release Runbook

## Preflight

Run from repository root:

```powershell
cmake --build build --config Release --target Injector
cmake --build build --config Release --target CefHook
```

Run from `injector-server`:

```powershell
go test ./...
```

Run from `injector-server/web`:

```powershell
npm run check
```

## Release

1. Build web assets.
2. Build server.
3. Build client and Hook DLL.
4. Build installer.
5. Compute SHA256.
6. Create release record.
7. Upload package.
8. Publish to a limited rollout.
9. Check release events.
10. Expand rollout.
