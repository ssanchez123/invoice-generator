# Guía de Ejecución Local — Invoice Generator

Instrucciones paso a paso para levantar todo el proyecto en local: PostgreSQL, migraciones, backend (API + relay-worker + pdf-service) y frontend.

---

## Tabla de contenidos

1. [Requisitos previos](#1-requisitos-previos)
2. [Opción A — Docker Compose (todo en uno)](#2-opción-a--docker-compose-todo-en-uno)
3. [Opción B — Ejecución manual (componente por componente)](#3-opción-b--ejecución-manual-componente-por-componente)
4. [Verificación](#4-verificación)
5. [Compilación / Build de producción](#5-compilación--build-de-producción)
6. [Ejecutar tests](#6-ejecutar-tests)
7. [Estructura de servicios](#7-estructura-de-servicios)
8. [Variables de entorno](#8-variables-de-entorno)
9. [Solución de problemas](#9-solución-de-problemas)

---

## 1. Requisitos previos

| Herramienta | Versión mínima | Para qué sirve |
|-------------|---------------|----------------|
| **Docker** | 24+ | Contenedor de PostgreSQL y builds |
| **Docker Compose** | v2+ | Orquestación de servicios |
| **Go** | 1.22+ | Compilar y correr el backend |
| **Node.js** | 20+ | Compilar y correr el frontend |
| **npm** | 10+ | Gestión de dependencias del frontend |
| **golang-migrate** | opcional | Correr migraciones manualmente (ver nota abajo) |

> **Nota sobre golang-migrate:** El proyecto incluye su propio script `migrate.sh` que usa `psql` para aplicar migraciones de forma idempotente. No es estrictamente necesario instalar `golang-migrate` a menos que prefieras usarlo.

Verifica que tienes todo:

```bash
docker --version
docker compose version
go version
node --version
npm --version
```

---

## 2. Opción A — Docker Compose (todo en uno)

Esta es la forma más rápida. Levanta los 5 servicios (PostgreSQL, migraciones, API, relay-worker, pdf-service y frontend) con un solo comando.

### Paso 1: Construir y levantar

```bash
cd ~/Code/invoice-generator
docker compose -f deploy/docker/docker-compose.yml up -d --build
```

Esto hace lo siguiente automáticamente:

1. Inicia **PostgreSQL 16** (puerto `5432`)
2. Ejecuta el contenedor `migrate` que aplica las migraciones SQL
3. Construye y arranca:
   - **backend** (API) → puerto `8080`
   - **relay-worker** (procesa eventos del outbox)
   - **pdf-service** (genera PDFs de facturas)
4. Construye y arranca el **frontend** (nginx sirviendo el build de React) → puerto `5173`

### Paso 2: Verificar

```bash
# Health check del API
curl http://localhost:8080/health
# Debería responder: {"status":"ok"}

# Frontend en el navegador
open http://localhost:5173
```

### Paso 3: Ver logs (si necesitas debuggear)

```bash
# Todos los servicios
docker compose -f deploy/docker/docker-compose.yml logs -f

# Solo un servicio
docker compose -f deploy/docker/docker-compose.yml logs -f backend
docker compose -f deploy/docker/docker-compose.yml logs -f relay-worker
docker compose -f deploy/docker/docker-compose.yml logs -f pdf-service
docker compose -f deploy/docker/docker-compose.yml logs -f frontend
docker compose -f deploy/docker/docker-compose.yml logs -f postgres
```

### Detener todo

```bash
docker compose -f deploy/docker/docker-compose.yml down
```

### Detener y borrar volúmenes (datos de BD)

```bash
docker compose -f deploy/docker/docker-compose.yml down -v
```

---

## 3. Opción B — Ejecución manual (componente por componente)

Útil para desarrollo: corres cada pieza por separado y tienes hot-reload del frontend y el backend.

### Paso 1: Levantar PostgreSQL

```bash
docker run -d --name invoicegen-pg \
  -e POSTGRES_DB=invoicegen \
  -e POSTGRES_USER=invoicegen \
  -e POSTGRES_PASSWORD=invoicegen_dev \
  -p 5432:5432 \
  postgres:16-alpine
```

Espera a que esté listo:

```bash
until docker exec invoicegen-pg pg_isready -U invoicegen; do sleep 1; done
```

### Paso 2: Aplicar migraciones

#### Opción 2a — Con el script incluido (usa `psql` dentro del contenedor)

```bash
# Copiar migraciones al contenedor
docker cp ~/Code/invoice-generator/backend/migrations invoicegen-pg:/migrations

# Ejecutar el script de migración dentro del contenedor
docker exec -e DB_HOST=localhost -e DB_PORT=5432 -e DB_NAME=invoicegen \
  -e DB_USER=invoicegen -e DB_PASSWORD=invoicegen_dev \
  invoicegen-pg /bin/sh -c '
    cd /migrations
    for file in /migrations/*.up.sql; do
      version=$(basename "$file" .up.sql)
      psql -v ON_ERROR_STOP=1 -f "$file" "postgres://invoicegen:invoicegen_dev@localhost:5432/invoicegen?sslmode=disable"
    done
    echo "migrations applied"
  '
```

#### Opción 2b — Con `golang-migrate` (si lo tienes instalado)

```bash
migrate -path ~/Code/invoice-generator/backend/migrations \
  -database "postgres://invoicegen:invoicegen_dev@localhost:5432/invoicegen?sslmode=disable" \
  up
```

#### Opción 2c — Con psql manualmente

```bash
cd ~/Code/invoice-generator/backend/migrations
psql "postgres://invoicegen:invoicegen_dev@localhost:5432/invoicegen?sslmode=disable" \
  -f 001_initial_schema.up.sql \
  -f 002_seed_dev.up.sql
```

### Paso 3: Aplicar seed data (datos de desarrollo)

La migración `002_seed_dev.up.sql` ya inserta:

- **Tenant:** `Acme Corp` (ID: `00000000-0000-0000-0000-000000000001`)
- **Customer:** `Cliente Demo S.A.S` (ID: `00000000-0000-0000-0000-000000000010`)

Si usaste la opción 2a o 2b, esto ya se ejecutó. Si hiciste migraciones a mano, asegúrate de correr también el archivo `002_seed_dev.up.sql`.

### Paso 4: Iniciar el API (terminal 1)

```bash
cd ~/Code/invoice-generator/backend
go run cmd/api/main.go
```

Salida esperada:

```
connected to database
API server listening on :8080
```

El API queda disponible en `http://localhost:8080`.

### Paso 5: Iniciar el relay-worker (terminal 2)

```bash
cd ~/Code/invoice-generator/backend
go run cmd/relay-worker/main.go
```

Salida esperada:

```
relay-worker starting...
relay-worker polling outbox every 2s...
```

Este worker procesa eventos del outbox (patrón Transactional Outbox). Cada 2 segundos busca eventos pendientes y los despacha a los handlers correspondientes.

### Paso 6: Iniciar el pdf-service (terminal 3)

```bash
cd ~/Code/invoice-generator/backend
go run cmd/pdf-service/main.go
```

Salida esperada:

```
pdf-service starting...
pdf-service polling every 3s, storage: /tmp/pdfs
```

Este servicio genera PDFs de facturas. Cada 3 segundos busca eventos `invoice.issued` en el outbox y genera el archivo HTML (placeholder del PDF real).

### Paso 7: Iniciar el frontend (terminal 4)

```bash
cd ~/Code/invoice-generator/frontend
npm install      # solo la primera vez
npm run dev
```

Salida esperada:

```
  VITE v5.x.x  ready in xxx ms

  ➜  Local:   http://localhost:5173/
  ➜  Network: use --host to expose
```

El frontend queda disponible en `http://localhost:5173`.

> **Proxy:** Vite está configurado para hacer proxy de las rutas `/api` hacia `http://localhost:8080`, así que las peticiones del frontend al backend funcionan automáticamente.

---

## 4. Verificación

### Health check

```bash
curl http://localhost:8080/health
# → {"status":"ok"}
```

### Listar facturas (con el seed data)

```bash
curl -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001" \
     -H "Authorization: Bearer dev-token" \
     http://localhost:8080/api/v1/invoices
```

### Abrir el frontend

```
http://localhost:5173
```

Deberías ver la interfaz con navegación a:
- **Invoices** — lista de facturas
- **Customers** — lista de clientes
- **Create Invoice** — formulario para crear facturas

---

## 5. Compilación / Build de producción

### Backend — Compilar binarios

```bash
cd ~/Code/invoice-generator/backend

# Compilar API
go build -o bin/api ./cmd/api

# Compilar relay-worker
go build -o bin/relay-worker ./cmd/relay-worker

# Compilar pdf-service
go build -o bin/pdf-service ./cmd/pdf-service
```

Los binarios quedan en `backend/bin/`. Puedes ejecutarlos directamente:

```bash
./bin/api
./bin/relay-worker
./bin/pdf-service
```

### Frontend — Build de producción

```bash
cd ~/Code/invoice-generator/frontend
npm run build
```

Esto ejecuta `tsc && vite build`, que:
1. Hace type-checking con TypeScript
2. Compila y empaqueta todo en `frontend/dist/`

Para servir el build de producción localmente:

```bash
cd ~/Code/invoice-generator/frontend
npm run preview
# → http://localhost:4173
```

### Build con Docker (imágenes individuales)

```bash
# Imagen del backend
docker build -f deploy/docker/Dockerfile.backend -t invoicegen-backend ~/Code/invoice-generator/backend

# Imagen del frontend
docker build -f deploy/docker/Dockerfile.frontend -t invoicegen-frontend ~/Code/invoice-generator/frontend
```

### Build cross-compilation (binarios para Linux)

```bash
cd ~/Code/invoice-generator/backend

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/api-linux-amd64 ./cmd/api
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/relay-worker-linux-amd64 ./cmd/relay-worker
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/pdf-service-linux-amd64 ./cmd/pdf-service
```

---

## 6. Ejecutar tests

### Todos los tests del backend

```bash
cd ~/Code/invoice-generator/backend
go test ./...
```

### Tests con verbose

```bash
go test -v ./...
```

### Tests con coverage

```bash
go test ./... -cover
```

### Tests por capa

```bash
# Domain layer (entidades, value objects, state machine)
go test ./internal/domain/...

# Application layer (comandos y queries con repos en memoria)
go test ./internal/application/...

# API layer (handlers HTTP con httptest)
go test ./internal/api/...

# Config
go test ./internal/config/...
```

### Tests específicos con verbose

```bash
go test -v ./internal/domain/entity/...
go test -v ./internal/api/handler/...
go test -v ./internal/api/middleware/...
```

---

## 7. Estructura de servicios

| Servicio | Puerto | Entry point | Función |
|----------|--------|-------------|---------|
| **PostgreSQL** | `5432` | Contenedor Docker `postgres:16-alpine` | Base de datos con RLS |
| **API** | `8080` | `backend/cmd/api/main.go` | HTTP API (chi router) |
| **relay-worker** | — | `backend/cmd/relay-worker/main.go` | Procesa eventos del outbox cada 2s |
| **pdf-service** | — | `backend/cmd/pdf-service/main.go` | Genera PDFs cada 3s |
| **frontend** | `5173` | `frontend/src/main.tsx` (Vite) | React + TypeScript SPA |

### Flujo de la app

```
Navegador (:5173)
    │
    ├── Vite proxy ──► /api/* ──► API (:8080)
    │                                  │
    │                                  ├── PostgreSQL (:5432)
    │                                  │      ├── RLS por tenant_id
    │                                  │      ├── Outbox table
    │                                  │      └── Audit entries
    │                                  │
    │                                  ├── relay-worker (polls outbox cada 2s)
    │                                  │      └── dispatcha eventos a handlers
    │                                  │
    │                                  └── pdf-service (polls outbox cada 3s)
    │                                         └── genera HTML → /tmp/pdfs/
    │
    └── Servir React SPA (Vite dev server)
```

---

## 8. Variables de entorno

El backend lee estas variables (con defaults para desarrollo):

| Variable | Default | Descripción |
|----------|---------|-------------|
| `PORT` | `8080` | Puerto del API |
| `DB_HOST` | `localhost` | Host de PostgreSQL |
| `DB_PORT` | `5432` | Puerto de PostgreSQL |
| `DB_NAME` | `invoicegen` | Nombre de la base de datos |
| `DB_USER` | `invoicegen` | Usuario de la BD |
| `DB_PASSWORD` | `invoicegen_dev` | Password de la BD |
| `JWT_SECRET` | `dev-secret-change-in-production` | Secret para JWT (cambiar en prod) |
| `PDF_STORAGE_PATH` | `/tmp/pdfs` | Directorio para guardar PDFs |
| `ENVIRONMENT` | `development` | Entorno (development/production) |

### .env ejemplo para desarrollo local

Si prefieres usar un archivo `.env`, créalo en `backend/`:

```bash
cat > ~/Code/invoice-generator/backend/.env << 'EOF'
PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_NAME=invoicegen
DB_USER=invoicegen
DB_PASSWORD=invoicegen_dev
JWT_SECRET=dev-secret-change-in-production
PDF_STORAGE_PATH=/tmp/pdfs
ENVIRONMENT=development
EOF
```

> **Nota:** Go no carga `.env` automáticamente. Si quieres usarlo, puedes exportar las variables con `set -a; source .env; set +a` antes de correr `go run`.

---

## 9. Solución de problemas

### El puerto 5432 ya está en uso

Hay otro PostgreSQL corriendo. Detenlo o cambia el puerto:

```bash
# Ver qué ocupa el puerto
lsof -i :5432

# O usa un puerto diferente
docker run -d --name invoicegen-pg \
  -e POSTGRES_DB=invoicegen \
  -e POSTGRES_USER=invoicegen \
  -e POSTGRES_PASSWORD=invoicegen_dev \
  -p 5433:5432 \
  postgres:16-alpine

# Y actualiza DB_PORT
DB_PORT=5433 go run cmd/api/main.go
```

### El puerto 8080 ya está en uso

```bash
lsof -i :8080
# O usa otro puerto
PORT=9090 go run cmd/api/main.go
```

### Error "failed to connect to db"

1. Verifica que PostgreSQL esté corriendo: `docker ps`
2. Verifica que las credenciales coincidan
3. Verifica que el puerto sea accesible: `psql postgres://invoicegen:invoicegen_dev@localhost:5432/invoicegen`

### Error en migraciones (psql no encontrado)

El script de migración usa `psql`. Instálalo o ejecuta las migraciones dentro del contenedor:

```bash
docker exec -it invoicegen-pg psql -U invoicegen -d invoicegen -f /migrations/001_initial_schema.up.sql
docker exec -it invoicegen-pg psql -U invoicegen -d invoicegen -f /migrations/002_seed_dev.up.sql
```

### El frontend no puede conectar al backend

1. Verifica que el API esté corriendo en `:8080`
2. Verifica el proxy de Vite en `frontend/vite.config.ts` — debe apuntar a `http://localhost:8080`
3. Revisa la consola del navegador para errores de CORS

### "relation does not exist" en el backend

Las migraciones no se aplicaron. Vuelve al [Paso 2](#paso-2-aplicar-migraciones) de la ejecución manual.

### RLS bloquea las queries

El backend setea `app.tenant_id` en la sesión de PostgreSQL para que RLS funcione. Si haces queries manuales a la BD, necesitas setear la variable:

```sql
SET app.tenant_id = '00000000-0000-0000-0000-000000000001';
SELECT * FROM invoices;
```

### Reiniciar todo desde cero

```bash
# Docker Compose
docker compose -f deploy/docker/docker-compose.yml down -v
docker compose -f deploy/docker/docker-compose.yml up -d --build

# O manual: borrar contenedor de PostgreSQL
docker rm -f invoicegen-pg
# Y volver al Paso 1 de ejecución manual
```

---

## Resumen rápido

**Docker (recomendado):**
```bash
docker compose -f deploy/docker/docker-compose.yml up -d --build
# API: localhost:8080 · Frontend: localhost:5173
```

**Manual (4 terminales):**
```bash
# T1: PostgreSQL
docker run -d --name invoicegen-pg -e POSTGRES_DB=invoicegen -e POSTGRES_USER=invoicegen -e POSTGRES_PASSWORD=invoicegen_dev -p 5432:5432 postgres:16-alpine

# Migraciones (una sola vez)
psql "postgres://invoicegen:invoicegen_dev@localhost:5432/invoicegen?sslmode=disable" \
  -f ~/Code/invoice-generator/backend/migrations/001_initial_schema.up.sql \
  -f ~/Code/invoice-generator/backend/migrations/002_seed_dev.up.sql

# T1: API
cd ~/Code/invoice-generator/backend && go run cmd/api/main.go

# T2: Relay worker
cd ~/Code/invoice-generator/backend && go run cmd/relay-worker/main.go

# T3: PDF service
cd ~/Code/invoice-generator/backend && go run cmd/pdf-service/main.go

# T4: Frontend
cd ~/Code/invoice-generator/frontend && npm install && npm run dev
```