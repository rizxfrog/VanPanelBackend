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

package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rizxfrog/VanPanelBackend/internal/cron/service"
	"go.uber.org/zap"
)

// TypeCronExecute 定时任务执行 Asynq 任务类型
const TypeCronExecute = "cron:execute"

// ExecutePayload 任务执行负载
type ExecutePayload struct {
	JobID       string    `json:"job_id"`
	TriggeredAt time.Time `json:"triggered_at"`
}

// NewExecuteTask 创建执行任务的 Asynq Task
func NewExecuteTask(jobID string, triggeredAt time.Time) (*asynq.Task, error) {
	payload := ExecutePayload{
		JobID:       jobID,
		TriggeredAt: triggeredAt,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal execute payload: %w", err)
	}
	return asynq.NewTask(TypeCronExecute, data), nil
}

// HandleExecuteTask 返回处理 cron:execute 的 Asynq Handler
func HandleExecuteTask(svc *service.CronService, logger *zap.Logger) asynq.HandlerFunc {
	return func(ctx context.Context, t *asynq.Task) error {
		var payload ExecutePayload
		if err := json.Unmarshal(t.Payload(), &payload); err != nil {
			return fmt.Errorf("unmarshal execute payload: %w", err)
		}
		if payload.JobID == "" {
			return fmt.Errorf("job_id is empty")
		}
		triggeredAt := payload.TriggeredAt
		if triggeredAt.IsZero() {
			triggeredAt = time.Now()
		}
		if err := svc.ExecuteJob(ctx, payload.JobID, triggeredAt); err != nil {
			logger.Error("cron execute task failed", zap.String("job_id", payload.JobID), zap.Error(err))
			return err
		}
		return nil
	}
}
