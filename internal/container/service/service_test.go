package service

import (
	"testing"
	"time"

	containermodel "github.com/GoSimplicity/AI-CloudOps/internal/container/model"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
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
				CPUUsage:   dockertypes.CPUUsage{TotalUsage: 300},
				SystemUsage: 1000,
				OnlineCPUs:  2,
			},
			PreCPUStats: dockertypes.CPUStats{
				CPUUsage:   dockertypes.CPUUsage{TotalUsage: 100},
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
