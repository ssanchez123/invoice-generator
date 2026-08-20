# Contributing to Invoice Generator

Thanks for your interest in contributing! This project is a showcase of backend system design, and contributions that improve architecture, testing, documentation, or add features are all welcome.

## Getting Started

1. **Fork** the repository
2. **Clone** your fork: `git clone https://github.com/<your-username>/invoice-generator.git`
3. **Create a branch**: `git checkout -b feat/my-feature`
4. **Make changes** following the conventions below
5. **Test**: `cd backend && go test ./...`
6. **Commit** using [conventional commits](https://www.conventionalcommits.org/)
7. **Push** and open a Pull Request

## Development Setup

```bash
# Start all services
docker compose -f deploy/docker/docker-compose.yml up -d

# Or run locally — see LOCAL_SETUP.md
```

## Architecture Conventions

### Hexagonal Architecture

- **Domain layer** (`internal/domain/`) has **zero infrastructure imports**. It only depends on standard library.
- **Application layer** (`internal/application/`) depends on domain ports, not infrastructure.
- **Infrastructure layer** (`internal/infrastructure/`) implements domain ports.
- **API layer** (`internal/api/`) wires everything together.

### CQRS

- **Commands** (`internal/application/command/`) mutate state. They never return data (except IDs).
- **Queries** (`internal/application/query/`) read state. They never mutate.

### Key Rules

- Never import infrastructure packages from domain code
- Use context to propagate tenant ID — never pass it as a parameter to domain methods
- All money values are in **integer minor units** (cents), never floats
- Tax rates are in **basis points** (1900 = 19%)
- Soft-delete everything — never `DELETE` rows
- Every state-changing operation writes an audit entry
- Every state-changing operation emits domain events to the outbox

## Commit Conventions

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(scope): description
fix(scope): description
docs(scope): description
test(scope): description
refactor(scope): description
chore(scope): description
```

Examples:
```
feat(domain): add invoice cancellation with state machine
fix(infrastructure): use set_config() for pgx RLS parameterized SET
docs(api): update OpenAPI spec with payment endpoints
test(application): add integration tests for record payment
```

## Testing

- **Unit tests** for all domain logic
- **Integration tests** for application handlers using in-memory repos
- **E2E tests** for HTTP handlers using `httptest`
- New features must include tests
- Bug fixes should include a regression test

```bash
cd backend && go test ./... -v
```

## Pull Request Checklist

- [ ] Tests pass: `go test ./...`
- [ ] Code follows architecture conventions (no domain → infra imports)
- [ ] Commit messages follow conventional commits
- [ ] If adding a feature, update relevant ADRs or add a new one
- [ ] If changing API, update `docs/openapi.yaml`
- [ ] No hardcoded secrets or credentials

## Reporting Issues

When reporting issues, include:
1. Steps to reproduce
2. Expected vs actual behavior
3. Environment (Docker Compose vs local, OS, Go version)
4. Relevant logs

## License

By contributing, you agree that your contributions are licensed under the MIT License.