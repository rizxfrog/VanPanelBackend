package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	mcpserver "github.com/rizxfrog/VanPanelBackend/internal/agent/mcp/server"
	"github.com/rizxfrog/VanPanelBackend/internal/gateway"
	gatewayRpc "github.com/rizxfrog/VanPanelBackend/internal/gateway/rpc"
	"github.com/rizxfrog/VanPanelBackend/mock"
	"github.com/rizxfrog/VanPanelBackend/pkg/base"
	"github.com/rizxfrog/VanPanelBackend/pkg/di"
	"github.com/spf13/viper"
	"go.uber.org/zap"
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

	cmd, err := di.ProvideCmd()
	if err != nil {
		return fmt.Errorf("failed to initialize DI: %v", err)
	}
	if cmd.AgentService != nil {
		gatewayRpc.SetAgentService(cmd.AgentService)
	}
	if cmd.ConfigService != nil {
		gatewayRpc.SetConfigService(cmd.ConfigService)
	}
	if cmd.CronService != nil {
		gatewayRpc.SetCronService(cmd.CronService)
	}
	db := di.InitDB()

	if db != nil && di.CheckDBHealth(db) == nil {
		log.Printf("database health check passed")
		// 重启后应用 DB 中的运行时配置覆盖（DB > YAML/env）
		if err := gatewayRpc.LoadRuntimeConfig(context.Background()); err != nil {
			log.Printf("load runtime config failed: %v", err)
		}
	} else {
		log.Printf("database unavailable, running in degraded mode")
	}

	cmd.Server.Use(gzip.Gzip(gzip.BestCompression))

	// Set up the gateway server
	gatewayLogger, _ := zap.NewDevelopment()
	gatewaySrv := setupGateway(gatewayLogger)

	// Register embedded OpenClaw Web UI + WebSocket gateway upgrade
	registerWebUI(cmd.Server, gatewaySrv)

	// MCP server mode: activated via config mcp.serve=true or --mcp-serve env
	if viper.GetBool("mcp.serve") {
		if cmd.ToolManager == nil {
			log.Fatalf("MCP server requires ToolManager, but it was not provided by DI")
		}

		transport := viper.GetString("mcp.transport")
		if transport == "" {
			transport = "stdio"
		}
		port := viper.GetInt("mcp.port")
		if port == 0 {
			port = 8890
		}

		mcpLogger, _ := zap.NewDevelopment()
		if err := mcpserver.Serve(
			context.Background(),
			mcpserver.ServeOptions{Transport: transport, Port: port},
			cmd.ToolManager,
			nil, // riskEval to be wired separately
			mcpLogger,
		); err != nil {
			log.Fatalf("MCP server failed: %v", err)
		}
		return nil
	}

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

	// 启动 Cron 调度管理器和 Asynq 任务消费端
	if cmd.CronManager != nil && db != nil && di.CheckDBHealth(db) == nil {
		cmd.CronManager.Start(context.Background())
		log.Printf("cron manager started")
	}
	if cmd.CronAsynqServer != nil {
		go func() {
			if err := cmd.CronAsynqServer.Server.Run(cmd.CronAsynqServer.Mux); err != nil {
				log.Printf("cron asynq server error: %v", err)
			}
		}()
		log.Printf("cron asynq server started")
	}

	log.Printf("system startup completed")

	srv := &http.Server{
		Addr:    ":" + viper.GetString("server.port"),
		Handler: cmd.Server,
	}

	quit := make(chan struct{})
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	go func() {
		showBootInfo(viper.GetString("server.port"))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
		close(quit)
	}()

	<-ctx.Done()
	log.Println("shutting down server")

	if cmd.CronManager != nil {
		cmd.CronManager.Stop()
		log.Println("cron manager stopped")
	}
	if cmd.CronAsynqServer != nil {
		cmd.CronAsynqServer.Server.Stop()
		log.Println("cron asynq server stopped")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
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

// setupGateway creates the gateway server with all components.
func setupGateway(logger *zap.Logger) *gateway.GatewayServer {
	config := &gateway.GatewayConfig{
		ServerVersion: "vanpanel-0.1.0",
	}

	// Create components
	broadcastMgr := gateway.NewBroadcastManager(logger)
	presenceTracker := gateway.NewPresenceTracker(logger)
	healthState := gateway.NewHealthState(logger, config.ServerVersion)
	healthState.Start()

	// Auth handler with "none" mode for local development
	authHandler := gateway.NewAuthHandler(logger, nil, "none", "", "")

	// Create RunTracker and SubscriptionHub for RPC handlers
	runTracker := gateway.NewRunTracker()
	subHub := gateway.NewSubscriptionHub()

	// Pass infrastructure to RPC package
	gatewayRpc.SetRunTracker(runTracker)
	gatewayRpc.SetSubscriptionHub(subHub)
	gatewayRpc.SetBroadcastManager(broadcastMgr)

	config.Methods = gateway.GetRegisteredMethods()
	config.Events = gateway.GetRegisteredEvents()

	gwServer := gateway.NewGatewayServer(
		logger,
		broadcastMgr,
		presenceTracker,
		healthState,
		authHandler,
		config,
		runTracker,
		subHub,
	)

	gatewayServerInstance = gwServer
	return gwServer
}
