# 🔍 Auditoría de Arquitectura y Código — Invoice Generator

**Proyecto:** invoice-generator
**Fecha:** 2026-08-18
**Auditor:** Senior Software Engineer & Architect
**Metodología:** Checklist manual archivo por archivo
**Objetivo:** Evaluar cada aspecto del proyecto desde POV de Ingeniería Senior y Arquitectura

---

## Instrucciones de Uso

Marca cada item con:
- `[x]` — Cumple
- `[ ]` — No cumple / Pendiente / Incompleto
- `[~]` — Cumple parcialmente (nota explicación)

Al final de cada sección, escribe un score: **X/Y** items cumplidos.

---

## 1. Arquitectura y System Design

### 1.1 Separación de Capas

- [ ] El `domain/` no importa nada de `infrastructure/`, `application/`, o `api/`
- [ ] El `application/` no importa nada de `infrastructure/` directamente (solo interfaces del `port/`)
- [ ] El `infrastructure/` implementa las interfaces definidas en `port/`
- [ ] El `api/` depende de `application/` (handlers/commands/queries), no de `infrastructure/` directamente
- [ ] No hay imports circulares entre capas

### 1.2 Puertos y Adapters (Hexagonal)

- [ ] Todos los interfaces están en `domain/port/` (puertos)
- [ ] Los adapters en `infrastructure/repository/` implementan los puertos
- [ ] Existe verificación en compile-time de que los adapters cumplen los puertos (`var _ port.X = (*Y)(nil)`)
- [ ] El domain no referencia drivers concretos (pgx, redis, etc.)
- [ ] Las entidades de dominio no tienen tags de JSON o DB (persistencia agnóstica)

### 1.3 CQRS

- [ ] Los commands (write side) y queries (read side) están en paquetes separados
- [ ] Los commands modifican estado a través del aggregate root
- [ ] Las queries NUNCA modifican estado
- [ ] Las queries retornan DTOs/read models, no entidades de dominio directas
- [ ] No hay acoplamiento entre command handlers y query handlers

### 1.4 Decisiones de Arquitectura (ADRs)

- [ ] Existen ADRs documentadas para cada decisión importante
- [ ] Cada ADR tiene: Status, Context, Decision, Consequences
- [ ] Las ADRs justifican por qué se eligió esta opción sobre las alternativas
- [ ] Las ADRs mencionan trade-offs (pros y contras)
- [ ] Las ADRs son consistentes entre sí (no se contradicen)

### 1.5 Diagramas

- [ ] Existe diagrama de arquitectura high-level
- [ ] Existe diagrama de hexagonal/ports & adapters
- [ ] Existe state diagram del Invoice lifecycle
- [ ] Existe sequence diagram del Outbox pattern
- [ ] Existe diagrama de multi-tenancy con RLS
- [ ] Existe diagrama CQRS flow
- [ ] Existe ERD de la base de datos
- [ ] Los diagramas usan Mermaid (renderizable en GitHub/Markdown)

---

## 2. Domain Layer

### 2.1 Entidades

- [ ] `Invoice` es el aggregate root del billing domain
- [ ] `Invoice` encapsula sus invariantes (no se puede modificar estado internamente)
- [ ] `Invoice` tiene state machine correcta (draft → issued → paid/overdue/cancelled)
- [ ] No se pueden añadir items a una factura issued (solo draft)
- [ ] No se puede cancelar una factura paid o ya cancelled
- [ ] `Issue()` valida que haya items y que issue_date <= due_date
- [ ] `MarkPaid()` solo se puede desde issued o overdue
- [ ] `MarkOverdue()` solo se puede desde issued
- [ ] `Customer` tiene constructor que valida campos requeridos
- [ ] `Payment` es append-only (no tiene método de update)
- [ ] `Subscription` tiene state machine (active → paused → cancelled)
- [ ] `Subscription.AdvanceBillingDate()` maneja daily/weekly/monthly/yearly
- [ ] `Tenant` existe como entidad raíz de multi-tenancy

### 2.2 Value Objects

- [ ] `Money` usa integer minor units (cents) — no floats
- [ ] `Money.Add()` valida que las monedas coincidan
- [ ] `Money.Subtract()` valida que las monedas coincidan
- [ ] `Money.MultiplyBasisPoints()` usa aritmética entera (no floats)
- [ ] `Money.Allocate()` distribuye sin perder pennies (largest remainder method)
- [ ] `Money.String()` formatea correctamente con signo negativo
- [ ] `NewMoney()` valida currency ISO 4217 (3 letras)
- [ ] `TaxRule` tiene effective_from/effective_to para validez temporal
- [ ] `TaxRule.IsActiveAt()` verifica correctamente el rango de fechas
- [ ] `TaxRule.Validate()` valida jurisdiction y rate range
- [ ] `Address.Validate()` valida line1, city, country requeridos
- [ ] `InvoiceNumber` valida no vacío y máximo length
- [ ] `DateRange` valida from < to
- [ ] `TaxResolverPort` está definido como interface en el dominio

