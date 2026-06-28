# Update Protocol Migration

## Current Paths

1. Legacy announcement update fields:
   - `latest_version`
   - `min_version`
   - `force_update`
   - `download_url`
   - `sha256`

2. Legacy update package index:
   - `DATA_DIR/updates/index.json`
   - `/admin/api/update/*`

3. V2 release manifest:
   - `/api/v1/update/check`
   - `/api/v1/update/package/{id}`
   - `/admin/api/releases/*`

## V2 Decision

Signed release manifest is the only long-term update protocol. Announcement content must not carry update protocol fields after migration.

## Compatibility Window

During migration:

- Existing legacy update data may be imported into releases.
- Legacy admin update routes remain read-only or hidden.
- Client keeps fallback only until all supported client versions understand signed manifests.

## Removal Criteria

Legacy update fields can be removed when:

1. The minimum supported client version uses `/api/v1/update/check`.
2. Admin UI release page fully supports upload, publish, pause, rollback, and rollout.
3. Release metrics are visible enough to diagnose failed upgrades.
4. A production backup exists before migration.
