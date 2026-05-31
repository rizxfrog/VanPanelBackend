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

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"github.com/rizxfrog/VanPanelBackend/mock"
	"github.com/rizxfrog/VanPanelBackend/pkg/base"
	"github.com/rizxfrog/VanPanelBackend/pkg/di"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("startup failed: %v", err)
	}
}

func run() error {
	_ = godotenv.Load()
	if err := di.InitViper(); err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}

	cmd := di.ProvideCmd()
	db := di.InitDB()

	if db != nil && di.CheckDBHealth(db) == nil {
		log.Printf("database health check passed")
	} else {
		log.Printf("database unavailable, running in degraded mode")
	}

	if di.IsDBAvailable(db) {
		if err := cmd.Bootstrap.InitializeK8sClients(context.Background()); err != nil {
			log.Printf("failed to initialize k8s clients: %v", err)
		}
	}

	cmd.Server.Use(gzip.Gzip(gzip.BestCompression))

	cmd.Server.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "AI-CloudOps API service is running",
			"status":  "running",
		})
	})

	cmd.Server.POST("/api/v1/debug/test", func(c *gin.Context) {
		log.Printf("DEBUG test request method=%s path=%s", c.Request.Method, c.Request.URL.Path)
		c.JSON(http.StatusOK, gin.H{
			"message": "test request received",
			"method":  c.Request.Method,
			"path":    c.Request.URL.Path,
			"time":    time.Now(),
		})
	})

	if viper.GetBool("mock.enabled") && di.IsDBAvailable(db) {
		if err := initMock(db); err != nil {
			log.Printf("mock initialization failed: %v", err)
		}
	} else if viper.GetBool("mock.enabled") {
		log.Printf("database unavailable, skipping mock initialization")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if di.IsDBAvailable(db) {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Asynq server panic: %v", r)
				}
			}()

			mux := asynq.NewServeMux()
			mux.Handle("cron:task", cmd.CronHandlers)

			log.Printf("starting Asynq server")
			if err := cmd.AsynqServer.Run(mux); err != nil {
				log.Printf("Asynq server failed: %v", err)
			}
		}()

		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Asynq scheduler panic: %v", r)
				}
			}()

			log.Printf("starting Asynq scheduler")
			if err := cmd.Scheduler.Run(); err != nil {
				log.Printf("Asynq scheduler failed: %v", err)
			}
		}()

		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Unified cron manager panic: %v", r)
				}
			}()

			log.Printf("starting unified cron manager")
			if err := cmd.CronManager.Start(ctx); err != nil {
				log.Printf("unified cron manager failed: %v", err)
			}
		}()

		log.Printf("system startup completed with Asynq and unified cron manager")
	} else {
		log.Printf("running in degraded mode")
	}

	srv := &http.Server{
		Addr:    ":" + viper.GetString("server.port"),
		Handler: cmd.Server,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		showBootInfo(viper.GetString("server.port"))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	<-quit
	log.Println("shutting down server")

	if di.IsDBAvailable(db) {
		log.Println("stopping cron manager and Asynq services")

		stopCtx, stopCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer stopCancel()
		if err := cmd.CronManager.Stop(stopCtx); err != nil {
			log.Printf("cron manager stop timeout: %v", err)
		}

		cmd.AsynqServer.Shutdown()
		cmd.Scheduler.Shutdown()
	}

	cancel()

	shutdownCtx, shutdownCancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
	time.Sleep(2 * time.Second)
	log.Println("server stopped")
	return nil
}

func initMock(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %v", err)
	}

	if err := mock.NewApiMock(db).InitApi(); err != nil {
		return fmt.Errorf("failed to initialize API mock: %v", err)
	}
	if err := mock.NewUserMock(db).CreateUserAdmin(); err != nil {
		return fmt.Errorf("failed to create admin user: %v", err)
	}

	log.Printf("mock initialization completed")
	return nil
}

func showBootInfo(port string) {
	ips, _ := base.GetLocalIPs()
	color.Green("AI-CloudOps API service started")
	fmt.Printf("%s  ", color.GreenString("->"))
	fmt.Printf("%s    ", color.New(color.Bold).Sprint("Local:"))
	fmt.Printf("%s\n", color.MagentaString("http://localhost:%s/", port))
	for _, ip := range ips {
		fmt.Printf("%s  ", color.GreenString("->"))
		fmt.Printf("%s  ", color.New(color.Bold).Sprint("Network:"))
		fmt.Printf("%s\n", color.MagentaString("http://%s:%s/", ip, port))
	}
}
