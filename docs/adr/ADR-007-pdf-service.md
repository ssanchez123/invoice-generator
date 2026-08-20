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
2. Renders PDF using Go templates + a PDF library (`go-pdf` or `chromedp` for HTML→PDF)
3. Stores the PDF artifact (local filesystem for dev, S3-compatible for prod)
4. Emits `PDFGenerated` event back to outbox

The API service can also request PDF generation synchronously via internal HTTP call when the user clicks "Download PDF" and no cached version exists.

## Consequences
- ✅ API service stays responsive (PDF work is offloaded)
- ✅ pdf-service can be scaled independently
- ✅ Can swap PDF engine without touching API
- ✅ Failed PDF jobs retry via outbox without losing data
- ⚠️ Additional service to deploy and monitor
- ⚠️ Slight latency on first PDF request (async generation)