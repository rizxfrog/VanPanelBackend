/*
 * MIT License
 *
 * Copyright (c) 2024 Bamboo
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 *
 */

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rizxfrog/VanPanelBackend/internal/model"
	"github.com/rizxfrog/VanPanelBackend/internal/system/utils"
	"go.uber.org/zap"
)

type SystemService interface {
	GetCurrentSystemInfo(ctx context.Context) (*model.System, error)
	GetSystemMetrics(ctx context.Context) (*model.System, error)
	RefreshSystemInfo(ctx context.Context) (*model.System, error)
}

type systemService struct {
	l     *zap.Logger
	redis redis.Cmdable
}

func NewSystemService(l *zap.Logger, redis redis.Cmdable) SystemService {
	return &systemService{
		l:     l,
		redis: redis,
	}
}

// GetCurrentSystemInfo 获取当前系统信息
func (s *systemService) GetCurrentSystemInfo(ctx context.Context) (*model.System, error) {
	const cacheKey = "system:info"
	const cacheExpiry = 5 * time.Minute

	// 首先尝试从Redis获取
	cached, err := s.redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var systemInfo model.System
		if err := json.Unmarshal([]byte(cached), &systemInfo); err == nil {
			// 检查缓存是否过期（额外检查，防止Redis过期时间不准确）
			if time.Now().Unix()-systemInfo.LastUpdateTime < 300 {
				return &systemInfo, nil
			}
		}
	}

	// 如果缓存中没有数据或者数据过期，则采集新数据
	s.l.Info("系统信息过期或不存在，开始采集新数据")
	systemInfo, err := s.collectSystemInfo(ctx)
	if err != nil {
		s.l.Error("采集系统信息失败", zap.Error(err))
		return nil, fmt.Errorf("采集系统信息失败: %v", err)
	}

	// 保存到Redis
	if data, err := json.Marshal(systemInfo); err == nil {
		if err := s.redis.Set(ctx, cacheKey, data, cacheExpiry).Err(); err != nil {
			s.l.Error("保存系统信息到Redis失败", zap.Error(err))
		}
	}

	return systemInfo, nil
}

// GetSystemMetrics 获取系统性能指标
func (s *systemService) GetSystemMetrics(ctx context.Context) (*model.System, error) {
	// 实时采集系统指标
	systemInfo, err := s.collectSystemInfo(ctx)
	if err != nil {
		s.l.Error("采集系统指标失败", zap.Error(err))
		return nil, fmt.Errorf("采集系统指标失败: %v", err)
	}

	return systemInfo, nil
}

// RefreshSystemInfo 刷新系统信息
func (s *systemService) RefreshSystemInfo(ctx context.Context) (*model.System, error) {
	const cacheKey = "system:info"
	const cacheExpiry = 5 * time.Minute

	// 强制采集新的系统信息
	systemInfo, err := s.collectSystemInfo(ctx)
	if err != nil {
		s.l.Error("刷新系统信息失败", zap.Error(err))
		return nil, fmt.Errorf("刷新系统信息失败: %v", err)
	}

	// 保存到Redis
	if data, err := json.Marshal(systemInfo); err == nil {
		if err := s.redis.Set(ctx, cacheKey, data, cacheExpiry).Err(); err != nil {
			s.l.Error("保存系统信息到Redis失败", zap.Error(err))
		}
	}

	s.l.Info("系统信息刷新完成")
	return systemInfo, nil
}

// collectSystemInfo 采集系统信息
func (s *systemService) collectSystemInfo(ctx context.Context) (*model.System, error) {
	systemInfo := &model.System{
		LastUpdateTime: time.Now().Unix(),
	}

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// 获取主机名
	hostname, err := os.Hostname()
	if err != nil {
		s.l.Warn("获取主机名失败", zap.Error(err))
		hostname = "unknown"
	}
	systemInfo.Hostname = hostname

	// 获取操作系统信息
	systemInfo.OS = runtime.GOOS
	systemInfo.Arch = runtime.GOARCH
	systemInfo.CPUCores = runtime.NumCPU()

	// 根据操作系统类型采集详细信息
	switch runtime.GOOS {
	case "linux", "darwin":
		if err := s.collectUnixSystemInfo(ctx, systemInfo); err != nil {
			return nil, err
		}
	case "windows":
		if err := s.collectWindowsSystemInfo(ctx, systemInfo); err != nil {
			return nil, err
		}
	default:
		s.l.Warn("不支持的操作系统", zap.String("os", runtime.GOOS))
	}

	return systemInfo, nil
}

