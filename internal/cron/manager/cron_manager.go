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
 */

package manager

import (
	"context"
	"sync"
	"time"

	"github.com/rizxfrog/VanPanelBackend/internal/cron/service"
	"go.uber.org/zap"
)

// Manager 是一个基于内存的定时任务调度器，定期从数据库加载启用的任务，
// 对到期的任务触发执行。适用于单实例部署。
type Manager struct {
	service  *service.CronService
	interval time.Duration
	logger   *zap.Logger

	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running sync.Map
	reload  chan struct{}

	enabled  bool
	mu       sync.Mutex
	nextWake time.Time
}

// NewManager 创建调度器
func NewManager(svc *service.CronService, interval time.Duration, logger *zap.Logger) *Manager {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &Manager{
		service:  svc,
		interval: interval,
		logger:   logger,
		reload:   make(chan struct{}, 1),
	}
}

// Start 启动调度循环
func (m *Manager) Start(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.enabled {
		return
	}
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.enabled = true
	m.nextWake = time.Now().Add(m.interval)

	m.wg.Add(1)
	go m.loop(ctx)
	m.logger.Info("cron manager started")
}

// Stop 停止调度循环
func (m *Manager) Stop() {
	m.mu.Lock()
	cancel := m.cancel
	m.enabled = false
	m.nextWake = time.Time{}
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	m.wg.Wait()
	m.logger.Info("cron manager stopped")
}

// Reload 通知调度器尽快刷新
func (m *Manager) Reload() {
	select {
	case m.reload <- struct{}{}:
	default:
	}
}

// Enabled 返回调度器是否正在运行
func (m *Manager) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

// NextWake 返回下次唤醒时间
func (m *Manager) NextWake() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nextWake
}

func (m *Manager) setNextWake(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextWake = t
}

func (m *Manager) loop(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.tick(ctx)
		case <-m.reload:
			m.tick(ctx)
		}
		m.setNextWake(time.Now().Add(m.interval))
	}
}

func (m *Manager) tick(ctx context.Context) {
	jobs, err := m.service.LoadDueJobs(ctx)
	if err != nil {
		m.logger.Error("加载到期任务失败", zap.Error(err))
		return
	}
	if len(jobs) == 0 {
		return
	}
	for _, job := range jobs {
		jobID := job.ID
		if _, loaded := m.running.LoadOrStore(jobID, true); loaded {
			continue
		}
		m.wg.Add(1)
		go func(id string) {
			defer m.wg.Done()
			defer m.running.Delete(id)
			execCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()
			if err := m.service.ExecuteJob(execCtx, id, time.Now()); err != nil {
				m.logger.Error("执行定时任务失败", zap.String("job_id", id), zap.Error(err))
			}
		}(jobID)
	}
}
