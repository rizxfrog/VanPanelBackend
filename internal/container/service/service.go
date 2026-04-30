package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	containermodel "github.com/GoSimplicity/AI-CloudOps/internal/container/model"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

const composeProjectLabel = "com.docker.compose.project"

type ContainerService interface {
	List(ctx context.Context, query containermodel.ListQuery) (containermodel.ListResult, error)
	Operate(ctx context.Context, id string, operation string) error
	Stats(ctx context.Context, id string) (containermodel.Stats, error)
	Logs(ctx context.Context, id string, opts containermodel.LogOptions) (io.ReadCloser, error)
}

type DockerClient interface {
	ContainerList(ctx context.Context, options dockertypes.ContainerListOptions) ([]dockertypes.Container, error)
	ContainerStart(ctx context.Context, containerID string, options dockertypes.ContainerStartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRestart(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options dockertypes.ContainerRemoveOptions) error
	ContainerStats(ctx context.Context, containerID string, stream bool) (dockertypes.ContainerStats, error)
	ContainerLogs(ctx context.Context, containerID string, options dockertypes.ContainerLogsOptions) (io.ReadCloser, error)
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

func (s *Service) List(ctx context.Context, query containermodel.ListQuery) (containermodel.ListResult, error) {
	cli, err := s.newClient()
	if err != nil {
		return containermodel.ListResult{}, normalizeDockerError(err)
	}
	defer cli.Close()

	summaries, err := cli.ContainerList(ctx, dockertypes.ContainerListOptions{All: true})
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
		err = cli.ContainerStart(ctx, id, dockertypes.ContainerStartOptions{})
	case "stop":
		err = cli.ContainerStop(ctx, id, container.StopOptions{})
	case "restart":
		err = cli.ContainerRestart(ctx, id, container.StopOptions{})
	case "delete":
		err = cli.ContainerRemove(ctx, id, dockertypes.ContainerRemoveOptions{Force: true, RemoveVolumes: false})
	}
	return normalizeDockerError(err)
}

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

	var stats dockertypes.StatsJSON
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

	logs, err := cli.ContainerLogs(ctx, id, dockertypes.ContainerLogsOptions{
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

func mapContainerSummary(item dockertypes.Container) containermodel.Container {
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

func mapStats(stats dockertypes.StatsJSON) containermodel.Stats {
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

func calculateNetwork(networks map[string]dockertypes.NetworkStats) (uint64, uint64) {
	var rx uint64
	var tx uint64
	for _, item := range networks {
		rx += item.RxBytes
		tx += item.TxBytes
	}
	return rx, tx
}

func calculateBlockIO(stats dockertypes.BlkioStats) (uint64, uint64) {
	var read uint64
	var write uint64
	for _, item := range stats.IoServiceBytesRecursive {
		switch strings.ToLower(item.Op) {
		case "read":
			read += item.Value
		case "write":
			write += item.Value
		}
	}
	return read, write
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
	case strings.Contains(msg, "cannot connect") || strings.Contains(msg, "connection refused") || strings.Contains(msg, "no such file"):
		return fmt.Errorf("Docker daemon is unavailable")
	default:
		return err
	}
}