### 2.3 Domain Events

- [ ] Los eventos implementan una interface común (`DomainEvent`)
- [ ] Cada evento tiene: EventName, AggregateID, TenantID, OccurredAt
- [ ] `InvoiceIssuedEvent` se emite al transicionar a issued
- [ ] `InvoicePaidEvent` se emite al marcar como paid
- [ ] `PaymentRecordedEvent` se emite al registrar cualquier pago
- [ ] `PDFGeneratedEvent` se emite cuando el PDF está listo
- [ ] `SubscriptionRenewedEvent` se emite al generar factura recurrente
- [ ] Los eventos son inmutables (structs con campos exported pero sin setters)
- [ ] El aggregate colecciona eventos y los expone via `ClearEvents()`
- [ ] `ClearEvents()` retorna los eventos y limpia la colección del aggregate

### 2.4 Puertos (Interfaces)

- [ ] `InvoiceRepository` define: Save, FindByID, FindByNumber, ExistsByNumber, NextInvoiceNumber, FindOverdue
- [ ] `CustomerRepository` define: Save, FindByID, FindByTenant, Delete
- [ ] `PaymentRepository` define: Save, FindByInvoice, FindByID
- [ ] `SubscriptionRepository` define: Save, FindByID, FindDue, UpdateNextBillingDate
- [ ] `TenantRepository` define: FindByID, Save
- [ ] `OutboxRepository` define: SaveEvents, FindUnprocessed, MarkProcessed
- [ ] `IdempotencyRepository` define: Get, Save
- [ ] Los puertos no exponen tipos de infraestructura (no pgx, no sql.DB)

### 2.5 Errores de Dominio

- [ ] Existen sentinel errors para cada invariant violation
- [ ] `ErrInvalidStatusTransition` para transiciones no permitidas
- [ ] `ErrNoItems` para issue sin items
- [ ] `ErrIssueAfterDue` para issue date > due date
- [ ] `ErrItemNotFound` para remove de item inexistente
- [ ] `ErrInsufficientPayment` para pago > total
- [ ] Los errores son exportables y comparables con `errors.Is()`

---

## 3. Application Layer (CQRS)

### 3.1 Commands (Write Side)

- [ ] `CreateInvoiceHandler` valida que el customer exista y pertenezca al tenant
- [ ] `CreateInvoiceHandler` genera el invoice number via repository (no en dominio)
- [ ] `CreateInvoiceHandler` construye el aggregate y lo persiste
- [ ] `IssueInvoiceHandler` carga el invoice, valida tenant, llama `Issue()`, persiste
- [ ] `CancelInvoiceHandler` carga el invoice, valida tenant, llama `Cancel()`, persiste
- [ ] `RecordPaymentHandler` carga el invoice, valida tenant, valida monto <= total
- [ ] `RecordPaymentHandler` calcula si el pago completa la factura → MarkPaid
- [ ] `RecordPaymentHandler` persiste el payment Y el invoice (con eventos) en operaciones separadas
- [ ] Todos los handlers reciben context.Context como primer parámetro
- [ ] Todos los handlers retornan error como último valor de retorno

### 3.2 Queries (Read Side)

- [ ] `ListInvoicesHandler` existe y retorna read models (no entities)
- [ ] `ListInvoicesHandler` NO modifica estado
- [ ] `GetInvoiceHandler` valida tenant ownership
- [ ] `ListCustomersHandler` delega al repository con paginación
- [ ] `ListPaymentsHandler` valida que el invoice pertenezca al tenant antes de listar payments
- [ ] Los query handlers retornan DTOs, no entities del dominio

### 3.3 Errores de Application

- [ ] `ErrCustomerNotInTenant` para customer de otro tenant
- [ ] `ErrInvoiceNotInTenant` para invoice de otro tenant
- [ ] Los errores de aplicación no exponen errores de infraestructura (no sql errors)

### 3.4 Invariantes y Transacciones

- [ ] `RecordPaymentHandler` guarda payment e invoice en la misma transacción o maneja el caso de fallo parcial
- [ ] No hay race condition entre verificar totalPaid y marcar como paid (¿debería ser atómico?)
- [ ] `CreateInvoiceHandler` no genera número de factura si el customer no existe (verifica customer primero)

---

## 4. Infrastructure Layer (Adapters)

### 4.1 Connection Manager (db.go)

