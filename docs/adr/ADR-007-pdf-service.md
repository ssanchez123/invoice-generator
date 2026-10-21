# ADR-007: PDF generation as separate service

## Status
Accepted

## Context
Invoice PDF generation is CPU-intensive (layout, font rendering, potentially QR codes for electronic invoicing). Running it in the main API process can block request handling.

Options considered:
- **In-process (Go template → HTML → PDF)** — blocks API
- **On-demand in API goroutine** — still competes for CPU with request handling
- **Separate microservice** — isolation, independent scaling
- **External SaaS (e.g., DocRaptor)** — adds dependency and cost

## Decision
**Separate Go service (`pdf-service`) that consumes events from the outbox relay.**

The pdf-service:
1. Subscribes to `InvoiceIssued` events via the relay
2. Loads the tenant-scoped invoice, customer, and seller data
3. Renders a real PDF using the pure-Go `gofpdf` library
4. Stores the PDF artifact using a deterministic tenant-scoped key in local storage or S3-compatible object storage
5. Emits `PDFGenerated` only after the artifact has been persisted

Development uses a local shared filesystem volume. S3 integration tests run against an in-process HTTP endpoint, so they need no AWS account, AWS charges, external service, or cloud credentials. Production requires S3-compatible storage and `PDF_STORAGE_BACKEND=s3`; AWS uses the SDK credential chain, while custom endpoints use explicit PDF-scoped credentials. Local filesystem storage is rejected when `ENVIRONMENT=production`.

The API download endpoint verifies the invoice belongs to the authenticated tenant before retrieving the artifact. It returns 404 until asynchronous generation has completed.

## Consequences
- ✅ API service stays responsive (PDF work is offloaded)
- ✅ pdf-service can be scaled independently
- ✅ Can swap PDF engine without touching API
- ✅ Failed PDF jobs retry via outbox without losing data
- ✅ Deterministic object keys make retry writes idempotent
- ✅ S3 protocol tests do not require cloud credentials or external services
- ⚠️ Additional service to deploy and monitor
- ⚠️ The download endpoint returns 404 until asynchronous generation completes