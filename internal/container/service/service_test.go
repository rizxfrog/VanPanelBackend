package service

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	containermodel "github.com/rizxfrog/VanPanelBackend/internal/container/model"
)

func TestMapContainerSummary(t *testing.T) {
	summary := dockertypes.Container{
		ID:      "1234567890abcdef",
		Names:   []string{"/web"},
		Image:   "nginx:latest",
		ImageID: "sha256:abcdef",
		State:   "running",
		Status:  "Up 2 minutes",
		Created: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC).Unix(),
		Ports:   []dockertypes.Port{{IP: "0.0.0.0", PrivatePort: 80, PublicPort: 8080, Type: "tcp"}},
		NetworkSettings: &dockertypes.SummaryNetworkSettings{
			Networks: map[string]*network.EndpointSettings{
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
	stats := dockertypes.StatsJSON{
		Stats: dockertypes.Stats{
			CPUStats: dockertypes.CPUStats{
				CPUUsage:    dockertypes.CPUUsage{TotalUsage: 300},
				SystemUsage: 1000,
				OnlineCPUs:  2,
			},
			PreCPUStats: dockertypes.CPUStats{
				CPUUsage:    dockertypes.CPUUsage{TotalUsage: 100},
				SystemUsage: 500,
			},
			MemoryStats: dockertypes.MemoryStats{
				Usage: 1000,
				Limit: 2000,
				Stats: map[string]uint64{"cache": 200},
			},
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

func TestServiceListFiltersAndPaginates(t *testing.T) {
	svc := &Service{newClient: func() (DockerClient, error) {
		return &fakeDockerClient{
			containers: []dockertypes.Container{
				{ID: "1", Names: []string{"/web"}, Image: "nginx:latest", State: "running"},
				{ID: "2", Names: []string{"/db"}, Image: "mysql:8", State: "exited"},
			},
		}, nil
	}}

	got, err := svc.List(context.Background(), containermodel.ListQuery{Name: "nginx", State: "running", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if got.Total != 1 || len(got.Items) != 1 || got.Items[0].Name != "web" {
		t.Fatalf("unexpected list result: %#v", got)
	}
}

func TestServiceOperateCallsDocker(t *testing.T) {
	fake := &fakeDockerClient{}
	svc := &Service{newClient: func() (DockerClient, error) { return fake, nil }}

	if err := svc.Operate(context.Background(), "abc", "restart"); err != nil {
		t.Fatalf("Operate returned error: %v", err)
	}
	if fake.operation != "restart:abc" {
		t.Fatalf("unexpected operation: %s", fake.operation)
	}
}

type fakeDockerClient struct {
	containers []dockertypes.Container
	operation  string
}

func (f *fakeDockerClient) ContainerList(context.Context, dockertypes.ContainerListOptions) ([]dockertypes.Container, error) {
	return f.containers, nil
}

func (f *fakeDockerClient) ContainerStart(_ context.Context, id string, _ dockertypes.ContainerStartOptions) error {
	f.operation = "start:" + id
	return nil
}

func (f *fakeDockerClient) ContainerStop(_ context.Context, id string, _ container.StopOptions) error {
	f.operation = "stop:" + id
	return nil
}

func (f *fakeDockerClient) ContainerRestart(_ context.Context, id string, _ container.StopOptions) error {
	f.operation = "restart:" + id
	return nil
}

func (f *fakeDockerClient) ContainerRemove(_ context.Context, id string, _ dockertypes.ContainerRemoveOptions) error {
	f.operation = "delete:" + id
	return nil
}

func (f *fakeDockerClient) ContainerStats(context.Context, string, bool) (dockertypes.ContainerStats, error) {
	return dockertypes.ContainerStats{}, nil
}

func (f *fakeDockerClient) ContainerLogs(context.Context, string, dockertypes.ContainerLogsOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("ok")), nil
}

func (f *fakeDockerClient) Close() error {
	return nil
}