- [ ] Usa pgxpool (pool de conexiones, no conexión individual)
- [ ] Pool tiene MaxConns, MinConns configurados
- [ ] `TenantTx()` inicia transacción Y setea `app.tenant_id` en misma operación
- [ ] `TenantTx()` hace rollback si falla el SET LOCAL
- [ ] `TenantConn()` adquiere conexión y setea tenant_id
- [ ] `TenantConn()` libera la conexión si falla el SET
- [ ] Existe `Ping()` para health check
- [ ] `DatabaseURL()` lee de env vars con defaults
- [ ] El password no se loguea en ningún lugar

### 4.2 InvoiceRepository

- [ ] `Save()` usa transacción con TenantTx (RLS activo)
- [ ] `Save()` persiste invoice + items + outbox events + audit en misma TX
- [ ] `Save()` hace rollback en caso de error (defer tx.Rollback)
- [ ] `saveInvoiceTx()` usa ON CONFLICT para upsert (idempotente a nivel DB)
- [ ] `saveInvoiceTx()` borra y re-inserta items (consistencia del aggregate)
- [ ] `saveAuditTx()` escribe entrada inmutable en audit_entries
- [ ] `SaveEventsTx()` escribe eventos al outbox en la misma TX
- [ ] `FindByID()` extrae tenant_id del context para RLS
- [ ] `FindByID()` retorna `ErrNotFound` si no existe (no nil nil)
- [ ] `FindByID()` carga el aggregate completo (header + items)
- [ ] `loadInvoice()` maneja issue_date nullable
- [ ] `loadInvoice()` deserializa metadata JSONB
- [ ] `loadInvoice()` setea currency en items desde el invoice
- [ ] `NextInvoiceNumber()` usa UPSERT en invoice_sequences (atómico, gap-resistant)
- [ ] `NextInvoiceNumber()` formatea como INV-YEAR-SEQ con padding
- [ ] `FindOverdue()` filtra por status='issued' y due_date < asOf
- [ ] Existe `var _ port.InvoiceRepository = (*InvoiceRepository)(nil)` (compile-time check)

### 4.3 CustomerRepository

- [ ] `Save()` usa ON CONFLICT para upsert
- [ ] `Save()` serializa address y metadata a JSONB
- [ ] `FindByID()` extrae tenant_id del context
- [ ] `FindByID()` retorna ErrNotFound si no existe
- [ ] `FindByID()` deserializa address y metadata desde JSONB
- [ ] `FindByTenant()` respeta limit/offset (paginación)
- [ ] `Delete()` es soft delete (SET deleted_at = NOW())
- [ ] `Delete()` retorna ErrNotFound si ya está eliminado
- [ ] Existe compile-time check de interface compliance

### 4.4 PaymentRepository

- [ ] `Save()` es insert-only (no ON CONFLICT, payments son append-only)
- [ ] `FindByInvoice()` ordena por paid_at (cronológico)
- [ ] `FindByID()` retorna ErrNotFound si no existe
- [ ] Existe compile-time check de interface compliance

### 4.5 SubscriptionRepository

- [ ] `Save()` usa ON CONFLICT para upsert
- [ ] `Save()` serializa plan_items a JSONB
- [ ] `FindDue()` usa SELECT FOR UPDATE SKIP LOCKED (multi-worker safe)
- [ ] `FindDue()` NO usa tenant context (cross-tenant worker)
- [ ] `UpdateNextBillingDate()` NO usa tenant context (cross-tenant worker)
- [ ] `FindByID()` deserializa plan_items desde JSONB
- [ ] Existe compile-time check de interface compliance

### 4.6 OutboxRepository

- [ ] `SaveEvents()` usa TenantConn (RLS para escritura)
- [ ] `SaveEventsTx()` función standalone para uso dentro de TX existente
- [ ] `FindUnprocessed()` usa pool.Acquire directo (cross-tenant, sin RLS)
- [ ] `FindUnprocessed()` usa SELECT FOR UPDATE SKIP LOCKED
- [ ] `FindUnprocessed()` respeta limit con clamping (max 100)
- [ ] `MarkProcessed()` actualiza status y processed_at
- [ ] Existe compile-time check de interface compliance

### 4.7 IdempotencyRepository

- [ ] `Get()` verifica que el registro no haya expirado (`expires_at > NOW()`)
- [ ] `Get()` retorna nil (no error) si no existe — permite proceder con la request
- [ ] `Save()` usa ON CONFLICT DO NOTHING (no sobrescribe key existente)
- [ ] `Save()` setea expires_at con TTL
- [ ] Existe compile-time check de interface compliance

### 4.8 TenantRepository y TaxRuleRepository

