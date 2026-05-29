# VanPanel App Store Design

## Goal

Add a 1Panel-style Docker-based application store to VanPanel. Users can browse a catalog of 100+ pre-defined applications, install them with parameterized configuration, and manage the full lifecycle (start/stop/restart/logs/uninstall). The first version reuses 1Panel's remote app repository for catalog data and deploys applications via Docker Compose on the same host as the VanPanel backend.

## Scope

In scope for v1:

- App catalog: sync from 1Panel remote repository, browse/search/filter, view app details and versions
- Parameterized installation: dynamic forms from app parameter definitions, port mapping, resource limits
- Lifecycle management: start, stop, restart, view logs (SSE), uninstall
- Installation progress: asynchronous via Asynq tasks, progress streaming via SSE

Out of scope for v1:

- App upgrades and version diff
- Backup and restore
- Custom/local app definitions
- Remote Docker host deployment
- Multi-node/cluster deployment
- Docker image management UI
- App favorites and drag-to-sort ordering

## Reference Model

1Panel's app store (located in `agent/app/`) provides the architectural reference:

- Remote catalog sync via `1panel.json.zip` from `https://apps.1panel.store`
- App definitions stored in local SQLite with GORM models (`App`, `AppDetail`, `AppInstall`)
- Parameterized docker-compose templates with `${PANEL_APP_*}` placeholder substitution
- Asynchronous installation via a structured Task system
- Frontend: grid-based catalog browser, drawer-based detail views, dynamic install forms

VanPanel reuses the same catalog format and deployment pattern but adapts the architecture to its own domain layering (api → service → dao), Postgres storage, and Asynq task system.

## Database Models

Five tables added to VanPanel's Postgres via GORM AutoMigrate:

### `appstore_app` — synced app definitions

| Field | Type | Notes |
|---|---|---|
| id | uint (PK) | auto-increment |
| created_at, updated_at, deleted_at | timestamp | GORM standard |
| key | string (unique) | e.g. "mysql", "wordpress" |
| name | string | display name |
| short_desc_zh, short_desc_en | string | bilingual short description |
| icon | string | local file path after download |
| type | string | "database", "website", "tool", "runtime" |
| resource | string | "remote" (always for v1) |
| architectures | string (JSON array) | e.g. ["amd64", "arm64"] |
| memory_required | int | minimum RAM in MB |
| readme_zh, readme_en | text | markdown descriptions |
| website, github, document | string | metadata links |
| status | string | "Normal" or "TakeDown" |
| last_modified | int64 | unix timestamp from remote |

### `appstore_app_detail` — version-specific data

| Field | Type | Notes |
|---|---|---|
| id | uint (PK) | auto-increment |
| created_at, updated_at, deleted_at | timestamp | GORM standard |
| app_id | uint (FK) | → appstore_app.id |
| version | string | e.g. "8.0.32" |
| params | text (JSON) | install form field definitions |
| docker_compose | text | template with `${PANEL_APP_*}` placeholders |
| download_url | string | app package .tar.gz URL |
| status | string | "Normal" or "TakeDown" |

### `appstore_install` — runtime installation instances

| Field | Type | Notes |
|---|---|---|
| id | uint (PK) | auto-increment |
| created_at, updated_at, deleted_at | timestamp | GORM standard |
| name | string | user-defined instance name |
| app_detail_id | uint (FK) | → appstore_app_detail.id |
| version | string | installed version |
| env | text (JSON) | resolved env vars (key-value map) |
| docker_compose | text | rendered compose file |
| status | string | "Installing", "Running", "Stopped", "Error", "Uninstalling" |
| message | string | error message when status=Error |
| http_port | int | mapped HTTP port |
| https_port | int | mapped HTTPS port |
| container_name | string | primary container name |
| install_dir | string | filesystem path to compose project |

Status lifecycle: `Installing` → `Running` or `Error`; `Running` ↔ `Stopped`; any → `Uninstalling` → deleted.

### `appstore_tag` — tag catalog

| Field | Type | Notes |
|---|---|---|
| id | uint (PK) | auto-increment |
| name | string (unique) | tag name |

### `appstore_app_tag` — many-to-many relation

| Field | Type | Notes |
|---|---|---|
| app_id | uint (FK) | → appstore_app.id |
| tag_id | uint (FK) | → appstore_tag.id |

## Domain Structure

New domain at `internal/appstore/`, following existing layering:

```text
internal/appstore/
├── api/
│   ├── handler.go          # Route registration
│   ├── app.go               # App catalog handlers
│   └── install.go           # Install lifecycle handlers
├── service/
│   ├── service.go           # Service interfaces + constructors
│   ├── app.go               # Catalog logic (search, sync, detail)
│   └── install.go           # Install logic (create, operate, logs)
├── dao/
│   ├── dao.go               # DAO interfaces + constructors
│   ├── app.go               # App CRUD + queries
│   ├── app_detail.go        # AppDetail CRUD
│   ├── install.go           # Install CRUD
│   └── tag.go               # Tag queries
└── utils/
    ├── compose.go           # Compose template rendering, env injection
    └── sync.go              # Remote repo download, JSON parse, diff
```

GORM models go in `internal/model/` (existing convention):

```text
internal/model/
├── appstore_app.go
├── appstore_app_detail.go
├── appstore_install.go
├── appstore_tag.go
└── appstore_app_tag.go
```

## API Routes

Route group `/api/appstore/`:

### Catalog

| Method | Path | Description |
|---|---|---|
| GET | `/api/appstore/apps` | Paginated app list (query: name, type, tag, page, pageSize) |
| GET | `/api/appstore/apps/:key` | App detail with version list |
| GET | `/api/appstore/apps/:key/:version` | Version detail with install params and compose template |
| GET | `/api/appstore/tags` | All tags |
| POST | `/api/appstore/sync` | Trigger remote catalog sync |
| GET | `/api/appstore/sync/status` | Current sync status |

### Installs

| Method | Path | Description |
|---|---|---|
| GET | `/api/appstore/installs` | Paginated install list (query: status, name, page, pageSize) |
| GET | `/api/appstore/installs/:id` | Install detail with status |
| POST | `/api/appstore/installs` | Create install (body: app_detail_id, name, params, cpu, memory, ports) |
| POST | `/api/appstore/installs/:id/start` | Start installed app |
| POST | `/api/appstore/installs/:id/stop` | Stop installed app |
| POST | `/api/appstore/installs/:id/restart` | Restart installed app |
| DELETE | `/api/appstore/installs/:id` | Uninstall app |
| GET | `/api/appstore/installs/:id/logs` | Container logs (SSE streaming when follow=true) |
| GET | `/api/appstore/installs/:id/progress` | Installation progress (SSE) |

## Deployment Flow

### Catalog Sync (`POST /api/appstore/sync`)

```
1. GET https://apps.1panel.store/stable/1panel.json.version.txt
2. Compare with stored AppStoreLastModified config value
3. If newer: GET https://apps.1panel.store/stable/1panel.json.zip
4. Parse JSON, iterate apps:
   - New app → INSERT into appstore_app + appstore_app_detail
   - Changed app → UPDATE existing records
   - Removed app → mark appstore_app.status = "TakeDown"
5. Download icons concurrently (4 workers) → save to AppResourceDir/icons/
6. Update AppStoreLastModified config
7. Return sync summary: {added, updated, removed}
```

### Installation (`POST /api/appstore/installs`)

```
1. Validate request: app_detail_id exists, name not duplicate, ports free
2. Load AppDetail from DB, get docker_compose template + params definition
3. INSERT into appstore_install with status = "Installing"
4. Enqueue Asynq task (type: "appstore:install") with install_id
5. Return install_id to client

Asynq Worker (async):
  1. Create install directory: {AppInstallDir}/{appKey}/{installName}/
  2. Download and extract app package if download_url present
  3. Render docker-compose.yml:
     - Replace ${PANEL_APP_PORT_HTTP}, ${PANEL_DB_ROOT_PASSWORD}, etc.
     - Set CPU/memory limits from user params
     - Set restart policy
  4. Write .env file with resolved env vars
  5. Write rendered compose file
  6. Run docker compose -f <path> up -d
  7. On success: UPDATE status = "Running", set container_name
  8. On failure: UPDATE status = "Error", set message
```

### Lifecycle Operations

All ops use `docker compose` CLI via shell execution:

- **Start**: `docker compose -f <path> start` → status "Running"
- **Stop**: `docker compose -f <path> stop` → status "Stopped"
- **Restart**: `docker compose -f <path> restart` → status "Running"
- **Logs**: `docker compose -f <path> logs --tail=N -f` streamed via SSE
- **Uninstall**: `docker compose -f <path> down --remove-orphans` → delete install dir → DELETE install record

### Progress Streaming (SSE)

During async installation, the worker writes progress events to Redis (key: `appstore:install:{id}:progress`). The SSE endpoint reads from this key and pushes events to the client. Events:

- `{"step": "download", "message": "Downloading app package..."}`
- `{"step": "render", "message": "Generating compose file..."}`
- `{"step": "deploy", "message": "Pulling image mysql:8.0.32..."}`
- `{"step": "complete", "message": "Installation complete"}`

## Frontend Structure

New files in `VanPanelWebUI/apps/web-antd/src/`:

```text
src/
├── api/core/appstore/
│   └── index.ts                    # Typed API client (apps, installs, sync)
├── router/routes/modules/
│   └── appstore.ts                 # Route definitions
└── views/appstore/
    ├── index.vue                   # Parent: tabs (All / Installed / Settings)
    ├── apps/
    │   └── index.vue               # App grid with search, tag filter, pagination
    ├── detail/
    │   ├── index.vue               # App detail drawer (README, versions, install button)
    │   └── install.vue             # Install form drawer (dynamic params, advanced options)
    └── installed/
        ├── index.vue               # Installed app cards with status indicators
        └── manage.vue              # Management panel (ops buttons, logs drawer)
```

### Routes

- `/appstore` — main app store page (tabs)
- `/appstore/apps` — catalog grid
- `/appstore/installed` — installed apps list

### Key UI Patterns

- **App cards**: icon, name, short description, type badge, tags, "Install" button or "Installed" indicator
- **Tag filter**: horizontal chip list above the grid, multi-select
- **Install form**: dynamically generated from app params definition. Field types: text, password, number, select. Advanced section: CPU limit, memory limit, port mapping, restart policy.
- **Installed cards**: icon, name, version, status dot (green=running, red=stopped, yellow=installing, gray=error), port info, action buttons (start/stop/restart/logs/uninstall)
- **Logs drawer**: terminal-style scrollable output with auto-follow toggle

## Wire DI Registration

Add to `pkg/di/wire.go`:

```go
// HandlerSet — add:
appstoreApi.NewAppStoreHandler

// ServiceSet — add:
appstoreService.NewAppStoreService

// DaoSet — add:
appstoreDao.NewAppStoreDAO
```

Then run `make generate` to regenerate `wire_gen.go`.

Add to `pkg/di/gorm.go` InitTables():

```go
&model.AppStoreApp{},
&model.AppStoreAppDetail{},
&model.AppStoreInstall{},
&model.AppStoreTag{},
&model.AppStoreAppTag{},
```

Add to `pkg/di/web.go` InitGinServer():

```go
appstoreHdl.RegisterRouters(server)
```

## Config

New config entries (in `config/*.yaml` or env vars):

| Key | Default | Description |
|---|---|---|
| `appstore.repo_url` | `https://apps.1panel.store` | Remote app repository base URL |
| `appstore.repo_mode` | `stable` | Repository mode (stable/dev) |
| `appstore.install_dir` | `/opt/appstore` | Base directory for installed app files |
| `appstore.resource_dir` | `/opt/appstore/resource` | Base directory for app resource files (icons, packages) |
| `appstore.last_modified` | `0` | Last sync timestamp (managed by system) |

## Filesystem Layout

```text
{AppInstallDir}/
└── {appKey}/
    └── {installName}/
        ├── docker-compose.yml     # Rendered compose file
        ├── .env                   # Environment variables
        └── [app files]            # Extracted from app package

{AppResourceDir}/
├── icons/
│   └── {appKey}.png               # Downloaded app icons
└── apps/
    └── {appKey}/
        └── {version}/
            └── [extracted package files]
```

## Error Handling

- Docker daemon unavailable: return 503 with "Docker service unavailable"
- Port conflict: return 409 with "Port {port} is already in use by {app}"
- Duplicate install name: return 409 with "Install name already exists"
- Sync failure: return 500 with "Failed to sync app catalog: {reason}"
- Compose template parse error: return 500 with "Invalid compose template"
- Compose up failure: install status → "Error", message from stderr
- Asynq not available: return 503 with "Task queue unavailable"

## Testing Strategy

- `internal/appstore/service/` — unit tests with mock Docker client and mock DAO
- `internal/appstore/utils/` — unit tests for compose template rendering, env injection, JSON parsing
- `internal/appstore/api/` — integration tests with httptest + gin test mode
- `internal/appstore/dao/` — integration tests with test Postgres instance
- Frontend: Vitest unit tests for API client functions and component rendering

## Dependencies to Add

```text
# go.mod additions:
github.com/compose-spec/compose-go/v2  # Compose file parsing/rendering
```

Already present: `github.com/docker/docker`, `github.com/gin-gonic/gin`, `gorm.io/gorm`, `gorm.io/driver/postgres`, `github.com/hibiken/asynq`
