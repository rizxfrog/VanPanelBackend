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

package server

import (
	"github.com/hibiken/asynq"
	"github.com/rizxfrog/VanPanelBackend/internal/cron/service"
	"github.com/rizxfrog/VanPanelBackend/internal/cron/tasks"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// CronAsynqServer 封装 Asynq Server 和 ServeMux
type CronAsynqServer struct {
	Server *asynq.Server
	Mux    *asynq.ServeMux
}

// NewCronAsynqServer 创建注册好 cron 任务处理器的 Asynq Server 和 ServeMux
func NewCronAsynqServer(svc *service.CronService, logger *zap.Logger) *CronAsynqServer {
	mux := asynq.NewServeMux()
	mux.HandleFunc(tasks.TypeCronExecute, tasks.HandleExecuteTask(svc, logger))

	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     viper.GetString("redis.addr"),
			Password: viper.GetString("redis.password"),
		},
		asynq.Config{
			Concurrency: 10,
		},
	)
	return &CronAsynqServer{Server: srv, Mux: mux}
}

// NewCronAsynqClient 创建 Asynq Client
func NewCronAsynqClient() *asynq.Client {
	return asynq.NewClient(asynq.RedisClientOpt{
		Addr:     viper.GetString("redis.addr"),
		Password: viper.GetString("redis.password"),
	})
}
