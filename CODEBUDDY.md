# CODEBUDDY.md

This file provides guidance to CodeBuddy Code when working with code in this repository.

## Commonly Used Commands

| Command | Purpose |
|---------|---------|
| `go run main.go` | Start dev server (port from `server.port` config, default 8889) |
| `make dev-air` | Hot-reload dev server (Air, auto-installs) |
| `make generate` | Regenerate Wire DI code — **required after adding/editing provider functions** |
| `make fmt` | `go fmt ./...` |
| `make fmt-imports` | goimports (auto-installs if missing) |
| `make lint` | `go vet ./...` |
| `make fmt-check` | Check code formatting (for CI) |
| `go test ./...` | Run all tests |
| `go test -run TestFunctionName ./...` | Run single test |
| `go build -o ai-cloudops main.go` | Build binary |
| `make docker-start-env` | Start MySQL + Redis via Docker |
| `make docker-start-server` | Start full stack via docker-compose |
| `make stop` | Stop all services and remove network |

## Architecture

### Module and Runtime

- **Go 1.25.5** · module path: `github.com/rizxfrog/VanPanelBackend`
- Gin + GORM + Google Wire DI + Asynq (Redis task queue)
- MySQL (runtime state) + Redis (cache/distributed-lock/queue)
- Port: configurable via `server.port` (default 8889 in config, 8889 in .env)

### Startup Sequence (`main.go` → `run()`)

1. `godotenv.Load()` — loads `.env`
2. `di.InitViper()` — loads `config/*.yaml` + env vars
3. `di.ProvideCmd()` — Wire builds the `Cmd` struct (all DI wired)
4. `di.InitDB()` — connects to DB, runs `AutoMigrate` on all models
5. `cmd.Bootstrap.InitializeK8sClients(ctx)` — loads K8s cluster clients (gated on DB health)
6. Mock init if `mock.enabled` and DB is available
7. Three background goroutines (all gated on DB availability): **AsynqServer**, **AsynqScheduler**, **UnifiedCronManager**
8. HTTP server on configured port
9. Graceful shutdown: CronManager (30s) → Asynq → HTTP

**Degraded mode**: Server starts without DB, K8s clients, cron, mock, or Asynq.

### Dependency Injection (Google Wire)

`pkg/di/wire.go` defines 13 provider sets. The `Cmd` struct is the central DI container:

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

**Adding a new domain**: declare provider → add to relevant set in `wire.go` → `make generate`. Never edit `wire_gen.go` manually.

### Layered Architecture (per domain)

```
Router → Middleware → API Handler → Service → Repository (DAO) → Model (GORM)
```

Strict 4-tier per domain: `api/` → `service/` → `dao/` → `utils/`. No cross-layer calls.

### Domains

| Domain | Path | Purpose |
|--------|------|---------|
| `agent` | `internal/agent/` | **AI Agent assistant** — ReAct agent via Eino (openai ChatModel), streaming SSE chat, text-based tool calling (XML parsing), MCP tool manager, risk evaluator, plugin hub, remote MCP configs. Endpoints at `/api/system/agent/*`. Streaming: `textReActLoop.Stream()` uses `chatModel.Stream()` directly for token-by-token deltas; tool calls are parsed from text output via XML patterns. Sync: `react.NewAgent().Generate()` with native function calling. |
| `k8s` | `internal/k8s/` | Multi-cluster K8s management — `api/` handlers, `service/`, `dao/`, `manager/` (22 client-go wrappers for Deployment/Pod/Node/Service/Ingress/ConfigMap/Secret/PV/PVC/RBAC), `client/` (kubeconfig loader), `utils/` (YAML diff/apply) |
| `prometheus` | `internal/prometheus/` | Scrape pools/jobs, alert rules, recording rules, on-duty groups, alert events. In-memory config cache. |
| `workorder` | `internal/workorder/` | ITIL ticketing: templates, instances, approval flows, timelines, comments, notifications |
| `tree` | `internal/tree/` | CMDB service-tree: `TreeNode` hierarchy, local/cloud assets, cloud accounts |
| `system` | `internal/system/` | Users, roles, RBAC (Casbin), audit logs, system config |
| `files` | `internal/files/` | File manager (upload/download/browse) |
| `cron` | `internal/cron/` | Unified cron manager backed by Asynq (Redis queue) |
| `not_auth` | `internal/not_auth/` | Public/unauthenticated endpoints |
| `startup` | `internal/startup/` | K8s client bootstrap on startup |

