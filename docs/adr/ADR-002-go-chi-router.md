# ADR-002: Go + chi router for API layer

## Status
Accepted

## Context
Backend language and HTTP framework choice for the invoice generator.

Options considered:
- **Go + net/http** — stdlib only, verbose
- **Go + chi** — lightweight, idiomatic, middleware-friendly
- **Go + Gin** — popular but heavier, some magic
- **Go + Echo** — similar to Gin
- **Python + FastAPI** — fast dev, slower runtime, GIL

## Decision
**Go with chi router.**

Reasoning:
- Go's strong typing and explicit error handling showcase robust backend design
- chi is minimal, composable, stdlib-compatible (`http.Handler` interface)
- No framework lock-in — chi uses standard `http.Handler` and `http.HandlerFunc`
- Excellent for showcase: middleware chains are readable and explicit
- Go's concurrency model (goroutines) useful for event relay worker and PDF service

## Consequences
- ✅ High performance, low memory footprint
- ✅ Single binary deployment per service
- ✅ Strong typing catches errors at compile time
- ✅ chi is ~2k lines, easy to reason about
- ⚠️ More verbose than Python for simple CRUD
- ⚠️ Need to write more boilerplate (validation, serialization)