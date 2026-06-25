package main

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"
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
	fmt.Printf("driver=%s dsn=%s\n", driver, dsn)

	if driver == "" {
		legacyMySQLDSN := viper.GetString("mysql.addr")
		if legacyMySQLDSN != "" {
			driver = "mysql"
			dsn = legacyMySQLDSN
			fmt.Printf("using legacy mysql dsn\n")
		}
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

	var tables []string
	if err := db.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_name LIKE 'cl_agent_cron%'").Scan(&tables).Error; err != nil {
		log.Fatalf("failed to list tables: %v", err)
	}
	fmt.Printf("tables: %v\n", tables)

	var count int64
	if err := db.Raw("SELECT count(*) FROM cl_agent_cron_jobs").Scan(&count).Error; err != nil {
		fmt.Printf("count error: %v\n", err)
	} else {
		fmt.Printf("job count: %d\n", count)
	}
}
