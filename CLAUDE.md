# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

VanPanelBackend (AI-CloudOps) is the Go backend for a cloud-native intelligent operations platform. It serves as the central API gateway, owns all persistent state, manages Kubernetes clusters, Prometheus monitoring, ITIL ticketing, CMDB service trees, and delegates AI-heavy tasks to a Python service.

- **Go 1.24.6** · Gin · GORM · Wire DI · MySQL or PostgreSQL · Redis
- **Port**: 8889 (configurable via `server.port`)
- **Module path**: `github.com/rizxfrog/VanPanelBackend`

## Commonly Used Commands

| Command | Purpose |
|---------|---------|
| `go run main.go` | Start dev server (port 8889) |
| `make dev-air` | Hot-reload dev server (Air, auto-installs if missing) |
| `make generate` | Regenerate Wire DI code (`go generate ./...`) — **required after adding/modifying provider functions** |
| `make fmt` | `go fmt ./...` |
| `make fmt-imports` | goimports (auto-installs if missing) |
| `make lint` | `go vet ./...` |
| `go test ./...` | Run all tests |
| `go test -run TestFunctionName ./...` | Run single test |
| `go build -o ai-cloudops main.go` | Build binary |
| `make docker-start-env` | Start MySQL + Redis via Docker |
| `make docker-start-server` | Start full-stack via docker-compose |
| `make stop` | Stop all Docker services and remove network |

---

## Architecture

### Startup Sequence (`main.go` → `run()`)

1. `di.InitViper()` — loads `config/*.yaml` + env vars
2. `godotenv.Load()` — loads `.env` file
3. `di.ProvideCmd()` — Wire DI builds the entire `Cmd` struct
4. `di.InitDB()` — connects to DB, runs `AutoMigrate` on 35+ model structs
5. `di.CheckDBHealth(db)` — retries 3× with 10s sleep between pings
6. `cmd.Bootstrap.InitializeK8sClients(ctx)` — loads all registered K8s cluster clients (30s timeout)
7. Mock init if `mock.enabled` and DB available
8. Three background goroutines (all gated on DB availability):
   - **Asynq server** — Redis-backed task consumer (handles `"cron:task"` type)
   - **Asynq scheduler** — `cmd.Scheduler.Run()`
   - **UnifiedCronManager** — `cmd.CronManager.Start(ctx)`
9. HTTP server on configured port
10. Graceful shutdown: CronManager (30s timeout) → AsynqServer → Scheduler → HTTP server

**Degraded mode**: If DB is unavailable, the server still starts but with no K8s clients, no cron, no mock, no Asynq.

### Dependency Injection (Google Wire)

`pkg/di/wire.go` defines the `Cmd` struct and 13 provider sets:

```go
type Cmd struct {
    Server       *gin.Engine
    Bootstrap    startup.ApplicationBootstrap
    CronManager  cron.CronManager
    AsynqServer  *asynq.Server
    AsynqClient  *asynq.Client
    Scheduler    *asynq.Scheduler
    CronHandlers *cronHandler.CronHandlers
}
```

**Provider sets**: HandlerSet (39 handlers), ServiceSet (40 services), DaoSet (29 DAOs), ManagerSet (21 K8s managers), CacheSet (6 Prometheus caches), ClientSet, AsynqSet, CronSet, SSHSet, UtilSet, NotificationSet, JobSet, Injector.

**Adding a new domain**: Declare provider function → add to relevant set in `wire.go` → run `make generate` to regenerate `wire_gen.go`. Do not edit `wire_gen.go` manually.

### Layered Architecture (4-tier per domain)

```
Router --> Middleware --> API Handler --> Service --> Repository --> Model (GORM) --> SQLite/Postgres
```

Each domain follows strict layering:
- `api/` — Gin HTTP handlers (parameter validation via `validator`/`binding`, response via `helper.Success()`)
- `service/` — Business logic (aggregates cache, DAO, third-party; idempotent; uses `context.Context` for timeout)
- `dao/` — GORM database access (named methods, pagination, sorting)
- `utils/` — Domain-specific utilities

Cross-layer calls are prohibited. HTTP entry only in `api`, business in `service`, DB/external in `dao`.

### Domains

