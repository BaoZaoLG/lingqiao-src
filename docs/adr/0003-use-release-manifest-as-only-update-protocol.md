# ADR-0003: Use Release Manifest As The Only Update Protocol

## Status
Accepted

## Context
Lingqiao currently has legacy announcement-driven update fields, legacy `updates/index.json`, and the newer release manifest flow. Maintaining all three creates ambiguous behavior.

## Decision
Make signed release manifest the only long-term update protocol. Announcements display messages only. Legacy update data may be imported or read during migration, but new releases must use release records, packages, hashes, signatures, channels, and rollout rules.

## Consequences
Benefits:

- One update path for client and admin.
- Stronger artifact verification.
- Clear version/channel/rollout semantics.

Costs:

- Client update code must be migrated carefully.
- Admin UI must stop treating announcement fields as update controls.
- Legacy data requires an import or read-only compatibility window.
