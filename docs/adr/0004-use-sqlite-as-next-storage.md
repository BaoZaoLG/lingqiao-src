# ADR-0004: Use SQLite As The Next Durable Storage

## Status
Accepted

## Context
Current JSON storage is easy to deploy but fragile for concurrent writes, auditing, pagination, and reporting. The release subsystem already uses SQLite through `modernc.org/sqlite`.

## Decision
Use repository interfaces first, then migrate durable server state from JSON to SQLite module by module. Keep JSON import/read compatibility until migration is complete.

## Consequences
Benefits:

- Single-file deployment remains simple.
- Transactions improve consistency.
- Query-heavy admin screens become easier to maintain.

Costs:

- Requires migration commands and backup/restore documentation.
- Requires repository tests for JSON and SQLite implementations.
