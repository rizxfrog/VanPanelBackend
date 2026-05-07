# VanPanel System Container Manager Design

## Goal

Add a 1Panel-style container management page to VanPanel under the existing system module. The first version provides a minimal usable Docker container management loop: list containers, start, stop, restart, delete, view logs, and view resource usage.

The feature must use VanPanel-owned backend APIs. The WebUI must not call Docker directly.

## Scope

In scope:

- Add a system container manager route at `/system/containers`.
- Add a backend container module that talks to Docker Engine through the Docker Go SDK.
- List local Docker containers visible to the VanPanelBackend process.
- Start, stop, restart, and delete containers.
- Read container logs.
- Read per-container resource usage.
- Return clear errors when Docker is unavailable or permission is denied.

Out of scope for the first version:

- Creating containers.
- Editing container settings.
- Image management.
- Network management.
- Volume management.
- Compose project management.
- Compose templates.
- Docker daemon settings.
- Container terminal or exec sessions.
- Remote Docker hosts.

## Reference Model

1Panel's container management is built as a WebUI over backend services that wrap Docker Engine operations. Its backend service uses Docker client calls for container list, lifecycle operations, logs, and stats, then exposes those operations through HTTP APIs.

VanPanel should follow the same high-level model but avoid copying 1Panel's broader coupling to app store installs, Compose projects, website bindings, and task orchestration. The first version keeps the module focused on raw Docker containers.

## Frontend Routing And Structure

The WebUI will expose the container manager as a child of the existing system route:

```text
/system/containers
```

Frontend files:

```text
apps/web-antd/src/views/system/ContainerManager.vue
apps/web-antd/src/views/system/container-manager.css
apps/web-antd/src/api/core/system/container.ts
```

The route will be registered next to the existing system pages:

```text
/system/files
/system/terminal
/system/containers
```

The page will show:

- Container table with name, short ID, image, state, status/runtime, created time, ports, and IP addresses.
- Row actions for start, stop, restart, delete, logs, and stats.
- Search and state filter.
- Logs drawer or modal.
- Stats drawer or compact panel.

## Backend Architecture

Add a new backend module:

```text
internal/container/model
internal/container/service
internal/container/api
pkg/di/container.go
```

Responsibilities:

- `model`: request and response types for containers, operations, logs, and stats.
- `service`: Docker client construction, container list mapping, lifecycle operations, logs, stats, and error normalization.
- `api`: HTTP routes and request validation.
- `pkg/di`: dependency wiring for the container handler.

The service should create Docker clients with API version negotiation:

```text
client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
```

This allows the deployment environment to use standard Docker environment variables while defaulting to the local Docker daemon.

## Backend API

The first version exposes:

```text
GET    /api/system/containers
POST   /api/system/containers/:id/start
POST   /api/system/containers/:id/stop
POST   /api/system/containers/:id/restart
DELETE /api/system/containers/:id
GET    /api/system/containers/:id/stats
GET    /api/system/containers/:id/logs
```

List query parameters:

```text
name     optional string filter against container name or image
state    optional state filter; all, created, running, paused, restarting, removing, exited, dead
page     optional page number
pageSize optional page size
```

Logs query parameters:

```text
tail       optional line count, default 200
since      optional Docker duration or timestamp string
timestamps optional boolean
follow     optional boolean
```

For the first implementation, logs may be returned as plain text when `follow=false`. If `follow=true`, the endpoint should use Server-Sent Events so the browser can stream logs without using the terminal WebSocket protocol.

## Container Data Model

Container list items should include:

```text
id
shortId
name
image
imageId
state
status
createdAt
ports
ips
labels
isCompose
```

Stats response should include:

```text
cpuPercent
memoryUsage
memoryLimit
memoryPercent
networkRx
networkTx
blockRead
blockWrite
readAt
```

The frontend should format bytes, percentages, and timestamps. The backend should return raw numeric values where practical.

## Lifecycle Operations

Lifecycle operations map directly to Docker Engine calls:

```text
start   -> ContainerStart
stop    -> ContainerStop
restart -> ContainerRestart
delete  -> ContainerRemove
```

Delete behavior:

- Delete should require the container ID or name from the route.
- The first version may use force delete to match typical panel behavior, but the API should make that explicit in service code.
- Removing anonymous volumes is allowed in the first version only if documented in the implementation. Named volume management remains out of scope.

## Logs

Logs should use Docker Engine logs instead of shelling out to `docker logs`.

Default behavior:

- Return the last 200 lines.
- Include stdout and stderr.
- Do not include timestamps unless requested.
- Do not persist logs in VanPanel.

Streaming behavior:

- `follow=true` returns SSE events.
- Client disconnect must cancel the Docker logs stream.
- Errors should be emitted as SSE error events or normal API errors before streaming starts.

## Resource Usage

Stats should use Docker Engine stats with `stream=false`.

CPU calculation must follow Docker's standard delta formula using `CPUStats` and `PreCPUStats`.

Memory usage should subtract cache when Docker provides cache data, while still returning the raw limit.

Network and block IO should be summed across interfaces and devices.

## Security Model

Container management is privileged. The API must require the same authentication middleware as other system APIs.

Security rules:

- The frontend cannot pass arbitrary Docker host socket paths.
- The backend uses its own Docker client configuration.
- Error responses must not include secrets from environment variables.
- Delete operations must be explicit and logged through normal backend logging when available.
- The first version does not expose container exec or shell access.

## Error Handling

Expected errors:

- Docker daemon is not running.
- Docker socket permission denied.
- Docker client cannot negotiate API version.
- Container not found.
- Operation invalid for the current container state.
- Logs unavailable for the selected logging driver.

The backend should normalize common Docker connection and permission errors into concise user-facing messages, while preserving detailed logs server-side.

## Testing

Backend tests:

- Container list mapping from Docker summaries.
- Name and state filtering.
- Stats calculation helpers.
- Operation validation.
- Error normalization.

Frontend tests:

- API response normalization.
- Stats and byte formatting helpers.
- Table filtering state.

Manual verification:

- Docker unavailable: page shows a clear Docker unavailable message.
- Docker available: `/system/containers` lists containers.
- Start a stopped container.
- Stop a running container.
- Restart a running container.
- View logs for a container.
- View stats for a running container.
- Delete a disposable test container.