// collectUnixSystemInfo 采集Unix系统信息
func (s *systemService) collectUnixSystemInfo(ctx context.Context, systemInfo *model.System) error {
	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// CPU使用率
	if cpuUsage, err := s.getCPUUsage(ctx); err == nil {
		systemInfo.CPUUsage = cpuUsage
	}

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// CPU型号
	if cpuModel, err := s.getCPUModel(ctx); err == nil {
		systemInfo.CPUModel = cpuModel
	}

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 内存信息
	if memInfo, err := s.getMemoryInfo(ctx); err == nil {
		systemInfo.MemoryTotal = memInfo["total"]
		systemInfo.MemoryUsed = memInfo["used"]
		systemInfo.MemoryUsage = utils.GetMemoryUsagePercentage(systemInfo)
	}

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 磁盘信息
	if diskInfo, err := s.getDiskInfo(ctx); err == nil {
		systemInfo.DiskTotal = diskInfo["total"]
		systemInfo.DiskUsed = diskInfo["used"]
		systemInfo.DiskUsage = utils.GetDiskUsagePercentage(systemInfo)
	}

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 系统负载
	if loadAvg, err := s.getLoadAverage(ctx); err == nil {
		systemInfo.LoadAvg1 = loadAvg[0]
		systemInfo.LoadAvg5 = loadAvg[1]
		systemInfo.LoadAvg15 = loadAvg[2]
	}

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 运行时间
	if uptime, err := s.getUptime(ctx); err == nil {
		systemInfo.Uptime = uptime
	}

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 进程数
	if processCount, err := s.getProcessCount(ctx); err == nil {
		systemInfo.ProcessCount = processCount
	}

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 获取操作系统版本
	if osVersion, err := s.getOSVersion(ctx); err == nil {
		systemInfo.OSVersion = osVersion
	}

	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// 获取网络流量信息
	if networkInfo, err := s.getNetworkTraffic(ctx); err == nil {
		systemInfo.NetworkIn = networkInfo["in"]
		systemInfo.NetworkOut = networkInfo["out"]
	}

	return nil
}

// collectWindowsSystemInfo 采集Windows系统信息
func (s *systemService) collectWindowsSystemInfo(ctx context.Context, systemInfo *model.System) error {
	// 检查上下文是否已取消
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// CPU使用率
	if cpuUsage, err := s.getCPUUsage(ctx); err == nil {
		systemInfo.CPUUsage = cpuUsage
	} else {
		s.l.Warn("获取Windows CPU使用率失败", zap.Error(err))
	}

	// CPU型号
	if cpuModel, err := s.getCPUModel(ctx); err == nil {
		systemInfo.CPUModel = cpuModel
	} else {
		s.l.Warn("获取Windows CPU型号失败", zap.Error(err))
	}

	// 内存信息
	if memInfo, err := s.getMemoryInfo(ctx); err == nil {
		systemInfo.MemoryTotal = memInfo["total"]
		systemInfo.MemoryUsed = memInfo["used"]
		systemInfo.MemoryUsage = utils.GetMemoryUsagePercentage(systemInfo)
	} else {
		s.l.Warn("获取Windows内存信息失败", zap.Error(err))
	}

	// 磁盘信息
	if diskInfo, err := s.getDiskInfo(ctx); err == nil {
		systemInfo.DiskTotal = diskInfo["total"]
		systemInfo.DiskUsed = diskInfo["used"]
		systemInfo.DiskUsage = utils.GetDiskUsagePercentage(systemInfo)
	} else {
		s.l.Warn("获取Windows磁盘信息失败", zap.Error(err))
	}

	// 负载（Windows下使用CPU使用率近似）
	if loadAvg, err := s.getLoadAverage(ctx); err == nil {
		systemInfo.LoadAvg1 = loadAvg[0]
		systemInfo.LoadAvg5 = loadAvg[1]
		systemInfo.LoadAvg15 = loadAvg[2]
	}

	// 运行时间
	if uptime, err := s.getUptime(ctx); err == nil {
		systemInfo.Uptime = uptime
	}

	// 进程数
	if processCount, err := s.getProcessCount(ctx); err == nil {
		systemInfo.ProcessCount = processCount
	} else {
		s.l.Warn("获取Windows进程数失败", zap.Error(err))
	}

	// 操作系统版本
	if osVersion, err := s.getOSVersion(ctx); err == nil {
		systemInfo.OSVersion = osVersion
	}

	// 网络流量
	if networkInfo, err := s.getNetworkTraffic(ctx); err == nil {
		systemInfo.NetworkIn = networkInfo["in"]
		systemInfo.NetworkOut = networkInfo["out"]
	}

	return nil
}

func runPowerShellCommand(ctx context.Context, script string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", script)
	if output, err := cmd.Output(); err == nil {
		return output, nil
	}

	cmd = exec.CommandContext(ctx, "pwsh", "-NoProfile", "-Command", script)
	return cmd.Output()
}

func parseFirstTextLine(output []byte) (string, bool) {
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "CLIXML") || strings.HasPrefix(line, "<Objs") {
			continue
		}
		line = strings.Trim(line, "\"'")
		if line != "" {
			return line, true
		}
	}
	return "", false
}

func parseWindowsMemoryInfoFromJSON(jsonText string) (map[string]uint64, error) {
	type memoryInfo struct {
		FreePhysicalMemory     uint64 `json:"FreePhysicalMemory"`
		TotalVisibleMemorySize uint64 `json:"TotalVisibleMemorySize"`
	}

	var item memoryInfo
	if err := json.Unmarshal([]byte(strings.TrimSpace(jsonText)), &item); err != nil {
		return nil, err
	}
	if item.TotalVisibleMemorySize == 0 {
		return nil, fmt.Errorf("invalid memory info")
	}

	return map[string]uint64{
		"total": item.TotalVisibleMemorySize / 1024,
		"used":  (item.TotalVisibleMemorySize - item.FreePhysicalMemory) / 1024,
	}, nil
}

func parseWindowsDiskInfoFromJSON(jsonText string) (map[string]uint64, error) {
	type diskInfo struct {
		Size      uint64 `json:"Size"`
		FreeSpace uint64 `json:"FreeSpace"`
	}

	trimmed := strings.TrimSpace(jsonText)
	if trimmed == "" {
		return nil, fmt.Errorf("empty disk info")
	}

	var list []diskInfo
	if err := json.Unmarshal([]byte(trimmed), &list); err != nil {
		var single diskInfo
		if err2 := json.Unmarshal([]byte(trimmed), &single); err2 != nil {
			return nil, err
		}
		list = []diskInfo{single}
	}

	var totalSize, totalFree uint64
	for _, item := range list {
		totalSize += item.Size
		totalFree += item.FreeSpace
	}
	if totalSize == 0 {
		return nil, fmt.Errorf("invalid disk info")
	}

	return map[string]uint64{
		"total": totalSize / 1024 / 1024 / 1024,
		"used":  (totalSize - totalFree) / 1024 / 1024 / 1024,
	}, nil
}