- [ ] `TenantRepository.FindByID()` NO usa RLS (tenants son la raíz de aislamiento)
- [ ] `TenantRepository.Save()` usa ON CONFLICT para upsert
- [ ] `TaxRuleRepository.Resolve()` busca regla activa por jurisdiction + región + fecha
- [ ] `TaxRuleRepository.Resolve()` maneja region opcional (NULL o valor)
- [ ] `TaxRuleRepository.Resolve()` retorna nil si no hay regla (tax-exempt)
- [ ] Existen compile-time checks para ambos repositorios

### 4.9 In-Memory Test Repositories

- [ ] Implementa los mismos interfaces que los repos reales
- [ ] Thread-safe (usa sync.RWMutex)
- [ ] `Save()` clona el objeto para evitar mutación externa
- [ ] `FindByID()` retorna clon (no referencia al original)
- [ ] `NextInvoiceNumber()` genera secuencia incremental
- [ ] `FindOverdue()` filtra correctamente por status y due_date

---

## 5. API Layer

### 5.1 Routing (main.go)

- [ ] Usa chi router v5
- [ ] Middleware en orden correcto: RequestID → RealIP → Logger → Recoverer → Timeout → CORS → TenantContext → Idempotency
- [ ] CORS configurado con origins, methods, y headers específicos
- [ ] CORS incluye Idempotency-Key en AllowedHeaders
- [ ] Health check en `/health` (sin auth)
- [ ] API v1 bajo `/api/v1` prefix
- [ ] Auth routes (`/auth/login`, `/auth/register`) sin JWT
- [ ] Protected routes con JWTAuth middleware
- [ ] Server tiene ReadTimeout, WriteTimeout, IdleTimeout configurados
- [ ] Graceful shutdown con context timeout (10s)
- [ ] Signal handling (SIGINT, SIGTERM)

### 5.2 Middleware

- [ ] `TenantContext` extrae tenant_id (header en scaffold, JWT en prod)
- [ ] `TenantContext` rechaza con 400 si falta tenant_id en paths protegidos
- [ ] `TenantContext` permite paths públicos sin tenant_id
- [ ] `JWTAuth` valida formato `Bearer <token>`
- [ ] `JWTAuth` rechaza con 401 si falta Authorization header
- [ ] `JWTAuth` rechaza con 401 si formato es inválido
- [ ] `Idempotency` solo aplica a POST
- [ ] `Idempotency` pasa through si no hay key (no es obligatorio)
- [ ] Context keys usan tipo privado (`contextKey`) para evitar colisiones

### 5.3 Handlers

- [ ] `CreateInvoice` parsea body y valida
- [ ] `CreateInvoice` retorna 201 con body
- [ ] `ListInvoices` acepta query params (status, limit, offset)
- [ ] `GetInvoice` usa chi.URLParam para extraer :id
- [ ] `CreateCustomer` valida que name no esté vacío (400)
- [ ] `RecordPayment` parsea body con amount, currency, method
- [ ] `CreateSubscription` parsea frequency, items, next_billing_date
- [ ] `respondJSON()` setea Content-Type application/json
- [ ] `respondError()` usa formato consistente `{"error":"..."}`
- [ ] `parseBody()` cierra el body (defer r.Body.Close)
- [ ] `DownloadPDF` setea Content-Type y Content-Disposition headers

### 5.4 Dependency Injection

- [ ] `HandlerDeps` struct agrupa todas las dependencias
- [ ] `SetDeps()` inyecta las dependencias en runtime
- [ ] main.go construye el grafo de dependencias completo
- [ ] No hay variable global sin inicializar (deps se setea antes de servir)
- [ ] Los handlers referencian deps pero NO lo usan directamente en scaffold (¿está cableado?)

---

## 6. Migraciones y Base de Datos

### 6.1 Schema

- [ ] Todas las tablas tenant-scoped tienen `tenant_id UUID NOT NULL REFERENCES tenants(id)`
- [ ] `invoices` tiene UNIQUE(tenant_id, number) — no duplicados por tenant
- [ ] `users` tiene UNIQUE(tenant_id, email)
- [ ] `invoices` tiene índices en: tenant_id, status, customer_id, due_date
- [ ] `invoice_items` tiene índice en invoice_id
- [ ] `payments` tiene índices en: invoice_id, tenant_id
- [ ] `subscriptions` tiene índice en next_billing_date WHERE status='active'
- [ ] `outbox` tiene índice en created_at WHERE status='pending'
- [ ] `audit_entries` tiene índice en (tenant_id, entity_type, entity_id)
- [ ] `idempotency_records` tiene PK compuesta (key, tenant_id)
- [ ] `idempotency_records` tiene índice en expires_at (para cleanup)
- [ ] `tax_rules` tiene índice en (tenant_id, jurisdiction, region)
- [ ] `invoice_sequences` tiene PK compuesta (tenant_id, year)
- [ ] Las columnas monetarias son BIGINT (no NUMERIC/DECIMAL)
- [ ] `metadata` y `settings` son JSONB (no JSON)
- [ ] Todas las tablas tienen created_at TIMESTAMPTZ
- [ ] Las que tienen updated_at usan TIMESTAMPTZ

