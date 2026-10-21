# Agent Instructions

## Read first
- For every task, follow this file and `docs/agents/SAFETY.md`.
- Before changing a subsystem, inspect its implementation and tests; treat README/ADRs as intent, not proof that a feature is complete.
- If instructions conflict or a change could weaken a safety boundary, stop and ask.

## Repository map
- `backend/internal/domain/`: entities, value objects, events, and ports.
- `backend/internal/application/`: command and query use cases.
- `backend/internal/infrastructure/`: PostgreSQL repositories and adapters.
- `backend/internal/api/` and `backend/cmd/api/`: HTTP handlers, middleware, and wiring.
- `backend/migrations/`: PostgreSQL schema and RLS policies.
- `frontend/`: React + TypeScript SPA.
- `docs/adr/`, `docs/openapi.yaml`, `LOCAL_SETUP.md`: architecture decisions, API contract, and setup.

## Safety and domain rules
- Preserve tenant isolation: authorization comes from verified JWT claims; never treat a request body or `X-Tenant-ID` as proof of access.
- Keep PostgreSQL RLS enabled and use tenant-scoped repository connections/transactions; never switch application access to a superuser to work around RLS.
- Store money in integer minor units and tax rates in basis points; do not use floating-point arithmetic.
- Do not bypass invoice state transitions, transactional payment handling, or audit/event writes when changing domain behavior.
- Check `docs/agents/SAFETY.md` before changing authentication, tenant/RLS handling, billing, migrations, outbox workers, or deployment.
- Label placeholder behavior as incomplete; do not describe scaffold endpoints or HTML placeholders as production features.
- Never add secrets, real customer data, or credentials to source, logs, tests, or documentation.

## Commands
| Task | Command |
|------|---------|
| Backend unit tests | `make test` |
| Backend integration tests (Docker/PostgreSQL) | `make test-integration` |
| Backend static analysis | `cd backend && go vet ./...` |
| Frontend install | `cd frontend && npm ci` |
| Frontend typecheck/build | `cd frontend && npm run build` |

## Change discipline
- Keep changes scoped; preserve unrelated working-tree changes.
- Add regression tests for fixes and update `docs/openapi.yaml` for API changes.
- Use forward migrations; run destructive database operations only against a confirmed test database.
- Follow Conventional Commits. AI-created commits must include `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>`.