func parseWindowsLastBootUptime(raw string, now time.Time) (uint64, error) {
	value := strings.TrimSpace(raw)
	if len(value) < 14 {
		return 0, fmt.Errorf("invalid last boot time")
	}
	boot, err := time.ParseInLocation("20060102150405", value[:14], time.Local)
	if err != nil {
		return 0, err
	}
	if now.Before(boot) {
		return 0, nil
	}
	return uint64(now.Unix() - boot.Unix()), nil
}

func parseUintRelaxed(raw string) (uint64, error) {
	digitsOnly := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, raw)
	if digitsOnly == "" {
		return 0, fmt.Errorf("invalid uint value: %q", raw)
	}
	return strconv.ParseUint(digitsOnly, 10, 64)
}

func parseWindowsNetworkInfoFromJSON(jsonText string) (map[string]uint64, error) {
	type adapterStats struct {
		ReceivedBytes uint64 `json:"ReceivedBytes"`
		SentBytes     uint64 `json:"SentBytes"`
	}

	trimmed := strings.TrimSpace(jsonText)
	if trimmed == "" {
		return nil, fmt.Errorf("empty network info")
	}

	var list []adapterStats
	if err := json.Unmarshal([]byte(trimmed), &list); err != nil {
		var single adapterStats
		if err2 := json.Unmarshal([]byte(trimmed), &single); err2 != nil {
			return nil, err
		}
		list = []adapterStats{single}
	}

	var in, out uint64
	for _, item := range list {
		in += item.ReceivedBytes
		out += item.SentBytes
	}

	if in == 0 && out == 0 {
		return nil, fmt.Errorf("invalid network info")
	}

	return map[string]uint64{"in": in, "out": out}, nil
}

func parseWindowsNetworkTrafficFromNetstat(output string) (map[string]uint64, error) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !(strings.HasPrefix(line, "Bytes") || strings.HasPrefix(line, "字节")) {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		in, errIn := parseUintRelaxed(fields[len(fields)-2])
		out, errOut := parseUintRelaxed(fields[len(fields)-1])
		if errIn == nil && errOut == nil {
			return map[string]uint64{"in": in, "out": out}, nil
		}
	}

	return nil, fmt.Errorf("unable to parse netstat output")
}

// getCPUUsage 获取CPU使用率

func (s *systemService) getCPUUsage(ctx context.Context) (float64, error) {
	switch runtime.GOOS {
	case "linux":
		return s.getLinuxCPUUsage(ctx)
	case "darwin":
		return s.getMacOSCPUUsage(ctx)
	case "windows":
		return s.getWindowsCPUUsage(ctx)
	default:
		return 0, fmt.Errorf("不支持的操作系统")
	}
}

// getLinuxCPUUsage 获取Linux CPU使用率
func (s *systemService) getLinuxCPUUsage(ctx context.Context) (float64, error) {
	// 第一次读取
	stat1, err := s.readLinuxCPUStat(ctx)
	if err != nil {
		return 0, err
	}

	// 等待1秒
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-time.After(time.Second):
	}

	// 第二次读取
	stat2, err := s.readLinuxCPUStat(ctx)
	if err != nil {
		return 0, err
	}

	// 计算差值
	totalDiff := stat2.total - stat1.total
	idleDiff := stat2.idle - stat1.idle

	if totalDiff == 0 {
		return 0, nil
	}

	usage := float64(totalDiff-idleDiff) / float64(totalDiff) * 100
	return usage, nil
}

// cpuStat CPU统计信息结构
type cpuStat struct {
	total int64
	idle  int64
}

// readLinuxCPUStat 读取Linux CPU统计信息
func (s *systemService) readLinuxCPUStat(ctx context.Context) (*cpuStat, error) {
	cmd := exec.CommandContext(ctx, "cat", "/proc/stat")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("无法解析CPU信息")
	}

	// 解析第一行（总CPU信息）
	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return nil, fmt.Errorf("CPU信息格式错误")
	}

	var stat cpuStat
	for i := 1; i < len(fields); i++ {
		val, err := strconv.ParseInt(fields[i], 10, 64)
		if err != nil {
			continue
		}
		stat.total += val
		if i == 4 { // idle time是第4个字段
			stat.idle = val
		}
	}

	return &stat, nil
}

// getMacOSCPUUsage 获取macOS CPU使用率
func (s *systemService) getMacOSCPUUsage(ctx context.Context) (float64, error) {
	cmd := exec.CommandContext(ctx, "top", "-l", "1", "-n", "0")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "CPU usage") {
			// 解析CPU使用率
			fields := strings.Fields(line)
			for i, field := range fields {
				if strings.Contains(field, "%") && i > 0 {
					usage := strings.TrimSuffix(field, "%")
					return strconv.ParseFloat(usage, 64)
				}
			}
		}
	}

	return 0, fmt.Errorf("无法解析CPU使用率")
}

