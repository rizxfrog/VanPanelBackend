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

package di

import (
	"time"

	"github.com/hibiken/asynq"
	"go.uber.org/zap"
	"gorm.io/gorm"

	agentService "github.com/rizxfrog/VanPanelBackend/internal/agent/service"
	cronDao "github.com/rizxfrog/VanPanelBackend/internal/cron/dao"
	cronManager "github.com/rizxfrog/VanPanelBackend/internal/cron/manager"
	cronServer "github.com/rizxfrog/VanPanelBackend/internal/cron/server"
	cronService "github.com/rizxfrog/VanPanelBackend/internal/cron/service"
)

// ProvideCronDAO 提供 Cron DAO
func ProvideCronDAO(db *gorm.DB) *cronDao.CronDAO {
	return cronDao.NewCronDAO(db)
}

// ProvideCronService 提供 Cron 服务；DB 不可用时返回 nil
func ProvideCronService(
	dao *cronDao.CronDAO,
	agentSvc agentService.AgentService,
	logger *zap.Logger,
	client *asynq.Client,
) *cronService.CronService {
	if dao == nil {
		return nil
	}
	return cronService.NewCronService(dao, agentSvc, logger, client)
}

// ProvideCronManager 提供 Cron 调度管理器
func ProvideCronManager(svc *cronService.CronService, logger *zap.Logger) *cronManager.Manager {
	if svc == nil {
		return nil
	}
	mgr := cronManager.NewManager(svc, 10*time.Second, logger)
	svc.SetManager(mgr)
	return mgr
}

// ProvideCronAsynqClient 提供 Asynq Client
func ProvideCronAsynqClient() *asynq.Client {
	return cronServer.NewCronAsynqClient()
}

// ProvideCronAsynqServer 提供 Asynq Server 和 ServeMux 封装
func ProvideCronAsynqServer(svc *cronService.CronService, logger *zap.Logger) *cronServer.CronAsynqServer {
	if svc == nil {
		return nil
	}
	return cronServer.NewCronAsynqServer(svc, logger)
}