### 6.2 Row-Level Security

- [ ] RLS habilitado en todas las tablas tenant-scoped (10 tablas)
- [ ] RLS NO habilitado en `tenants` (son la raíz)
- [ ] RLS NO habilitado en `invoice_sequences` (cross-tenant)
- [ ] Políticas usan `current_setting('app.tenant_id')::UUID`
- [ ] `invoice_items` protegido vía subquery a invoices (no tiene tenant_id directo)
- [ ] Todas las políticas son `FOR ALL` (INSERT, UPDATE, DELETE, SELECT)

### 6.3 Integridad

- [ ] FKs con ON DELETE CASCADE solo en invoice_items → invoices
- [ ] Payments NO tienen cascade (referencia invoice pero si se borra invoice... ¿qué pasa?)
- [ ] Audit entries no tienen FK a ninguna tabla (puede referenciar entidades borradas)
- [ ] Existe migración down (reversible)
- [ ] Las extensiones necesarias están en init-db.sql (uuid-ossp, pgcrypto)

### 6.4 Consideraciones de Producción

- [ ] ¿Existe índice en `outbox(tenant_id)` para el relay worker?
- [ ] ¿Las políticas RLS tienen `WITH CHECK` además de `USING`? (verifica en INSERT/UPDATE)
- [ ] ¿Las contraseñas se hashean con bcrypt/argon2? (campo `password_hash` existe pero no se usa aún)
- [ ] ¿Hay constraint CHECK en `status` de invoices? (sólo comment, no CHECK)

---

## 7. Relay Worker

- [ ] Conecta a la base de datos al iniciar
- [ ] Polla outbox cada 2 segundos
- [ ] Usa FindUnprocessed con SKIP LOCKED
- [ ] Tiene registry de handlers por event_type
- [ ] Si no hay handler para un event_type, marca como processed (no se atasca)
- [ ] Si handler falla, NO marca como processed (reintenta en próximo ciclo)
- [ ] Maneja graceful shutdown (SIGINT/SIGTERM)
- [ ] Usa context con cancel para parar el ticker
- [ ] Loggea errores pero no crashea (continúa procesando)
- [ ] Los handlers hacen unmarshal del payload correctamente

---

## 8. PDF Service

- [ ] Conecta a la base de datos al iniciar
- [ ] Crea directorio de storage si no existe
- [ ] Polla outbox cada 3 segundos
- [ ] Filtra solo eventos `invoice.issued`
- [ ] Renderiza template HTML
- [ ] Guarda artifact en {storagePath}/{tenantID}/{invoiceID}.html
- [ ] Emite evento `pdf.generated` al outbox
- [ ] Maneja graceful shutdown
- [ ] Loggea errores pero no crashea

---

## 9. Docker y Deployment

### 9.1 Docker Compose

- [ ] 5 servicios definidos: postgres, backend, relay-worker, pdf-service, frontend
- [ ] PostgreSQL con healthcheck
- [ ] Backend depende de postgres (condition: healthy)
- [ ] relay-worker depende de postgres
- [ ] pdf-service depende de postgres
- [ ] Frontend depende de backend
- [ ] Volumen para pgdata (persistencia)
- [ ] Volumen para pdfdata (PDFs generados)
- [ ] init-db.sql montado en docker-entrypoint-initdb.d
- [ ] Variables de entorno pasadas a cada servicio
- [ ] Puertos expuestos correctamente (8080, 5173, 5432)

### 9.2 Dockerfiles

- [ ] Backend: multi-stage build (builder → final)
- [ ] Backend: usa alpine como base
- [ ] Backend: instala ca-certificates y tzdata
- [ ] Backend: CGO_ENABLED=0 para static binary
- [ ] Backend: copia migrations al final image
- [ ] Frontend: multi-stage build (builder → nginx)
- [ ] Frontend: npm ci para install determinístico
- [ ] Frontend: nginx sirve el build estático
- [ ] Frontend: nginx proxy /api/ al backend
- [ ] nginx.conf tiene try_files para SPA routing

### 9.3 Seguridad del Deployment