// getCPUModel 获取CPU型号
func (s *systemService) getCPUModel(ctx context.Context) (string, error) {
	switch runtime.GOOS {
	case "linux":
		cmd := exec.CommandContext(ctx, "cat", "/proc/cpuinfo")
		output, err := cmd.Output()
		if err != nil {
			return "", err
		}

		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "model name") {
				parts := strings.Split(line, ":")
				if len(parts) > 1 {
					return strings.TrimSpace(parts[1]), nil
				}
			}
		}
	case "darwin":
		cmd := exec.CommandContext(ctx, "sysctl", "-n", "machdep.cpu.brand_string")
		output, err := cmd.Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(output)), nil
	case "windows":
		output, err := runPowerShellCommand(ctx, "Get-CimInstance Win32_Processor | Select-Object -First 1 -ExpandProperty Name")
		if err == nil {
			if line, ok := parseFirstTextLine(output); ok {
				return line, nil
			}
		}

		cmd := exec.CommandContext(ctx, "wmic", "cpu", "get", "name")
		output, err = cmd.Output()
		if err != nil {
			return "", err
		}
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && line != "Name" {
				return line, nil
			}
		}
	}

	return "Unknown", nil
}

// getMemoryInfo 获取内存信息
func (s *systemService) getMemoryInfo(ctx context.Context) (map[string]uint64, error) {
	switch runtime.GOOS {
	case "linux":
		return s.getLinuxMemoryInfo(ctx)
	case "darwin":
		return s.getMacOSMemoryInfo(ctx)
	case "windows":
		return s.getWindowsMemoryInfo(ctx)
	default:
		return nil, fmt.Errorf("不支持的操作系统")
	}
}

// getLinuxMemoryInfo 获取Linux内存信息
func (s *systemService) getLinuxMemoryInfo(ctx context.Context) (map[string]uint64, error) {
	cmd := exec.CommandContext(ctx, "cat", "/proc/meminfo")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	memInfo := make(map[string]uint64)
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			key := strings.TrimSuffix(fields[0], ":")
			value, err := strconv.ParseUint(fields[1], 10, 64)
			if err == nil {
				memInfo[key] = value / 1024 // 转换为MB
			}
		}
	}

	result := make(map[string]uint64)
	if total, ok := memInfo["MemTotal"]; ok {
		result["total"] = total
	}
	if available, ok := memInfo["MemAvailable"]; ok {
		result["used"] = result["total"] - available
	} else if free, ok := memInfo["MemFree"]; ok {
		result["used"] = result["total"] - free
	}

	return result, nil
}

// getMacOSMemoryInfo 获取macOS内存信息
func (s *systemService) getMacOSMemoryInfo(ctx context.Context) (map[string]uint64, error) {
	// 获取总内存
	cmd := exec.CommandContext(ctx, "sysctl", "-n", "hw.memsize")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	totalBytes, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return nil, err
	}

	// 获取内存使用情况
	cmd = exec.CommandContext(ctx, "vm_stat")
	output, err = cmd.Output()
	if err != nil {
		return nil, err
	}

	// 解析vm_stat输出
	lines := strings.Split(string(output), "\n")
	var freePages, inactivePages uint64

	for _, line := range lines {
		if strings.Contains(line, "Pages free:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				freePages, _ = strconv.ParseUint(strings.TrimSuffix(fields[2], "."), 10, 64)
			}
		} else if strings.Contains(line, "Pages inactive:") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				inactivePages, _ = strconv.ParseUint(strings.TrimSuffix(fields[2], "."), 10, 64)
			}
		}
	}

	// 页面大小通常是4KB
	pageSize := uint64(4096)
	freeBytes := (freePages + inactivePages) * pageSize
	usedBytes := totalBytes - freeBytes

	result := map[string]uint64{
		"total": totalBytes / 1024 / 1024, // 转换为MB
		"used":  usedBytes / 1024 / 1024,  // 转换为MB
	}

	return result, nil
}

// getDiskInfo 获取磁盘信息
func (s *systemService) getDiskInfo(ctx context.Context) (map[string]uint64, error) {
	switch runtime.GOOS {
	case "darwin":
		return s.getMacOSDiskInfo(ctx)
	case "windows":
		return s.getWindowsDiskInfo(ctx)
	default:
		return s.getLinuxDiskInfo(ctx)
	}
}

// getMacOSDiskInfo 获取macOS磁盘信息，优先获取数据分区使用情况
func (s *systemService) getMacOSDiskInfo(ctx context.Context) (map[string]uint64, error) {
	// 获取所有挂载点信息
	cmd := exec.CommandContext(ctx, "df", "-k")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	var maxUsagePercent float64
	var maxUsageTotal, maxUsageUsed uint64

	// 查找使用率最高的重要分区
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 6 {
			mountPoint := fields[len(fields)-1]

			// 优先检查数据分区和根分区
			if mountPoint == "/System/Volumes/Data" || mountPoint == "/" {
				totalKB, err1 := strconv.ParseUint(fields[1], 10, 64)
				usedKB, err2 := strconv.ParseUint(fields[2], 10, 64)

				if err1 == nil && err2 == nil && totalKB > 0 {
					usagePercent := float64(usedKB) / float64(totalKB) * 100

					// 如果是数据分区，直接使用；如果是根分区，只有在没找到数据分区时才使用
					if mountPoint == "/System/Volumes/Data" || (mountPoint == "/" && maxUsagePercent == 0) {
						if usagePercent > maxUsagePercent || mountPoint == "/System/Volumes/Data" {
							maxUsagePercent = usagePercent
							maxUsageTotal = totalKB / 1024 / 1024 // 转换为GB
							maxUsageUsed = usedKB / 1024 / 1024   // 转换为GB
						}
					}
				}
			}
		}
	}

	// 如果没找到重要分区，使用默认的根分区
	if maxUsageTotal == 0 {
		return s.getDefaultDiskInfo(ctx)
	}

	s.l.Info("获取到macOS磁盘信息", zap.Float64("usage_percent", maxUsagePercent), zap.Uint64("total_gb", maxUsageTotal), zap.Uint64("used_gb", maxUsageUsed))

	return map[string]uint64{
		"total": maxUsageTotal,
		"used":  maxUsageUsed,
	}, nil
}

