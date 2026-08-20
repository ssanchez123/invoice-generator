# ADR-001: Hexagonal Architecture (Ports & Adapters)

## Status
Accepted

## Context
The project's primary goal is to showcase system design and software architecture. We need an architecture that clearly separates domain logic from infrastructure, making the system testable, maintainable, and extensible.

Options considered:
- **Layered (N-tier)** — simple but couples domain to infrastructure
- **Clean Architecture** — similar to hexagonal, more prescriptive
- **Hexagonal** — explicit ports & adapters, domain at center
- **Microservices** — premature for this scope, adds operational complexity

## Decision
Use **Hexagonal Architecture** (Ports & Adapters).

The domain layer defines interfaces (ports) for operations it needs. Infrastructure layer implements those interfaces as adapters. Application layer orchestrates use cases using domain entities and ports.

```
domain/       → entities, value objects, domain events, port interfaces
application/  → use case handlers, command/query DTOs
infrastructure/ → repository implementations, message bus, external APIs
api/          → HTTP controllers, middleware
```

## Consequences
- ✅ Domain logic is pure Go, no framework dependencies
- ✅ Easy to swap implementations (e.g., Postgres → in-memory for tests)
- ✅ Clear boundary between "what" and "how"
- ✅ Each layer is independently testable
- ⚠️ More boilerplate (interfaces + implementations)
- ⚠️ Slight indirection cost for simple operations