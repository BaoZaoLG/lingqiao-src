# ADR 0005: Script Single Source of Truth

## Status
Accepted

## Context
The server dynamically provides the `AutoExam.js` script to clients upon session start or script update requests. Historically, it was possible for multiple scripts or modules to be defined in various places without a strict hierarchy. This could lead to conflicting versions of the AutoExam script being pushed or loaded.

## Decision
We will establish a Single Source of Truth (SSOT) for the AutoExam script.

1. **Active Script Pointer:** The `cards` or `scripts` module MUST contain exactly ONE active pointer (e.g., in the database or config) indicating the current `AutoExam.js` version to serve.
2. **Immutability:** Once a script version is uploaded or registered, its contents MUST NOT be modified. New updates require a new version.
3. **Delivery:** The API `GET /api/v1/script` MUST serve only the script referenced by the active script pointer.

## Consequences
- **Positive:** Guaranteed consistency. Clients will not receive divergent script content. Easy rollback by simply updating the active pointer to a previous version.
- **Negative:** Harder to do A/B testing or canary releases for scripts (unless we introduce targeting logic to the SSOT pointer in the future).
