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