- [ ] ¿Las contraseñas en docker-compose están hardcodeadas? (password visible en compose)
- [ ] ¿JWT_SECRET está hardcodeado en docker-compose?
- [ ] ¿Los contenedores corren como root o usuario non-root?
- [ ] ¿Hay .dockerignore para evitar copiar node_modules, .git, etc.?
- [ ] ¿SSL/TLS está configurado en nginx?

---

## 10. Frontend

### 10.1 Estructura

- [ ] Usa Vite como bundler
- [ ] Usa TypeScript (strict mode en tsconfig.json)
- [ ] Usa React Router para navegación
- [ ] Tiene 3 páginas: InvoiceList, InvoiceDetail, CreateInvoice
- [ ] App.tsx tiene layout con sidebar
- [ ] CSS mínimo pero funcional

### 10.2 Funcionalidad

- [ ] InvoiceList muestra tabla con number, status, dates, total
- [ ] InvoiceList tiene badge de status con colores por estado
- [ ] InvoiceList tiene link "New Invoice"
- [ ] InvoiceList maneja estado vacío (no invoices yet)
- [ ] CreateInvoice tiene form con currency, due_date, line items
- [ ] CreateInvoice permite añadir y remover line items dinámicamente
- [ ] CreateInvoice calcula subtotal en tiempo real
- [ ] InvoiceDetail tiene link de back

### 10.3 API Integration

- [ ] Hay proxy configurado en vite.config.ts (/api → localhost:8080)
- [ ] Los componentes usan fetch para llamar al API (¿o falta implementación?)
- [ ] ¿Manejo de errores de API?
- [ ] ¿Loading states?

---

## 11. Testing

### 11.1 Unit Tests — Domain

- [ ] `money_test.go` cubre: NewMoney, Add, Subtract, Multiply, MultiplyBasisPoints, Allocate, String, Equals, IsNegative, IsZero
- [ ] `tax_address_test.go` cubre: TaxRule.Validate, TaxRule.IsActiveAt, Address.Validate, InvoiceNumber, DateRange
- [ ] `invoice_test.go` cubre: LineTotal, TaxAmount, AddItem, RemoveItem, Issue, Cancel, MarkPaid, MarkOverdue, ApplyPayment, ClearEvents, recalculate
- [ ] `subscription_test.go` cubre: AdvanceBillingDate, Pause, Resume, Cancel
- [ ] Tests cubren casos felices Y casos de error
- [ ] Tests verifican que los domain events se emitan correctamente
- [ ] Tests verifican invariantes de la state machine

### 11.2 Integration Tests — Application

- [ ] `command_test.go` usa in-memory repositories (no necesita PostgreSQL)
- [ ] Test: CreateInvoice exitoso verifica cálculos de subtotal/tax/total
- [ ] Test: CreateInvoice con customer inexistente → error
- [ ] Test: CreateInvoice con customer de otro tenant → error
- [ ] Test: IssueInvoice exitoso verifica status y issue_date
- [ ] Test: CancelInvoice exitoso
- [ ] Test: RecordPayment completo → invoice paid
- [ ] Test: RecordPayment que excede total → error
- [ ] Los tests usan `repository.WithTenantID()` para setear context

### 11.3 E2E Tests — API

- [ ] `handlers_test.go` usa httptest
- [ ] Test: GET /health → 200
- [ ] Test: POST /api/v1/invoices → 201
- [ ] Test: GET /api/v1/invoices → 200
- [ ] Test: POST /api/v1/customers → 201
- [ ] Test: POST /api/v1/customers sin name → 400
- [ ] Test: POST /api/v1/subscriptions → 201
- [ ] Test: POST /api/v1/invoices/:id/payments → 201
- [ ] Test: JSON malformado → 400

### 11.4 Middleware Tests

- [ ] Test: TenantContext setea tenant_id desde header
- [ ] Test: TenantContext rechaza sin tenant_id en path protegido
- [ ] Test: TenantContext permite paths públicos
- [ ] Test: JWTAuth rechaza sin Authorization
- [ ] Test: JWTAuth rechaza formato inválido
- [ ] Test: Idempotency en non-POST pasa through
- [ ] Test: Idempotency en POST sin key pasa through

### 11.5 Config Tests

- [ ] Test: defaults se cargan correctamente
- [ ] Test: env vars se leen correctamente
- [ ] Test: production requiere JWT_SECRET
- [ ] Test: DB_PORT inválido → error

### 11.6 Cobertura y Calidad

- [ ] ¿Cuál es la cobertura total de tests? (ejecutar `go test -cover`)
- [ ] ¿Hay tests del relay worker?
- [ ] ¿Hay tests del PDF service?
- [ ] ¿Hay tests de integración con PostgreSQL real (testcontainers)?
- [ ] ¿Los tests son determinísticos (no dependen de timing)?
- [ ] ¿Los tests pueden correr en paralelo?