// getLinuxDiskInfo 获取Linux磁盘信息
func (s *systemService) getLinuxDiskInfo(ctx context.Context) (map[string]uint64, error) {
	cmd := exec.CommandContext(ctx, "df", "-h", "/")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("无法解析磁盘信息")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return nil, fmt.Errorf("磁盘信息格式错误")
	}

	total, err := s.parseSize(fields[1])
	if err != nil {
		return nil, err
	}
	used, err := s.parseSize(fields[2])
	if err != nil {
		return nil, err
	}

	return map[string]uint64{
		"total": total,
		"used":  used,
	}, nil
}

// getDefaultDiskInfo 获取默认磁盘信息（根分区）
func (s *systemService) getDefaultDiskInfo(ctx context.Context) (map[string]uint64, error) {
	cmd := exec.CommandContext(ctx, "df", "-k", "/")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("无法解析磁盘信息")
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		return nil, fmt.Errorf("磁盘信息格式错误")
	}

	totalKB, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return nil, err
	}
	usedKB, err := strconv.ParseUint(fields[2], 10, 64)
	if err != nil {
		return nil, err
	}

	return map[string]uint64{
		"total": totalKB / 1024 / 1024, // 转换为GB
		"used":  usedKB / 1024 / 1024,  // 转换为GB
	}, nil
}

// parseSize 解析大小字符串（如1.5G, 500M）
func (s *systemService) parseSize(sizeStr string) (uint64, error) {
	sizeStr = strings.TrimSpace(sizeStr)
	if len(sizeStr) == 0 {
		return 0, fmt.Errorf("空的大小字符串")
	}

	unit := sizeStr[len(sizeStr)-1:]
	valueStr := sizeStr[:len(sizeStr)-1]

	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, err
	}

	switch strings.ToUpper(unit) {
	case "K":
		return uint64(value / 1024), nil
	case "M":
		return uint64(value), nil
	case "G":
		return uint64(value * 1024), nil
	case "T":
		return uint64(value * 1024 * 1024), nil
	default:
		return uint64(value), nil
	}
}

// getLoadAverage 获取系统负载
func (s *systemService) getLoadAverage(ctx context.Context) ([]float64, error) {
	switch runtime.GOOS {
	case "linux":
		return s.getLinuxLoadAverage(ctx)
	case "darwin":
		return s.getMacOSLoadAverage(ctx)
	case "windows":
		return s.getWindowsLoadAverage(ctx)
	default:
		return []float64{0, 0, 0}, nil
	}
}

// getLinuxLoadAverage 获取Linux系统负载
func (s *systemService) getLinuxLoadAverage(ctx context.Context) ([]float64, error) {
	cmd := exec.CommandContext(ctx, "uptime")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// 解析uptime输出中的负载信息
	uptimeStr := string(output)
	if idx := strings.Index(uptimeStr, "load average:"); idx != -1 {
		loadStr := uptimeStr[idx+13:]
		loadStr = strings.TrimSpace(loadStr)

		loads := strings.Split(loadStr, ",")
		if len(loads) >= 3 {
			result := make([]float64, 3)
			for i := 0; i < 3; i++ {
				load, err := strconv.ParseFloat(strings.TrimSpace(loads[i]), 64)
				if err != nil {
					return nil, err
				}
				result[i] = load
			}
			return result, nil
		}
	}
	return []float64{0, 0, 0}, nil
}

// getMacOSLoadAverage 获取macOS系统负载
func (s *systemService) getMacOSLoadAverage(ctx context.Context) ([]float64, error) {
	// 使用sysctl获取负载平均值
	cmd := exec.CommandContext(ctx, "sysctl", "-n", "vm.loadavg")
	output, err := cmd.Output()
	if err != nil {
		s.l.Error("获取macOS负载失败，尝试备用方法", zap.Error(err))
		return s.getMacOSLoadAverageFromUptime(ctx)
	}

	loadStr := strings.TrimSpace(string(output))
	loadStr = strings.Trim(loadStr, "{}")
	loads := strings.Fields(loadStr)

	if len(loads) >= 3 {
		result := make([]float64, 3)
		for i := 0; i < 3; i++ {
			load, err := strconv.ParseFloat(strings.TrimSpace(loads[i]), 64)
			if err != nil {
				return nil, err
			}
			result[i] = load
		}
		return result, nil
	}

	return []float64{0, 0, 0}, nil
}

// getMacOSLoadAverageFromUptime macOS负载获取备用方法
func (s *systemService) getMacOSLoadAverageFromUptime(ctx context.Context) ([]float64, error) {
	cmd := exec.CommandContext(ctx, "uptime")
	output, err := cmd.Output()
	if err != nil {
		return []float64{0, 0, 0}, nil
	}

	uptimeStr := string(output)

	keywords := []string{"load averages:", "load average:"}
	for _, keyword := range keywords {
		if idx := strings.Index(uptimeStr, keyword); idx != -1 {
			loadStr := uptimeStr[idx+len(keyword):]
			loadStr = strings.TrimSpace(loadStr)

			loads := strings.Fields(loadStr)
			if len(loads) >= 3 {
				result := make([]float64, 3)
				for i := 0; i < 3; i++ {
					load, err := strconv.ParseFloat(strings.TrimSpace(loads[i]), 64)
					if err != nil {
						continue
					}
					result[i] = load
				}
				return result, nil
			}
		}
	}

	return []float64{0, 0, 0}, nil
}

