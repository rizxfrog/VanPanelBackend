# Container Manager Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the A-scope system container manager: list Docker containers, start/stop/restart/delete containers, view logs, and view stats under `/system/containers`.

**Architecture:** The backend owns Docker access through a small `internal/container` module and exposes authenticated `/api/system/containers` APIs. The frontend adds a system page that consumes only VanPanel APIs and does not talk to Docker directly. Tests focus on pure mapping, formatting, route/API behavior, and handler error paths so they can run without a Docker daemon.

**Tech Stack:** Go, Gin, Docker Go SDK, Wire, Vue 3, Ant Design Vue, Vben request client, Vitest.

---

## File Structure

Backend files:

- Create: `internal/container/model/types.go` for API DTOs and Docker-independent models.
- Create: `internal/container/service/service.go` for Docker client abstraction, list/log/stats/operation orchestration, and error normalization.
- Create: `internal/container/service/service_test.go` for mapping, filtering, stats, and error tests.
- Create: `internal/container/api/handler.go` for HTTP routes.
- Create: `internal/container/api/handler_test.go` for route and API response tests.
- Create: `pkg/di/container.go` for Docker container service provider helpers if needed.
- Modify: `pkg/di/wire.go` to register the container handler and service.
- Modify: `pkg/di/wire_gen.go` manually or regenerate with Wire so runtime injection includes the container handler.
- Modify: `pkg/di/web.go` to call `containerHdl.RegisterRouters(server)`.
- Modify: `go.mod` and `go.sum` to include Docker SDK dependencies.

Frontend files:

- Create: `apps/web-antd/src/api/core/system/container.ts` for container API calls and response normalization.
- Create: `apps/web-antd/src/api/core/system/container.test.ts` for response/formatter tests.
- Create: `apps/web-antd/src/views/system/container-manager-utils.ts` for byte, percent, state, and log helpers.
- Create: `apps/web-antd/src/views/system/container-manager-utils.test.ts` for helper tests.
- Create: `apps/web-antd/src/views/system/ContainerManager.vue` for the table, actions, logs drawer, and stats drawer.
- Create: `apps/web-antd/src/views/system/container-manager.css` for page styling.
- Modify: `apps/web-antd/src/router/routes/modules/files.ts` to add `/system/containers`.

---

### Task 1: Backend Models And Pure Mapping Helpers

**Files:**
- Create: `internal/container/model/types.go`
- Create: `internal/container/service/service.go`
- Test: `internal/container/service/service_test.go`

- [ ] **Step 1: Write failing tests for filtering, port/IP mapping, and stats helpers**

Add tests in `internal/container/service/service_test.go`:

```go
package service

import (
	"testing"
	"time"

	containermodel "github.com/GoSimplicity/AI-CloudOps/internal/container/model"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

func TestMapContainerSummary(t *testing.T) {
	summary := container.Summary{
		ID:      "1234567890abcdef",
		Names:   []string{"/web"},
		Image:   "nginx:latest",
		ImageID: "sha256:abcdef",
		State:   "running",
		Status:  "Up 2 minutes",
		Created: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC).Unix(),
		Ports: []dockertypes.Port{{IP: "0.0.0.0", PrivatePort: 80, PublicPort: 8080, Type: "tcp"}},
		NetworkSettings: &dockertypes.SummaryNetworkSettings{
			Networks: map[string]*dockertypes.EndpointSettings{
				"bridge": {IPAddress: "172.17.0.2"},
			},
		},
		Labels: map[string]string{"com.docker.compose.project": "demo"},
	}

	got := mapContainerSummary(summary)
	if got.ID != "1234567890abcdef" || got.ShortID != "1234567890ab" {
		t.Fatalf("unexpected ids: %#v", got)
	}
	if got.Name != "web" || got.Image != "nginx:latest" || !got.IsCompose {
		t.Fatalf("unexpected mapped container: %#v", got)
	}
	if len(got.Ports) != 1 || got.Ports[0] != "0.0.0.0:8080->80/tcp" {
		t.Fatalf("unexpected ports: %#v", got.Ports)
	}
	if len(got.IPs) != 1 || got.IPs[0] != "172.17.0.2" {
		t.Fatalf("unexpected ips: %#v", got.IPs)
	}
}

func TestFilterContainers(t *testing.T) {
	items := []containermodel.Container{
		{Name: "web", Image: "nginx:latest", State: "running"},
		{Name: "db", Image: "mysql:8", State: "exited"},
	}
	filtered := filterContainers(items, containermodel.ListQuery{Name: "nginx", State: "running"})
	if len(filtered) != 1 || filtered[0].Name != "web" {
		t.Fatalf("unexpected filtered containers: %#v", filtered)
	}
}

func TestCalculateStats(t *testing.T) {
	stats := container.StatsResponse{
		CPUStats: container.CPUStats{
			CPUUsage:    container.CPUUsage{TotalUsage: 300},
			SystemUsage: 1000,
			OnlineCPUs:  2,
		},
		PreCPUStats: container.CPUStats{
			CPUUsage:    container.CPUUsage{TotalUsage: 100},
			SystemUsage: 500,
		},
		MemoryStats: container.MemoryStats{
			Usage: 1000,
			Limit: 2000,
			Stats: map[string]uint64{"cache": 200},
		},
	}
	got := mapStats(stats)
	if got.CPUPercent != 80 {
		t.Fatalf("unexpected cpu percent: %v", got.CPUPercent)
	}
	if got.MemoryUsage != 800 || got.MemoryLimit != 2000 || got.MemoryPercent != 40 {
		t.Fatalf("unexpected memory stats: %#v", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```powershell
go test ./internal/container/service -run "TestMapContainerSummary|TestFilterContainers|TestCalculateStats" -v
```

Expected: FAIL because the package and functions do not exist.

- [ ] **Step 3: Create model types and pure helpers**

Create `internal/container/model/types.go`:

```go
package model

import "time"

type ListQuery struct {
	Name     string
	State    string
	Page     int
	PageSize int
}

type Container struct {
	ID        string            `json:"id"`
	ShortID   string            `json:"shortId"`
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	ImageID   string            `json:"imageId"`
	State     string            `json:"state"`
	Status    string            `json:"status"`
	CreatedAt time.Time         `json:"createdAt"`
	Ports     []string          `json:"ports"`
	IPs       []string          `json:"ips"`
	Labels    map[string]string `json:"labels"`
	IsCompose bool              `json:"isCompose"`
}

type ListResult struct {
	Items []Container `json:"items"`
	Total int         `json:"total"`
}

type LogOptions struct {
	Tail       string
	Since      string
	Timestamps bool
	Follow     bool
}

type Stats struct {
	CPUPercent   float64   `json:"cpuPercent"`
	MemoryUsage  uint64    `json:"memoryUsage"`
	MemoryLimit  uint64    `json:"memoryLimit"`
	MemoryPercent float64  `json:"memoryPercent"`
	NetworkRx    uint64    `json:"networkRx"`
	NetworkTx    uint64    `json:"networkTx"`
	BlockRead    uint64    `json:"blockRead"`
	BlockWrite   uint64    `json:"blockWrite"`
	ReadAt       time.Time `json:"readAt"`
}
```

Add helpers to `internal/container/service/service.go`:

```go
package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	containermodel "github.com/GoSimplicity/AI-CloudOps/internal/container/model"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

const composeProjectLabel = "com.docker.compose.project"

func mapContainerSummary(item container.Summary) containermodel.Container {
	name := item.ID
	if len(item.Names) > 0 {
		name = strings.TrimPrefix(item.Names[0], "/")
	}
	shortID := item.ID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	return containermodel.Container{
		ID:        item.ID,
		ShortID:   shortID,
		Name:      name,
		Image:     item.Image,
		ImageID:   strings.TrimPrefix(item.ImageID, "sha256:"),
		State:     item.State,
		Status:    item.Status,
		CreatedAt: time.Unix(item.Created, 0),
		Ports:     mapPorts(item.Ports),
		IPs:       mapIPs(item.NetworkSettings),
		Labels:    item.Labels,
		IsCompose: item.Labels[composeProjectLabel] != "",
	}
}