---

## 12. Documentación

### 12.1 README

- [ ] Tiene badges de shields.io
- [ ] Tiene tabla de features
- [ ] Tiene diagrama de arquitectura ASCII
- [ ] Tiene tabla de stack tecnológico
- [ ] Tiene estructura del proyecto detallada
- [ ] Tiene Quick Start con Docker Compose
- [ ] Tiene Quick Start manual
- [ ] Tiene comandos de desarrollo completos
- [ ] Tiene referencia de API endpoints
- [ ] Tiene sección de testing con comandos
- [ ] Tiene tabla de ADRs con links
- [ ] Tiene link a diagramas

### 12.2 OpenAPI

- [ ] Versión 3.0.x
- [ ] Info: title, description, version, contact, license
- [ ] Servers: dev y prod
- [ ] Tags organizados por recurso
- [ ] Security scheme BearerAuth definido
- [ ] Todos los endpoints tienen: summary, description, parameters, requestBody, responses
- [ ] Códigos de respuesta documentados (200, 201, 400, 401, 404, 409, 422, 202)
- [ ] Schemas reutilizables con $ref
- [ ] Ejemplos en campos importantes
- [ ] Idempotency-Key documentado como parámetro header

### 12.3 ADRs

- [ ] 10 ADRs documentadas
- [ ] Formato consistente: Status, Context, Decision, Consequences
- [ ] Numeración secuencial
- [ ] Nombres descriptivos en títulos

### 12.4 Diagramas

- [ ] 8+ diagramas en Mermaid
- [ ] Diagrama high-level
- [ ] Diagrama hexagonal
- [ ] State diagram
- [ ] Sequence diagrams (outbox, subscription)
- [ ] ERD completo

---

## 13. Seguridad

### 13.1 Autenticación

- [ ] JWT middleware existe
- [ ] Validación de token implementada (¿o solo scaffold?)
- [ ] Tokens tienen expiry
- [ ] Refresh tokens (¿existen o son necesarios?)

### 13.2 Autorización

- [ ] Roles definidos (admin, member, viewer en schema)
- [ ] Role-based access control implementado (¿o solo schema?)
- [ ] Tenant isolation enforceado por RLS
- [ ] Tenant isolation enforceado por application layer (validación de tenant_id)

### 13.3 Input Validation

- [ ] Handlers validan Content-Type
- [ ] Handlers validan campos requeridos
- [ ] Handlers sanitizan input (¿SQL injection protection? — pgx usa parameterized queries)
- [ ] Money usa integers (no floats) — previene precision bugs
- [ ] UUIDs validados antes de usar en queries

### 13.4 Secrets

- [ ] JWT_SECRET en env var (no hardcoded)
- [ ] DB password en env var
- [ ] Production valida que JWT_SECRET no sea default
- [ ] ¿Secrets en docker-compose visibles? (auditar)

### 13.5 RLS

- [ ] RLS habilitado en 10 tablas
- [ ] Políticas usan current_setting
- [ ] `SET LOCAL app.tenant_id` en cada TX
- [ ] `SET app.tenant_id` en cada connection adquirida
- [ ] ¿Qué pasa si tenant_id es inválido (no existe)? ¿Hay validación?

---

## 14. Observability

### 14.1 Logging

- [ ] API usa chi middleware.Logger (request logging)
- [ ] Relay worker loggea cada batch procesado
- [ ] PDF service loggea cada PDF generado
- [ ] Errores se loggean con contexto
- [ ] ¿Logs estructurados (JSON) o plain text?

### 14.2 Monitoring

- [ ] Health check endpoint (/health)
- [ ] ¿Health check verifica conexión a DB?
- [ ] ¿Métricas expuestas (Prometheus, etc.)?
- [ ] ¿Distributed tracing?
- [ ] ¿Request ID propagado?

### 14.3 Error Handling

- [ ] Recoverer middleware (panic recovery)
- [ ] Errores no exponen stack traces al cliente
- [ ] Errores tienen formato JSON consistente
- [ ] ¿Error codes estandarizados?

---

## 15. Performance y Escalabilidad

### 15.1 Database

- [ ] Índices en columnas usadas para filtrar/ordenar
- [ ] Connection pool configurado (25 max conns)
- [ ] Queries usan parameterized args (no string concatenation)
- [ ] `FindUnprocessed` usa SKIP LOCKED (no bloquea workers)
- [ ] `FindDue` usa SKIP LOCKED (no bloquea billing workers)
- [ ] ¿Hay N+1 queries? (loadInvoice carga items en query separada por invoice)

### 15.2 Concurrency