// getUptime 获取系统运行时间
func (s *systemService) getUptime(ctx context.Context) (uint64, error) {
	switch runtime.GOOS {
	case "linux":
		return s.getLinuxUptime(ctx)
	case "darwin":
		return s.getMacOSUptime(ctx)
	case "windows":
		return s.getWindowsUptime(ctx)
	default:
		return 0, fmt.Errorf("不支持的操作系统")
	}
}

// getLinuxUptime 获取Linux系统运行时间
func (s *systemService) getLinuxUptime(ctx context.Context) (uint64, error) {
	cmd := exec.CommandContext(ctx, "cat", "/proc/uptime")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	fields := strings.Fields(string(output))
	if len(fields) > 0 {
		uptime, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			return 0, err
		}
		return uint64(uptime), nil
	}
	return 0, fmt.Errorf("无法解析Linux uptime")
}

// getMacOSUptime 获取macOS系统运行时间
func (s *systemService) getMacOSUptime(ctx context.Context) (uint64, error) {
	// 方法1: 使用sysctl获取启动时间
	cmd := exec.CommandContext(ctx, "sysctl", "-n", "kern.boottime")
	output, err := cmd.Output()
	if err != nil {
		s.l.Error("sysctl方法失败，尝试备用方法", zap.Error(err))
		return s.getMacOSUptimeFromUptime(ctx)
	}

	// 解析boottime输出 - 格式可能是: { sec = 1640995200, usec = 0 } Wed Jan 01 12:00:00 2022
	bootStr := strings.TrimSpace(string(output))

	// 寻找sec=后面的值
	if strings.Contains(bootStr, "sec =") {
		start := strings.Index(bootStr, "sec =") + 6
		end := strings.Index(bootStr[start:], ",")
		if end == -1 {
			end = strings.Index(bootStr[start:], " ")
		}
		if end != -1 {
			bootTimeStr := strings.TrimSpace(bootStr[start : start+end])
			bootTime, err := strconv.ParseInt(bootTimeStr, 10, 64)
			if err == nil {
				uptime := time.Now().Unix() - bootTime
				return uint64(uptime), nil
			}
		}
	}

	// 如果上面方法失败，尝试旧格式解析
	fields := strings.Fields(bootStr)
	if len(fields) >= 4 {
		bootTime, err := strconv.ParseInt(fields[3], 10, 64)
		if err == nil {
			uptime := time.Now().Unix() - bootTime
			return uint64(uptime), nil
		}
	}

	s.l.Error("解析boottime失败，尝试备用方法")
	return s.getMacOSUptimeFromUptime(ctx)
}

// getMacOSUptimeFromUptime macOS uptime备用方法
func (s *systemService) getMacOSUptimeFromUptime(ctx context.Context) (uint64, error) {
	cmd := exec.CommandContext(ctx, "uptime")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	uptimeStr := string(output)
	// 解析uptime输出，查找"up"关键字
	if strings.Contains(uptimeStr, " up ") {
		// 尝试解析各种格式
		// 格式1: "up 5 days"
		// 格式2: "up 1:23"
		// 格式3: "up 1 day, 2:34"

		parts := strings.Split(uptimeStr, " up ")
		if len(parts) > 1 {
			timePart := strings.Split(parts[1], ",")[0] // 取逗号前的部分
			timePart = strings.TrimSpace(timePart)

			// 简单的小时:分钟格式解析
			if strings.Contains(timePart, ":") {
				timeParts := strings.Split(timePart, ":")
				if len(timeParts) == 2 {
					hours, err1 := strconv.Atoi(timeParts[0])
					minutes, err2 := strconv.Atoi(timeParts[1])
					if err1 == nil && err2 == nil {
						return uint64(hours*3600 + minutes*60), nil
					}
				}
			}
		}
	}

	// 如果解析失败，返回0但不报错
	s.l.Warn("无法解析macOS uptime，返回0")
	return 0, nil
}

// getProcessCount 获取进程数
func (s *systemService) getProcessCount(ctx context.Context) (int, error) {
	switch runtime.GOOS {
	case "windows":
		return s.getWindowsProcessCount(ctx)
	default:
		cmd := exec.CommandContext(ctx, "ps", "aux")
		output, err := cmd.Output()
		if err != nil {
			return 0, err
		}

		lines := strings.Split(string(output), "\n")
		count := len(lines) - 2
		if count < 0 {
			count = 0
		}
		return count, nil
	}
}

func parseWindowsProcessCount(output []byte) (int, error) {
	if line, ok := parseFirstTextLine(output); ok {
		return strconv.Atoi(strings.TrimSpace(line))
	}
	return 0, fmt.Errorf("invalid process count output")
}

func (s *systemService) getWindowsProcessCount(ctx context.Context) (int, error) {
	output, err := runPowerShellCommand(ctx, "(Get-Process | Measure-Object).Count")
	if err == nil {
		if count, parseErr := parseWindowsProcessCount(output); parseErr == nil {
			return count, nil
		}
	}

	cmd := exec.CommandContext(ctx, "tasklist")
	output, err = cmd.Output()
	if err != nil {
		return 0, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) <= 3 {
		return 0, fmt.Errorf("invalid tasklist output")
	}
	return len(lines) - 3, nil
}

