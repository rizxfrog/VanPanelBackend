# AGENTS.md

## What This Repo Is

Go backend (VanPanelBackend) for a cloud-native ops platform. API gateway on port 8889, manages K8s clusters, Prometheus monitoring, RBAC, ticketing, CMDB service trees, and an AI agent subsystem. Frontend lives in a separate repo.

- **Go 1.24.6** · Gin · GORM · Wire DI · MySQL or PostgreSQL · Redis
- See `CLAUDE.md` for full architecture, startup sequence, middleware chain, and domain details.

## Commands That Matter

```bash
make generate          # Wire DI — REQUIRED after adding/modifying provider functions in pkg/di/wire.go
make fmt               # go fmt ./...
make fmt-imports       # goimports (auto-installs if missing)
make lint              # go vet ./...
go test ./...          # All tests
go test -run TestName ./path  # Single test
make dev-air           # Hot-reload dev server (Air, auto-installs if missing)
go run main.go         # Start dev server on :8889
```

**Order after touching Wire**: `make generate` → `make fmt` → `make lint` → `go test ./...`

## Repository Structure

```
internal/
├── agent/          # AI agent subsystem (18 sub-packages: mcp, risk, guard, pipeline, skill, tool, hub, memory, audit, search, insight, nudge, etc.)
├── constants/      # Shared constants
├── files/          # File manager (upload/download/browse)
├── gateway/        # WebSocket/JSON-RPC gateway server (separate from Gin HTTP router)
├── middleware/     # Gin middleware (CORS, JWT, auth, audit log)
├── model/          # GORM models (system_user, system_role, agent, file_share, etc.)
├── not_auth/       # Unauthenticated API routes
├── startup/        # Bootstrap/init logic
├── system/         # Users, roles, RBAC (Casbin), audit logs, system config
└── terminal/       # Web terminal (PTY + WebSocket)
pkg/
├── di/             # Wire DI definitions (wire.go), DB init, middleware setup
├── base/           # Response helpers, param extraction
├── jwt/            # Token issue/verify
├── sse/            # Server-Sent Events
├── ssh/            # SSH client
├── websocket/      # WebSocket hub
└── ...
config/             # YAML config files (config.development.yaml, config.production.yaml, etc.)
deploy/             # Docker, K8s, Nginx, MySQL init, Prometheus configs
```

## Key Gotchas

- **Wire DI is strict**: New providers must be added to the correct set in `pkg/di/wire.go` (HandlerSet, ServiceSet, DaoSet, AgentSet, etc.), then `make generate` to regenerate `wire_gen.go`. Never edit `wire_gen.go` manually.
- **4-tier layering**: `api/` → `service/` → `dao/` → `model/`. No cross-layer calls.
- **Config priority**: env vars > `config/*.yaml` > defaults. Env vars map dots to underscores: `server.port` → `SERVER_PORT`.
- **DB driver**: `database.driver` selects `"mysql"` or `"postgres"`. Check `config/config.development.yaml` before assuming which one.
- **MCP server mode**: Activated via `mcp.serve: true` in config or `--mcp-serve`. Runs as separate stdio/TCP process on port 8890.
- **Degraded mode**: If DB is unavailable, server starts without K8s clients, cron, mock, or Asynq.
- **Agent module is large**: `internal/agent/` has 18 sub-packages (mcp, risk, guard, pipeline, skill, tool, hub, memory, audit, search, insight, nudge, etc.). Read the specific sub-package before modifying.
- **Gateway**: `internal/gateway/` is a separate JSON-RPC/WebSocket server, not the Gin HTTP router.
- **Tests**: Use standard `testing.T` (no testify). Tests in `internal/agent/`, `internal/files/`, `internal/terminal/`, `internal/system/`, `pkg/`. Naming: `TestTypeName_MethodOrScenario`.
- **Makefile fmt-imports** has a hardcoded `github.com/yourusername` — this is a bug in the Makefile, not something to replicate.
- **Go version discrepancy**: `go.mod` says 1.24.6, CI uses 1.24.6, Dockerfile uses 1.26.1. Match the version to the context you're working in.

## Conventions

- Comments in Chinese, concise, logic-focused
- Errors: `fmt.Errorf` or `pkg/errors`; user-facing messages translated at API layer
- Redis keys must have business prefixes
- Concurrency via `x/sync/errgroup` or channels; no bare goroutines
- Swagger annotations on all API handlers (`@Tags`, `@Summary`, `@Router`, `@Security`)
- `@x-panel-log` annotations for audit trail on mutation endpoints
- Commit convention: `feat` · `fix` · `docs` · `style` · `refactor` · `test` · `ci`

## CI

**GitHub Actions** (`.github/workflows/go.yml`): `test-and-build` → `auto-release` (auto-tags on main push, creates GitHub releases).

**GitLab CI** (`.gitlab-ci.yml`): 4 stages: `build-frontend` (node:18) → `build-backend` (golang:1.24.6) → `build-image` (Kaniko, tags only) → `deploy` (manual, tags only).