- [ ] Relay worker usa SKIP LOCKED (múltiples instancias safe)
- [ ] Billing worker puede escalar horizontalmente (SKIP LOCKED)
- [ ] PDF service puede escalar horizontalmente
- [ ] Connection pool compartido entre goroutines

### 15.3 Consideraciones

- [ ] ¿Hay rate limiting en API?
- [ ] ¿Hay caching de queries frecuentes?
- [ ] ¿Outbox table tiene particionamiento o cleanup?
- [ ] ¿Idempotency records tienen cleanup automático?
- [ ] ¿Audit entries crecen indefinidamente?

---

## 16. Code Quality

### 16.1 Go Idioms

- [ ] Errores se manejan explícitamente (if err != nil)
- [ ] No se ignoran errores (salvo con justificación)
- [ ] Context se propaga a todas las operaciones de DB
- [ ] Defer para cleanup (conn.Release, tx.Rollback, rows.Close)
- [ ] Structs exported tienen comentarios de documentación
- [ ] Paquetes tienen comentario de paquete (// Package xxx ...)
- [ ] Nombres siguen convenciones Go (CamelCase, exported = UpperCamelCase)

### 16.2 Consistencia

- [ ] Nombres de archivos consistentes (snake_case)
- [ ] Nombres de paquetes consistentes (lowercase single word)
- [ ] Estilo de errores consistente (sentinel errors vs wrapped errors)
- [ ] Patron de constructores consistente (NewXxx)
- [ ] Patron de repositories consistente (NewXxxRepository)

### 16.3 Technical Debt

- [ ] Handlers son scaffold (no usan deps directamente) — documentado
- [ ] JWT validation es pass-through — documentado
- [ ] Idempotency middleware es pass-through — documentado
- [ ] ListInvoices handler retorna placeholder — documentado
- [ ] PDF service genera HTML (no PDF real) — documentado
- [ ] No hay go.sum (dependencias no resueltas)
- [ ] Frontend no hace fetch real al API
- [ ] No hay .dockerignore

---

## 17. Git y Repository

- [ ] Existe .gitignore
- [ ] .gitignore cubre: node_modules, dist, .env, *.pdf, .DS_Store, IDE files
- [ ] No hay secretos en el repositorio
- [ ] No hay archivos binarios commited
- [ ] Existe LICENSE (MIT)
- [ ] ¿Hay CHANGELOG?
- [ ] ¿Hay CONTRIBUTING.md?

---

## 18. Final Assessment

### 18.1 Puntuación por Sección

| Sección | Items Totales | Items Cumplidos | Score |
|---------|:---:|:---:|:---:|
| 1. Arquitectura y System Design | 28 | | /28 |
| 2. Domain Layer | 44 | | /44 |
| 3. Application Layer (CQRS) | 18 | | /18 |
| 4. Infrastructure Layer | 52 | | /52 |
| 5. API Layer | 26 | | /26 |
| 6. Migraciones y Base de Datos | 28 | | /28 |
| 7. Relay Worker | 10 | | /10 |
| 8. PDF Service | 9 | | /9 |
| 9. Docker y Deployment | 18 | | /18 |
| 10. Frontend | 14 | | /14 |
| 11. Testing | 28 | | /28 |
| 12. Documentación | 18 | | /18 |
| 13. Seguridad | 18 | | /18 |
| 14. Observability | 11 | | /11 |
| 15. Performance y Escalabilidad | 11 | | /11 |
| 16. Code Quality | 16 | | /16 |
| 17. Git y Repository | 7 | | /7 |
| **TOTAL** | **336** | | **/336** |

### 18.2 Calificación Final

| Score % | Grade | Descripción |
|---------|-------|-------------|
| 90-100% | A | Excellent — production-ready |
| 80-89% | B | Good — minor gaps |
| 70-79% | C | Fair — needs work |
| 60-69% | D | Poor — significant gaps |
| <60% | F | Fail — not ready |

### 18.3 Hallazgos Críticos (Critical Findings)

Listar aquí los issues más graves encontrados durante la auditoria:

1. **[CRÍTICO/ALTO/MEDIO/BAJO]** Descripción del hallazgo
2. ...
3. ...

### 18.4 Recomendaciones Prioritarias (Top 5)

1. ...
2. ...
3. ...
4. ...
5. ...

### 18.5 Notas del Auditor

Espacio para observaciones generales, impresiones de la arquitectura, y comentarios sobre la calidad general del proyecto:

...

---

## Auditoría Completada ✅

**Firma del Auditor:** _________________
**Fecha:** 2026-08-18
**Duración:** ___ horas
**Score Final:** ___ / 336 (___ %)
**Grade:** ___