### Middleware Chain (applied in order)

1. **CORS** — localhost origins; exposes `x-jwt-token`, `x-refresh-token`
2. **JWT CheckLogin** — parses `Authorization` header (or `?token=` for WebSocket), validates against JWT keys, session check
3. **Log** — request path, method, body, status, duration (zap)
4. **Auth (CheckAuth)** — Casbin RBAC; skips login/logout endpoints; admin always passes; wildcard path matching
5. **AuditLog** — async records non-GET; 1MB body limit; 3s context timeout

### Response Format

All handlers return `{code, message, data}` JSON via `helper.Success(ctx)` / `helper.SuccessWithData(ctx, data)` / `helper.ErrorWithDetail(ctx, code, err)`. User-facing errors translated at API layer; developer errors wrapped with `fmt.Errorf`.

### Config System

- **Priority**: env vars > config file > defaults
- **Dot keys → env vars**: `server.port` → `SERVER_PORT`, `agent.llm.base_url` → `AGENT_LLM_BASE_URL`
- **Files**: `config/config.development.yaml`, `config.production.yaml`, `config.test.yaml` — selected by `ENV` env var or `--config` flag
- **Database**: `database.driver` (`"mysql"` or `"postgres"`), `database.dsn` for connection
- **Agent LLM**: `agent.llm.*` section, overridden by `AGENT_LLM_*` env vars — provider, base_url, api_key, model, temperature, max_tokens

### Shared Packages (`pkg/`)

| Package | Purpose |
|---------|---------|
| `di/` | Wire DI, DB init, middleware setup, config loading, Agent/LLM config |
| `base/` | Response helpers (`Success`, `ErrorWithDetail`), param extraction |
| `jwt/` | Token issue/verify |
| `sse/` | Server-Sent Events push |
| `websocket/` | WebSocket hub |
| `stream/` | Streaming response helpers |

## Code Conventions

- Strict `api → service → dao` layering; no cross-layer calls or transaction exposure to API
- Transactions via `gorm.Transaction` in Service layer
- Table names snake_case; struct fields camelCase + `gorm` tags
- Redis keys: business prefix required, TTL configurable
- Logging: zap with request-id; sensitive data masked
- Errors wrapped with `fmt.Errorf`; user-facing messages translated at API layer
- Comments default to Chinese; keep concise, logic-focused
- Config via Viper + YAML + env vars; no hardcoded secrets
- All external input validated (length, format, enum)
- Concurrency: `x/sync/errgroup` or channels; no bare goroutines
- Commit convention: `feat` · `fix` · `docs` · `style` · `refactor` · `test` · `ci` (Conventional Commits)

### Agent Module Streaming Architecture

The agent module has two ReAct implementations for the same model:

- **Sync path** (`Query`): `react.NewAgent(AgentConfig{ToolCallingModel: chatModel, ToolsConfig: {...}}).Generate()` — native function calling via OpenAI tools API
- **Streaming path** (`QueryStream`): `textReActLoop.Stream(chatModel, tools, maxStep, writer, writeSSE)` — direct `chatModel.Stream()` for token-by-token deltas, text-based tool call parsing (XML regex), SSE events: `delta`, `tool_call`, `tool_result`, `done`

Both use the same `createChatModel()` → `einoopenai.ChatModel`. The streaming path injects tool descriptions into the system prompt via `injectToolPrompt()` and parses XML-format tool calls from model text output via `parseTextToolCalls()`. Max 10 tool-calling steps per request.

## 前端代码仓库

前端代码仓库位于: `D:\Repositories\MyRepository\VanPanel\VanPanelWebUI`