// getOSVersion 获取操作系统版本
func (s *systemService) getOSVersion(ctx context.Context) (string, error) {
	switch runtime.GOOS {
	case "linux":
		// 尝试读取 /etc/os-release
		cmd := exec.CommandContext(ctx, "cat", "/etc/os-release")
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "PRETTY_NAME=") {
					version := strings.TrimPrefix(line, "PRETTY_NAME=")
					version = strings.Trim(version, "\"")
					return version, nil
				}
			}
		}

		// 备选方案：读取 /proc/version
		cmd = exec.CommandContext(ctx, "uname", "-r")
		output, err = cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(output)), nil
		}
	case "darwin":
		cmd := exec.CommandContext(ctx, "sw_vers", "-productVersion")
		output, err := cmd.Output()
		if err == nil {
			return "macOS " + strings.TrimSpace(string(output)), nil
		}
	case "windows":
		cmd := exec.CommandContext(ctx, "cmd", "/c", "ver")
		output, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(output)), nil
		}
	}

	return "Unknown", nil
}

// getNetworkTraffic 获取网络流量信息
func (s *systemService) getNetworkTraffic(ctx context.Context) (map[string]uint64, error) {
	switch runtime.GOOS {
	case "linux":
		return s.getLinuxNetworkTraffic(ctx)
	case "darwin":
		return s.getMacOSNetworkTraffic(ctx)
	case "windows":
		return s.getWindowsNetworkTraffic(ctx)
	default:
		return map[string]uint64{"in": 0, "out": 0}, nil
	}
}

// getLinuxNetworkTraffic 获取Linux网络流量信息
func (s *systemService) getLinuxNetworkTraffic(ctx context.Context) (map[string]uint64, error) {
	cmd := exec.CommandContext(ctx, "cat", "/proc/net/dev")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	var totalRxBytes, totalTxBytes uint64

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, ":") && !strings.HasPrefix(line, "Inter-") && !strings.HasPrefix(line, "face") {
			// 跳过 lo (loopback) 接口
			if strings.HasPrefix(line, "lo:") {
				continue
			}

			parts := strings.Split(line, ":")
			if len(parts) != 2 {
				continue
			}

			fields := strings.Fields(parts[1])
			if len(fields) >= 9 {
				// 接收字节数 (第1列)
				rxBytes, err := strconv.ParseUint(fields[0], 10, 64)
				if err == nil {
					totalRxBytes += rxBytes
				}

				// 发送字节数 (第9列)
				txBytes, err := strconv.ParseUint(fields[8], 10, 64)
				if err == nil {
					totalTxBytes += txBytes
				}
			}
		}
	}

	result := map[string]uint64{
		"in":  totalRxBytes / 1024 / 1024, // 转换为MB
		"out": totalTxBytes / 1024 / 1024, // 转换为MB
	}

	return result, nil
}

// getMacOSNetworkTraffic 获取macOS网络流量信息
func (s *systemService) getMacOSNetworkTraffic(ctx context.Context) (map[string]uint64, error) {
	// 使用 netstat -ib 命令获取网络接口统计信息
	cmd := exec.CommandContext(ctx, "netstat", "-ib")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	var totalRxBytes, totalTxBytes uint64

	for i, line := range lines {
		// 跳过表头
		if i == 0 {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 10 {
			// 跳过 lo0 (loopback) 接口
			if strings.HasPrefix(fields[0], "lo0") {
				continue
			}

			// 检查是否是物理网络接口（如 en0, en1）
			if strings.HasPrefix(fields[0], "en") || strings.HasPrefix(fields[0], "eth") {
				// 接收字节数 (第7列)
				rxBytes, err := strconv.ParseUint(fields[6], 10, 64)
				if err == nil {
					totalRxBytes += rxBytes
				}

				// 发送字节数 (第10列)
				txBytes, err := strconv.ParseUint(fields[9], 10, 64)
				if err == nil {
					totalTxBytes += txBytes
				}
			}
		}
	}

	result := map[string]uint64{
		"in":  totalRxBytes,
		"out": totalTxBytes,
	}

	return result, nil
}

// getWindowsCPUUsage 获取Windows CPU使用率

func (s *systemService) getWindowsCPUUsage(ctx context.Context) (float64, error) {
	parseSingleValue := func(output []byte) (float64, bool) {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.Contains(line, "CLIXML") {
				continue
			}
			line = strings.Trim(line, "\"'")
			if v, err := strconv.ParseFloat(line, 64); err == nil && v >= 0 {
				return v, true
			}
		}
		return 0, false
	}

	// 优先使用PowerShell CIM（兼容新Windows，wmic已在新版本移除）
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", "Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average | Select-Object -ExpandProperty Average")
	if output, err := cmd.Output(); err == nil {
		if usage, ok := parseSingleValue(output); ok {
			return usage, nil
		}
	}

	// 回退到 typeperf 性能计数器
	cmd = exec.CommandContext(ctx, "typeperf", `\\Processor Information(_Total)\\% Processor Time`, "-sc", "1")
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if !strings.Contains(line, `","`) {
				continue
			}
			parts := strings.Split(line, `","`)
			if len(parts) < 2 {
				continue
			}
			valueStr := strings.Trim(parts[len(parts)-1], "\" ")
			if usage, err := strconv.ParseFloat(valueStr, 64); err == nil && usage >= 0 {
				return usage, nil
			}
		}
	}

	// 最后尝试旧版WMIC（兼容旧系统）
	cmd = exec.CommandContext(ctx, "wmic", "cpu", "get", "loadpercentage")
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.EqualFold(line, "LoadPercentage") {
				continue
			}
			if usage, err := strconv.ParseFloat(line, 64); err == nil && usage >= 0 {
				return usage, nil
			}
		}
	}

	return 0, fmt.Errorf("无法获取Windows CPU使用率")
}

