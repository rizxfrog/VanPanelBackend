package di

import (
	"testing"

	"github.com/spf13/viper"
)

func TestResolveDatabaseConfigUsesUnifiedDatabaseSettings(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	viper.Set("database.driver", "postgres")
	viper.Set("database.dsn", "host=postgres port=5432 user=postgres password=postgres dbname=cloudops sslmode=disable")

	cfg, err := resolveDatabaseConfig()
	if err != nil {
		t.Fatalf("resolveDatabaseConfig returned error: %v", err)
	}

	if cfg.Driver != "postgres" {
		t.Fatalf("expected driver postgres, got %s", cfg.Driver)
	}
	if cfg.DSN != "host=postgres port=5432 user=postgres password=postgres dbname=cloudops sslmode=disable" {
		t.Fatalf("unexpected dsn: %s", cfg.DSN)
	}
}

func TestResolveDatabaseConfigFallsBackToLegacyMySQLSettings(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	viper.Set("mysql.addr", "root:root@tcp(localhost:3306)/cloudops?charset=utf8mb4&parseTime=True&loc=Local")

	cfg, err := resolveDatabaseConfig()
	if err != nil {
		t.Fatalf("resolveDatabaseConfig returned error: %v", err)
	}

	if cfg.Driver != "mysql" {
		t.Fatalf("expected legacy mysql driver, got %s", cfg.Driver)
	}
	if cfg.DSN != "root:root@tcp(localhost:3306)/cloudops?charset=utf8mb4&parseTime=True&loc=Local" {
		t.Fatalf("unexpected dsn: %s", cfg.DSN)
	}
}

func TestResolveDatabaseConfigRejectsUnsupportedDriver(t *testing.T) {
	t.Cleanup(viper.Reset)
	viper.Reset()
	viper.Set("database.driver", "sqlite")
	viper.Set("database.dsn", "file:test.db")

	_, err := resolveDatabaseConfig()
	if err == nil {
		t.Fatal("expected unsupported driver error")
	}
}
