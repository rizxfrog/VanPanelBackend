package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	containermodel "github.com/GoSimplicity/AI-CloudOps/internal/container/model"
	dockertypes "github.com/docker/docker/api/types"
)

const composeProjectLabel = "com.docker.compose.project"

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