// getWindowsMemoryInfo 获取Windows内存信息
func (s *systemService) getWindowsMemoryInfo(ctx context.Context) (map[string]uint64, error) {
	output, err := runPowerShellCommand(ctx, "Get-CimInstance Win32_OperatingSystem | Select-Object -First 1 FreePhysicalMemory,TotalVisibleMemorySize | ConvertTo-Json -Compress")
	if err == nil {
		if parsed, parseErr := parseWindowsMemoryInfoFromJSON(string(output)); parseErr == nil {
			return parsed, nil
		}
	}

	cmd := exec.CommandContext(ctx, "wmic", "OS", "get", "FreePhysicalMemory,TotalVisibleMemorySize", "/Value")
	output, err = cmd.Output()
	if err != nil {
		return nil, err
	}

	result := make(map[string]uint64)
	var free, total uint64
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "FreePhysicalMemory=") {
			valStr := strings.TrimPrefix(line, "FreePhysicalMemory=")
			free, _ = strconv.ParseUint(valStr, 10, 64)
		} else if strings.HasPrefix(line, "TotalVisibleMemorySize=") {
			valStr := strings.TrimPrefix(line, "TotalVisibleMemorySize=")
			total, _ = strconv.ParseUint(valStr, 10, 64)
		}
	}

	if total > 0 {
		result["total"] = total / 1024
		result["used"] = (total - free) / 1024
	}
	return result, nil
}

// getWindowsDiskInfo 获取Windows磁盘信息
func (s *systemService) getWindowsDiskInfo(ctx context.Context) (map[string]uint64, error) {
	output, err := runPowerShellCommand(ctx, "Get-CimInstance Win32_LogicalDisk -Filter \"DriveType=3\" | Select-Object Size,FreeSpace | ConvertTo-Json -Compress")
	if err == nil {
		if parsed, parseErr := parseWindowsDiskInfoFromJSON(string(output)); parseErr == nil {
			return parsed, nil
		}
	}

	cmd := exec.CommandContext(ctx, "wmic", "logicaldisk", "where", "drivetype=3", "get", "size,freespace", "/Value")
	output, err = cmd.Output()
	if err != nil {
		return nil, err
	}

	var totalSize, totalFree uint64

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "FreeSpace=") {
			valStr := strings.TrimPrefix(line, "FreeSpace=")
			val, _ := strconv.ParseUint(valStr, 10, 64)
			totalFree += val
		} else if strings.HasPrefix(line, "Size=") {
			valStr := strings.TrimPrefix(line, "Size=")
			val, _ := strconv.ParseUint(valStr, 10, 64)
			totalSize += val
		}
	}

	result := make(map[string]uint64)
	if totalSize > 0 {
		result["total"] = totalSize / 1024 / 1024 / 1024
		result["used"] = (totalSize - totalFree) / 1024 / 1024 / 1024
	}
	return result, nil
}

// getWindowsLoadAverage 获取Windows系统负载
func (s *systemService) getWindowsLoadAverage(ctx context.Context) ([]float64, error) {
	// Windows不直接提供load average, 用CPU使用率替代或者返回0
	usage, _ := s.getWindowsCPUUsage(ctx)
	// 转换成类似负载的形式，非精确
	return []float64{usage / 100.0, usage / 100.0, usage / 100.0}, nil
}

// getWindowsUptime 获取Windows系统运行时间
func (s *systemService) getWindowsUptime(ctx context.Context) (uint64, error) {
	output, err := runPowerShellCommand(ctx, "Get-CimInstance Win32_OperatingSystem | Select-Object -First 1 -ExpandProperty LastBootUpTime")
	if err == nil {
		if line, ok := parseFirstTextLine(output); ok {
			if uptime, parseErr := parseWindowsLastBootUptime(line, time.Now()); parseErr == nil {
				return uptime, nil
			}
		}
	}

	cmd := exec.CommandContext(ctx, "wmic", "os", "get", "lastbootuptime", "/Value")
	output, err = cmd.Output()
	if err != nil {
		return 0, err
	}
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "LastBootUpTime=") {
			val := strings.TrimPrefix(line, "LastBootUpTime=")
			if uptime, parseErr := parseWindowsLastBootUptime(val, time.Now()); parseErr == nil {
				return uptime, nil
			}
		}
	}
	return 0, nil
}

// getWindowsNetworkTraffic 获取Windows网络流量
func (s *systemService) getWindowsNetworkTraffic(ctx context.Context) (map[string]uint64, error) {
	output, err := runPowerShellCommand(ctx, "Get-NetAdapterStatistics | Select-Object ReceivedBytes,SentBytes | ConvertTo-Json -Compress")
	if err == nil {
		if parsed, parseErr := parseWindowsNetworkInfoFromJSON(string(output)); parseErr == nil {
			return parsed, nil
		}
	}

	cmd := exec.CommandContext(ctx, "netstat", "-e")
	output, err = cmd.Output()
	if err != nil {
		return nil, err
	}

	parsed, parseErr := parseWindowsNetworkTrafficFromNetstat(string(output))
	if parseErr != nil {
		return nil, parseErr
	}
	return parsed, nil
}
