# Lingqiao API Versioning

## Version Families

- Client API: `/api/v1/*`
- Admin API: `/admin/api/*`
- Agent API: `/api/*` on the agent server

## Compatibility Policy

1. Existing `/api/v1/*` fields remain backward compatible during V2 migration.
2. New response fields may be added.
3. Existing fields must not change type.
4. Breaking client API changes require `/api/v2/*`.
5. New update protocol work must stay behind `/api/v1/update/check` until all current clients support the signed manifest response.

## Semantic Versioning

Lingqiao release artifacts use `MAJOR.MINOR.PATCH`.

- `MAJOR`: breaking client/server protocol changes or incompatible data migration.
- `MINOR`: new backward-compatible features.
- `PATCH`: bug fixes, UI fixes, security patches without protocol breakage.
