# ADR-0002: Use Clean And Hexagonal Boundaries

## Status
Accepted

## Context
Current handlers, services, storage, and response formatting are mixed in root-level Go files. This makes testing and future storage/API changes difficult.

## Decision
Use `handler -> service/usecase -> repository -> storage` as the default dependency direction. HTTP, JSON, SQLite, file storage, signing, and CLI are adapters around feature services.

## Consequences
Benefits:

- Business rules can be tested without HTTP.
- Storage can migrate from JSON to SQLite behind repository interfaces.
- API contract changes can be isolated to handlers and response mappers.

Costs:

- Initial refactors add adapter code.
- Some old root-level functions will temporarily call new services until callers are migrated.