func mapPorts(ports []dockertypes.Port) []string {
	result := make([]string, 0, len(ports))
	for _, port := range ports {
		if port.PublicPort > 0 {
			result = append(result, fmt.Sprintf("%s:%d->%d/%s", port.IP, port.PublicPort, port.PrivatePort, port.Type))
			continue
		}
		result = append(result, fmt.Sprintf("%d/%s", port.PrivatePort, port.Type))
	}
	sort.Strings(result)
	return result
}

func mapIPs(settings *dockertypes.SummaryNetworkSettings) []string {
	if settings == nil {
		return nil
	}
	ips := make([]string, 0, len(settings.Networks))
	for _, network := range settings.Networks {
		if network != nil && network.IPAddress != "" {
			ips = append(ips, network.IPAddress)
		}
	}
	sort.Strings(ips)
	return ips
}

func filterContainers(items []containermodel.Container, query containermodel.ListQuery) []containermodel.Container {
	state := query.State
	if state == "" {
		state = "all"
	}
	name := strings.ToLower(strings.TrimSpace(query.Name))
	filtered := make([]containermodel.Container, 0, len(items))
	for _, item := range items {
		if state != "all" && item.State != state {
			continue
		}
		if name != "" && !strings.Contains(strings.ToLower(item.Name), name) && !strings.Contains(strings.ToLower(item.Image), name) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func mapStats(stats container.StatsResponse) containermodel.Stats {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	onlineCPUs := float64(stats.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	var cpuPercent float64
	if systemDelta > 0 && cpuDelta > 0 && onlineCPUs > 0 {
		cpuPercent = (cpuDelta / systemDelta) * onlineCPUs * 100
	}
	memoryUsage := stats.MemoryStats.Usage
	if cache, ok := stats.MemoryStats.Stats["cache"]; ok && memoryUsage >= cache {
		memoryUsage -= cache
	}
	var memoryPercent float64
	if stats.MemoryStats.Limit > 0 {
		memoryPercent = float64(memoryUsage) / float64(stats.MemoryStats.Limit) * 100
	}
	networkRx, networkTx := calculateNetwork(stats.Networks)
	blockRead, blockWrite := calculateBlockIO(stats.BlkioStats)
	return containermodel.Stats{
		CPUPercent:    cpuPercent,
		MemoryUsage:   memoryUsage,
		MemoryLimit:   stats.MemoryStats.Limit,
		MemoryPercent: memoryPercent,
		NetworkRx:     networkRx,
		NetworkTx:     networkTx,
		BlockRead:     blockRead,
		BlockWrite:    blockWrite,
		ReadAt:        stats.Read,
	}
}
```

- [ ] **Step 4: Run tests and fix compile issues**

Run:

```powershell
go test ./internal/container/service -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/container/model/types.go internal/container/service/service.go internal/container/service/service_test.go go.mod go.sum
git commit -m "feat(container): add models and mapping helpers"
```

---

### Task 2: Backend Docker Service

**Files:**
- Modify: `internal/container/service/service.go`
- Test: `internal/container/service/service_test.go`

- [ ] **Step 1: Add operation validation and service interface tests**

Append tests:

```go
func TestValidateOperation(t *testing.T) {
	for _, op := range []string{"start", "stop", "restart", "delete"} {
		if err := validateOperation(op); err != nil {
			t.Fatalf("expected %s to be valid: %v", op, err)
		}
	}
	if err := validateOperation("exec"); err == nil {
		t.Fatal("expected invalid operation error")
	}
}

func TestNormalizeDockerError(t *testing.T) {
	got := normalizeDockerError(fmt.Errorf("permission denied while trying to connect to the Docker daemon socket"))
	if !strings.Contains(got.Error(), "permission") {
		t.Fatalf("unexpected normalized error: %v", got)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```powershell
go test ./internal/container/service -run "TestValidateOperation|TestNormalizeDockerError" -v
```

Expected: FAIL because functions are missing.

- [ ] **Step 3: Implement service interface and Docker methods**

Extend `service.go` with:

```go
type ContainerService interface {
	List(ctx context.Context, query containermodel.ListQuery) (containermodel.ListResult, error)
	Operate(ctx context.Context, id string, operation string) error
	Stats(ctx context.Context, id string) (containermodel.Stats, error)
	Logs(ctx context.Context, id string, opts containermodel.LogOptions) (io.ReadCloser, error)
}

type DockerClient interface {
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRestart(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerStats(ctx context.Context, containerID string, stream bool) (container.StatsResponseReader, error)
	ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	Close() error
}

type Service struct {
	newClient func() (DockerClient, error)
}

func NewContainerService() ContainerService {
	return &Service{newClient: newDockerClient}
}

func newDockerClient() (DockerClient, error) {
	return client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
}
```

Add methods:

```go
func (s *Service) List(ctx context.Context, query containermodel.ListQuery) (containermodel.ListResult, error) {
	cli, err := s.newClient()
	if err != nil {
		return containermodel.ListResult{}, normalizeDockerError(err)
	}
	defer cli.Close()
	summaries, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return containermodel.ListResult{}, normalizeDockerError(err)
	}
	items := make([]containermodel.Container, 0, len(summaries))
	for _, item := range summaries {
		items = append(items, mapContainerSummary(item))
	}
	items = filterContainers(items, query)
	total := len(items)
	if query.Page > 0 && query.PageSize > 0 {
		start := (query.Page - 1) * query.PageSize
		if start >= len(items) {
			items = []containermodel.Container{}
		} else {
			end := start + query.PageSize
			if end > len(items) {
				end = len(items)
			}
			items = items[start:end]
		}
	}
	return containermodel.ListResult{Items: items, Total: total}, nil
}

func (s *Service) Operate(ctx context.Context, id string, operation string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("container id is required")
	}
	if err := validateOperation(operation); err != nil {
		return err
	}
	cli, err := s.newClient()
	if err != nil {
		return normalizeDockerError(err)
	}
	defer cli.Close()
	switch operation {
	case "start":
		err = cli.ContainerStart(ctx, id, container.StartOptions{})
	case "stop":
		err = cli.ContainerStop(ctx, id, container.StopOptions{})
	case "restart":
		err = cli.ContainerRestart(ctx, id, container.StopOptions{})
	case "delete":
		err = cli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true, RemoveVolumes: false})
	}
	return normalizeDockerError(err)
}
```

Add `Stats`, `Logs`, `validateOperation`, and `normalizeDockerError`:

```go
func (s *Service) Stats(ctx context.Context, id string) (containermodel.Stats, error) {
	if strings.TrimSpace(id) == "" {
		return containermodel.Stats{}, fmt.Errorf("container id is required")
	}
	cli, err := s.newClient()
	if err != nil {
		return containermodel.Stats{}, normalizeDockerError(err)
	}
	defer cli.Close()
	reader, err := cli.ContainerStats(ctx, id, false)
	if err != nil {
		return containermodel.Stats{}, normalizeDockerError(err)
	}
	defer reader.Body.Close()
	var stats container.StatsResponse
	if err := json.NewDecoder(reader.Body).Decode(&stats); err != nil {
		return containermodel.Stats{}, err
	}
	return mapStats(stats), nil
}

func (s *Service) Logs(ctx context.Context, id string, opts containermodel.LogOptions) (io.ReadCloser, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("container id is required")
	}
	if opts.Tail == "" {
		opts.Tail = "200"
	}
	cli, err := s.newClient()
	if err != nil {
		return nil, normalizeDockerError(err)
	}
	logs, err := cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     opts.Follow,
		Tail:       opts.Tail,
		Since:      opts.Since,
		Timestamps: opts.Timestamps,
	})
	if err != nil {
		_ = cli.Close()
		return nil, normalizeDockerError(err)
	}
	return &dockerReadCloser{ReadCloser: logs, closeClient: cli.Close}, nil
}

type dockerReadCloser struct {
	io.ReadCloser
	closeClient func() error
}

func (r *dockerReadCloser) Close() error {
	logErr := r.ReadCloser.Close()
	clientErr := r.closeClient()
	if logErr != nil {
		return logErr
	}
	return clientErr
}

func validateOperation(operation string) error {
	switch operation {
	case "start", "stop", "restart", "delete":
		return nil
	default:
		return fmt.Errorf("unsupported container operation: %s", operation)
	}
}

func normalizeDockerError(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "permission denied"):
		return fmt.Errorf("Docker permission denied; check Docker socket permissions")
	case strings.Contains(msg, "cannot connect") || strings.Contains(msg, "connection refused"):
		return fmt.Errorf("Docker daemon is unavailable")
	default:
		return err
	}
}
```

- [ ] **Step 4: Run service tests**

Run:

```powershell
go test ./internal/container/service -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add internal/container/model/types.go internal/container/service/service.go internal/container/service/service_test.go go.mod go.sum
git commit -m "feat(container): add docker service"
```

---

### Task 3: Backend HTTP API And DI

**Files:**
- Create: `internal/container/api/handler.go`
- Create: `internal/container/api/handler_test.go`
- Modify: `pkg/di/wire.go`
- Modify: `pkg/di/wire_gen.go`
- Modify: `pkg/di/web.go`

- [ ] **Step 1: Write route and error response tests**

Create `internal/container/api/handler_test.go`:

```go
package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	containermodel "github.com/GoSimplicity/AI-CloudOps/internal/container/model"
	"github.com/gin-gonic/gin"
)

type fakeContainerService struct{}

func (fakeContainerService) List(context.Context, containermodel.ListQuery) (containermodel.ListResult, error) {
	return containermodel.ListResult{Items: []containermodel.Container{{ID: "abc", Name: "web"}}, Total: 1}, nil
}
func (fakeContainerService) Operate(context.Context, string, string) error { return nil }
func (fakeContainerService) Stats(context.Context, string) (containermodel.Stats, error) {
	return containermodel.Stats{}, nil
}
func (fakeContainerService) Logs(context.Context, string, containermodel.LogOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("ok\n")), nil
}

func TestRegisterRouters(t *testing.T) {
	router := gin.New()
	NewContainerHandler(fakeContainerService{}).RegisterRouters(router)
	routes := map[string]bool{}
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, expected := range []string{
		"GET /api/system/containers",
		"POST /api/system/containers/:id/start",
		"POST /api/system/containers/:id/stop",
		"POST /api/system/containers/:id/restart",
		"DELETE /api/system/containers/:id",
		"GET /api/system/containers/:id/stats",
		"GET /api/system/containers/:id/logs",
	} {
		if !routes[expected] {
			t.Fatalf("missing route %s", expected)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```powershell
go test ./internal/container/api -v
```

Expected: FAIL because handler does not exist.

- [ ] **Step 3: Implement handler**

Create `internal/container/api/handler.go`:

```go
package api

import (
	"io"
	"net/http"
	"strconv"

	containermodel "github.com/GoSimplicity/AI-CloudOps/internal/container/model"
	containerservice "github.com/GoSimplicity/AI-CloudOps/internal/container/service"
	"github.com/GoSimplicity/AI-CloudOps/pkg/base"
	"github.com/gin-gonic/gin"
)

type ContainerHandler struct {
	service containerservice.ContainerService
}

func NewContainerHandler(service containerservice.ContainerService) *ContainerHandler {
	return &ContainerHandler{service: service}
}

func (h *ContainerHandler) RegisterRouters(server *gin.Engine) {
	group := server.Group("/api/system/containers")
	group.GET("", h.List)
	group.POST("/:id/start", h.Start)
	group.POST("/:id/stop", h.Stop)
	group.POST("/:id/restart", h.Restart)
	group.DELETE("/:id", h.Delete)
	group.GET("/:id/stats", h.Stats)
	group.GET("/:id/logs", h.Logs)
}

func (h *ContainerHandler) List(ctx *gin.Context) {
	result, err := h.service.List(ctx, containermodel.ListQuery{
		Name:     ctx.Query("name"),
		State:    ctx.DefaultQuery("state", "all"),
		Page:     parseInt(ctx.Query("page")),
		PageSize: parseInt(ctx.Query("pageSize")),
	})
	if err != nil {
		base.ErrorWithMessage(ctx, err.Error())
		return
	}
	base.SuccessWithData(ctx, result)
}
```

Implement `Start`, `Stop`, `Restart`, `Delete`, `Stats`, `Logs`, and `parseInt`. `Logs` should stream SSE when `follow=true`; otherwise read the log stream and return `text/plain`.

- [ ] **Step 4: Wire the handler**

Modify `pkg/di/wire.go` imports and provider sets:

```go
containerApi "github.com/GoSimplicity/AI-CloudOps/internal/container/api"
containerService "github.com/GoSimplicity/AI-CloudOps/internal/container/service"
```

Add:

```go
containerApi.NewContainerHandler
containerService.NewContainerService
```

Modify `pkg/di/web.go` to accept `containerHdl *containerApi.ContainerHandler` and call:

```go
containerHdl.RegisterRouters(server)
```

Regenerate Wire if available:

```powershell
go run -mod=mod github.com/google/wire/cmd/wire ./pkg/di
```

If Wire generation fails because the repo uses manual generated files, update `pkg/di/wire_gen.go` consistently with `wire.go` and `web.go`.

- [ ] **Step 5: Run backend tests**

Run:

```powershell
go test ./internal/container/... ./pkg/di
```

Expected: PASS.

- [ ] **Step 6: Commit**

```powershell
git add internal/container pkg/di go.mod go.sum
git commit -m "feat(container): expose system container api"
```

---

### Task 4: Frontend API And Utility Tests

**Files:**
- Create: `apps/web-antd/src/api/core/system/container.ts`
- Create: `apps/web-antd/src/api/core/system/container.test.ts`
- Create: `apps/web-antd/src/views/system/container-manager-utils.ts`
- Create: `apps/web-antd/src/views/system/container-manager-utils.test.ts`

- [ ] **Step 1: Write frontend utility tests**

Create `apps/web-antd/src/views/system/container-manager-utils.test.ts`:

```ts
import { describe, expect, it } from 'vitest';

import { formatBytes, formatPercent, stateTone } from './container-manager-utils';

describe('container-manager-utils', () => {
  it('formats bytes', () => {
    expect(formatBytes(1024)).toBe('1.0 KB');
    expect(formatBytes(1024 * 1024)).toBe('1.0 MB');
  });

  it('formats percent', () => {
    expect(formatPercent(12.345)).toBe('12.35%');
  });

  it('maps state tone', () => {
    expect(stateTone('running')).toBe('success');
    expect(stateTone('exited')).toBe('default');
  });
});
```

Create `apps/web-antd/src/api/core/system/container.test.ts`:

```ts
import { describe, expect, it } from 'vitest';

import { normalizeContainerResponse } from './container';

describe('container api', () => {
  it('unwraps standard response envelopes', () => {
    expect(normalizeContainerResponse({ code: 0, data: [{ name: 'web' }] })).toEqual([{ name: 'web' }]);
  });

  it('throws backend messages from failed envelopes', () => {
    expect(() => normalizeContainerResponse({ code: 500, data: null, message: 'Docker unavailable' })).toThrow('Docker unavailable');
  });
});
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```powershell
pnpm exec vitest run apps/web-antd/src/api/core/system/container.test.ts apps/web-antd/src/views/system/container-manager-utils.test.ts
```

Expected: FAIL because files do not exist.

- [ ] **Step 3: Implement API and utilities**

Create `container-manager-utils.ts`:

```ts
export function formatBytes(value?: number) {
  const bytes = Number(value || 0);
  if (bytes < 1024) return `${bytes} B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let current = bytes / 1024;
  let unit = 0;
  while (current >= 1024 && unit < units.length - 1) {
    current /= 1024;
    unit += 1;
  }
  return `${current.toFixed(1)} ${units[unit]}`;
}

export function formatPercent(value?: number) {
  return `${Number(value || 0).toFixed(2)}%`;
}

export function stateTone(state: string) {
  if (state === 'running') return 'success';
  if (state === 'exited' || state === 'created') return 'default';
  if (state === 'paused') return 'warning';
  return 'error';
}
```

Create `container.ts` with `ContainerItem`, `ContainerStats`, `getContainers`, `startContainer`, `stopContainer`, `restartContainer`, `deleteContainer`, `getContainerStats`, and `getContainerLogs`. Follow `terminal.ts` for `authHeaders` and envelope normalization.

- [ ] **Step 4: Run frontend tests**

Run:

```powershell
pnpm exec vitest run apps/web-antd/src/api/core/system/container.test.ts apps/web-antd/src/views/system/container-manager-utils.test.ts
```

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add apps/web-antd/src/api/core/system/container.ts apps/web-antd/src/api/core/system/container.test.ts apps/web-antd/src/views/system/container-manager-utils.ts apps/web-antd/src/views/system/container-manager-utils.test.ts
git commit -m "feat(container): add frontend api helpers"
```

---

### Task 5: Frontend Container Manager Page And Route

**Files:**
- Create: `apps/web-antd/src/views/system/ContainerManager.vue`
- Create: `apps/web-antd/src/views/system/container-manager.css`
- Modify: `apps/web-antd/src/router/routes/modules/files.ts`

- [ ] **Step 1: Add route**

Modify `files.ts` under the `System` children:

```ts
{
  name: 'SystemContainers',
  path: '/system/containers',
  component: () => import('#/views/system/ContainerManager.vue'),
  meta: {
    icon: 'lucide:container',
    title: '容器管理',
  },
},
```

- [ ] **Step 2: Implement page**

Create `ContainerManager.vue` using Ant Design Vue components already available in the project:

```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { message, Modal } from 'ant-design-vue';

import {
  deleteContainer,
  getContainerLogs,
  getContainers,
  getContainerStats,
  restartContainer,
  startContainer,
  stopContainer,
  type ContainerItem,
  type ContainerStats,
} from '#/api/core/system/container';

import { formatBytes, formatPercent, stateTone } from './container-manager-utils';
import './container-manager.css';

const loading = ref(false);
const containers = ref<ContainerItem[]>([]);
const name = ref('');
const state = ref('all');
const logsOpen = ref(false);
const logsLoading = ref(false);
const logs = ref('');
const statsOpen = ref(false);
const stats = ref<ContainerStats | null>(null);

async function loadContainers() {
  loading.value = true;
  try {
    const result = await getContainers({ name: name.value, state: state.value });
    containers.value = result.items || [];
  } catch (error: any) {
    message.error(error?.message || '加载容器失败');
  } finally {
    loading.value = false;
  }
}

async function runOperation(container: ContainerItem, action: 'start' | 'stop' | 'restart') {
  try {
    if (action === 'start') await startContainer(container.id);
    if (action === 'stop') await stopContainer(container.id);
    if (action === 'restart') await restartContainer(container.id);
    message.success('操作已提交');
    await loadContainers();
  } catch (error: any) {
    message.error(error?.message || '操作失败');
  }
}
</script>
```

Use this template structure:

```vue
<template>
  <div class="container-manager">
    <a-card :bordered="false">
      <div class="container-manager__toolbar">
        <a-input v-model:value="name" allow-clear placeholder="搜索容器名称或镜像" @press-enter="loadContainers" />
        <a-select v-model:value="state" class="container-manager__state" @change="loadContainers">
          <a-select-option value="all">全部状态</a-select-option>
          <a-select-option value="running">运行中</a-select-option>
          <a-select-option value="exited">已停止</a-select-option>
          <a-select-option value="paused">已暂停</a-select-option>
          <a-select-option value="created">已创建</a-select-option>
        </a-select>
        <a-button type="primary" :loading="loading" @click="loadContainers">刷新</a-button>
      </div>
      <a-table :data-source="containers" :loading="loading" row-key="id" size="middle">
        <a-table-column title="名称" data-index="name" />
        <a-table-column title="镜像" data-index="image" />
        <a-table-column title="状态">
          <template #default="{ record }">
            <a-tag :color="stateTone(record.state)">{{ record.state }}</a-tag>
            <span class="container-manager__status">{{ record.status }}</span>
          </template>
        </a-table-column>
        <a-table-column title="端口">
          <template #default="{ record }">{{ record.ports?.join(', ') || '-' }}</template>
        </a-table-column>
        <a-table-column title="IP">
          <template #default="{ record }">{{ record.ips?.join(', ') || '-' }}</template>
        </a-table-column>
        <a-table-column title="操作" fixed="right">
          <template #default="{ record }">
            <a-space>
              <a-button size="small" @click="runOperation(record, 'start')">启动</a-button>
              <a-button size="small" @click="runOperation(record, 'stop')">停止</a-button>
              <a-button size="small" @click="runOperation(record, 'restart')">重启</a-button>
              <a-button size="small" @click="openLogs(record)">日志</a-button>
              <a-button size="small" @click="openStats(record)">资源</a-button>
              <a-button danger size="small" @click="confirmDelete(record)">删除</a-button>
            </a-space>
          </template>
        </a-table-column>
      </a-table>
    </a-card>
    <a-drawer v-model:open="logsOpen" title="容器日志" width="720">
      <a-spin :spinning="logsLoading">
        <pre class="container-manager__logs">{{ logs }}</pre>
      </a-spin>
    </a-drawer>
    <a-drawer v-model:open="statsOpen" title="资源占用" width="420">
      <a-descriptions v-if="stats" :column="1" bordered size="small">
        <a-descriptions-item label="CPU">{{ formatPercent(stats.cpuPercent) }}</a-descriptions-item>
        <a-descriptions-item label="内存">{{ formatBytes(stats.memoryUsage) }} / {{ formatBytes(stats.memoryLimit) }}</a-descriptions-item>
        <a-descriptions-item label="内存占比">{{ formatPercent(stats.memoryPercent) }}</a-descriptions-item>
        <a-descriptions-item label="网络接收">{{ formatBytes(stats.networkRx) }}</a-descriptions-item>
        <a-descriptions-item label="网络发送">{{ formatBytes(stats.networkTx) }}</a-descriptions-item>
        <a-descriptions-item label="磁盘读取">{{ formatBytes(stats.blockRead) }}</a-descriptions-item>
        <a-descriptions-item label="磁盘写入">{{ formatBytes(stats.blockWrite) }}</a-descriptions-item>
      </a-descriptions>
    </a-drawer>
  </div>
</template>
```

- [ ] **Step 3: Typecheck frontend**

Run:

```powershell
pnpm --filter @vben/web-antd typecheck
```

Expected: PASS.

- [ ] **Step 4: Commit**

```powershell
git add apps/web-antd/src/views/system/ContainerManager.vue apps/web-antd/src/views/system/container-manager.css apps/web-antd/src/router/routes/modules/files.ts
git commit -m "feat(container): add system container page"
```

---

### Task 6: Full Verification And Integration Notes

**Files:**
- Modify only if verification exposes a bug.

- [ ] **Step 1: Run backend focused tests**

Run in backend worktree:

```powershell
go test ./internal/container/... ./pkg/di
```

Expected: PASS.

- [ ] **Step 2: Run frontend focused tests**

Run in frontend worktree:

```powershell
pnpm exec vitest run apps/web-antd/src/api/core/system/container.test.ts apps/web-antd/src/views/system/container-manager-utils.test.ts
```

Expected: PASS.

- [ ] **Step 3: Run frontend typecheck**

Run:

```powershell
pnpm --filter @vben/web-antd typecheck
```

Expected: PASS.

- [ ] **Step 4: Run backend build or broad tests if practical**

Run:

```powershell
go test ./...
```

Expected: PASS. If unrelated pre-existing tests fail, capture the failing package and error, then run `go test ./internal/container/... ./pkg/di` again as the minimum required verification.

- [ ] **Step 5: Manual verification checklist**

Use a machine with Docker available:

```powershell
docker ps -a
```

Then verify:

- `/system/containers` loads.
- Docker unavailable shows a clear error if Docker daemon is stopped.
- Running containers appear in the table.
- A stopped disposable container can be started.
- A running disposable container can be stopped.
- A disposable container can be restarted.
- Logs modal shows recent logs.
- Stats modal shows CPU, memory, network, and IO.
- Delete works only after confirmation.

- [ ] **Step 6: Commit fixes if needed**

If verification required fixes:

```powershell
git add <changed-files>
git commit -m "fix(container): address verification issues"
```

If no fixes were needed, do not create an empty commit.
