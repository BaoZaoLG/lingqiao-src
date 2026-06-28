# ADR-0001: Use Modular Monolith For V2 Server Architecture

## Status
Accepted

## Context
Lingqiao currently ships as a tightly coupled Windows client, Hook DLL, Go server, web admin, agent panel, AutoExam script, and release pipeline. The server has grown beyond a tool-style codebase, but the deployment and team size do not justify microservices.

## Decision
Keep the Go backend as a modular monolith. Split by business feature under `internal/features`, keep shared infrastructure under `internal/platform`, and keep one deployable server process.

## Consequences
Benefits:

- Deployment remains simple.
- Feature boundaries become clear.
- Tests can target services without booting the full server.
- A future service split remains possible after boundaries are proven.

Costs:

- Code review must enforce boundaries.
- Shared state must be moved behind repository interfaces gradually.
- Top-level directory migration must wait until feature boundaries are stable.
