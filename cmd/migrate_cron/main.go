package main

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/rizxfrog/VanPanelBackend/internal/model"
	"github.com/rizxfrog/VanPanelBackend/pkg/di"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	_ = godotenv.Load()
	if err := di.InitViper(); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	driver := viper.GetString("database.driver")
	dsn := viper.GetString("database.dsn")
	if driver == "" {
		legacyMySQLDSN := viper.GetString("mysql.addr")
		if legacyMySQLDSN != "" {
			driver = "mysql"
			dsn = legacyMySQLDSN
		}
	}
	if driver == "" || dsn == "" {
		log.Fatalf("database config not found")
	}

	var db *gorm.DB
	var err error
	switch driver {
	case "mysql":
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case "postgres":
		db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	default:
		log.Fatalf("unsupported driver: %s", driver)
	}
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	if err := db.AutoMigrate(&model.CronJob{}, &model.CronRun{}); err != nil {
		log.Fatalf("failed to migrate cron tables: %v", err)
	}
	fmt.Println("cron tables migrated successfully")
}
