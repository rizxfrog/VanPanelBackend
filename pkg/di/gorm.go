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

package di

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func InitDB() *gorm.DB {
	cfg, err := resolveDatabaseConfig()
	if err != nil {
		log.Printf("failed to resolve database config: %v", err)
		return nil
	}

	dialector, err := newDialector(cfg)
	if err != nil {
		log.Printf("failed to create database dialector: %v", err)
		return nil
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		NamingStrategy: schema.NamingStrategy{},
	})
	if err != nil {
		log.Printf("database connection failed: %v", err)
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Printf("failed to get sql.DB: %v", err)
		return nil
	}
	if err := sqlDB.Ping(); err != nil {
		log.Printf("database ping failed: %v", err)
		return nil
	}
	if err := InitTables(db); err != nil {
		log.Printf("failed to initialize database tables: %v", err)
	}
	return db
}

func resolveDatabaseConfig() (DatabaseConfig, error) {
	driver := strings.TrimSpace(viper.GetString("database.driver"))
	dsn := strings.TrimSpace(viper.GetString("database.dsn"))

	legacyMySQLDSN := strings.TrimSpace(viper.GetString("mysql.addr"))
	if driver == "" && legacyMySQLDSN != "" {
		driver = "mysql"
	}
	if dsn == "" && legacyMySQLDSN != "" {
		dsn = legacyMySQLDSN
	}

	if driver == "" {
		driver = "mysql"
	}

	driver = strings.ToLower(driver)
	switch driver {
	case "mysql", "postgres", "postgresql":
	default:
		return DatabaseConfig{}, fmt.Errorf("unsupported database driver %q", driver)
	}

	if driver == "postgresql" {
		driver = "postgres"
	}
	if dsn == "" {
		return DatabaseConfig{}, fmt.Errorf("database dsn is required")
	}

	return DatabaseConfig{
		Driver: driver,
		DSN:    dsn,
	}, nil
}

func newDialector(cfg DatabaseConfig) (gorm.Dialector, error) {
	switch cfg.Driver {
	case "mysql":
		return mysql.Open(cfg.DSN), nil
	case "postgres":
		return postgres.Open(cfg.DSN), nil
	default:
		return nil, fmt.Errorf("unsupported database driver %q", cfg.Driver)
	}
}

func CheckDBHealth(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database connection is nil")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %v", err)
	}

	var pingErr error
	for i := 0; i < 3; i++ {
		pingErr = sqlDB.Ping()
		if pingErr == nil {
			return nil
		}
		if i < 2 {
			log.Printf("database ping failed, retrying: %v", pingErr)
			time.Sleep(10 * time.Second)
		}
	}

	if pingErr != nil {
		return fmt.Errorf("database ping failed: %v", pingErr)
	}
	return nil
}

func IsDBAvailable(db *gorm.DB) bool {
	return CheckDBHealth(db) == nil
}