| Domain | Path | Purpose |
|--------|------|---------|
| `k8s` | `internal/k8s/` | Multi-cluster K8s management. `api/` (23 handlers), `service/`, `dao/`, `manager/` (22 client-go wrappers), `client/` (kubeconfig loader), `utils/` (YAML diff/apply) |
| `prometheus` | `internal/prometheus/` | Scrape pools/jobs, alert rules, recording rules, on-duty groups, alert events. Config state in `cache/` (in-memory). |
| `workorder` | `internal/workorder/` | ITIL ticketing: templates, instances, approval flows, timelines, comments, notifications |
| `tree` | `internal/tree/` | CMDB service-tree: `TreeNode` hierarchy, local/cloud assets, cloud accounts |
| `system` | `internal/system/` | Users, roles, RBAC (Casbin), audit logs, system config |
| `cron` | `internal/cron/` | Unified cron manager backed by Asynq (Redis queue) |
| `files` | `internal/files/` | File manager (upload/download/browse) |

### Middleware Chain (applied in order)

1. **CORS** — localhost origins; exposes `x-jwt-token`, `x-refresh-token`
2. **JWT CheckLogin** — parses `Authorization` header (or `?token=` for WebSocket paths); validates against `jwt.key1`; checks session via `CheckSession(ctx, uc.Ssid)`
3. **Log** — request path, method, body, status, duration (zap)
4. **Auth (CheckAuth)** — Casbin RBAC: skips `/api/user/login`, `/api/user/logout`, etc.; always passes for `admin` and service accounts; wildcard path matching
5. **AuditLog** — records non-GET requests asynchronously; 1MB body limit; 3s context timeout

Unauthenticated routes in `internal/not_auth/`.

### Response Format

All API handlers return via `helper.Success(ctx)` / `helper.SuccessWithData(ctx, data)` / `helper.ErrorWithDetail(ctx, code, err)`. Unified `{code, message, data}` JSON structure. User-facing errors translated at API layer; developer-facing errors wrapped with `fmt.Errorf`.

### Config System

- **Priority**: Environment variables > config file > defaults
- **Env var mapping**: Dot-separated keys → underscored env vars (e.g., `server.port` → `SERVER_PORT`)
- **Config selection**: `--config` flag overrides; otherwise `ENV` env var selects: `production` → `config.production.yaml`, `test` → `config.test.yaml`, default → `config.development.yaml`
- **Database**: `database.driver` selects `"mysql"` or `"postgres"`; `database.dsn` for connection string
- **See**: `env.example` for all configurable variables

### Shared Packages (`pkg/`)

| Package | Purpose |
|---------|---------|
| `di/` | Wire DI, DB init, middleware setup, config loading |
| `jwt/` | Token issue/verify |
| `sse/` | Server-Sent Events push |
| `ssh/` | SSH client for node terminal |
| `terminal/` | Web-terminal handler |
| `websocket/` | WebSocket hub |
| `stream/` | Streaming response helpers |
| `base/` | Base response helpers, param extraction |
| `retry/` | Retry logic |

### Testing

Tests use standard `testing.T` with `t.Fatal`/`t.Fatalf` (no testify). Subtests via `t.Run()`. Naming: `TestTypeName_MethodOrScenario`. Tests are sparse — mostly in `internal/files/` and `pkg/`. No mocking framework.

### Docker Infrastructure (docker-compose.yaml)

9 services: cloudops-backend (:8889), cloudops-web, cloudops-aiops (:8080), cloudops-mcp, mysql (:3306), postgres (:5432), nginx (:80), prometheus (:9090), redis (:36379). Network: `cloudops_net` (172.30.0.0/16).

### CI/CD (GitLab CI)

4 stages: `build-frontend` (node:18) → `build-backend` (golang:1.24.6) → `build-image` (Kaniko, tags only) → `deploy` (manual, tags only). Go build cache: `~/.cache/go-build`.

---

## Code Conventions

- Strict `api → service → dao` layering; no cross-layer calls
- DI via `pkg/di` + Wire; new services must update injection graph and `generate.go`
- Table names snake_case, struct fields camelCase + `gorm` tags
- Transactions via `gorm.Transaction` in Service; never exposed to API
- Redis keys must have business prefixes; TTL configurable
- Logging: zap with request-id tracing; sensitive data masked
- Errors wrapped with `fmt.Errorf` or `pkg/errors`; user-facing messages translated at API layer
- Comments default to Chinese; keep concise, logic-focused; no AI-generated traces
- New features require tests; `go test ./...` must pass
- Commit convention: `feat` · `fix` · `docs` · `style` · `refactor` · `test` · `ci` (Conventional Commits)
- Concurrency: `x/sync/errgroup` or channels; no bare goroutines
- All external input must be validated (length, format, enum)
- Config via Viper + YAML + env vars; no hardcoded secrets
- Swagger annotations (`@Tags`, `@Summary`, `@Router`, `@Security`) on all API handlers
- `@x-panel-log` annotations for audit trail on mutation endpoints
