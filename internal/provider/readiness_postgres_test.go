//go:build integration

package provider

import (
	"context"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/internal/readiness"
	"github.com/zhimma/grove/pkg/database"
)

func TestPostgresReadinessCheck(t *testing.T) {
	dsn := os.Getenv("GROVE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("GROVE_TEST_POSTGRES_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get PostgreSQL pool: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	p := &Provider{
		Config: &config.Config{
			Databases: config.DatabasesConfig{
				Default: config.DatabaseConfig{Enabled: true},
			},
		},
		DB: database.NewConnectionsFromDBs(db, nil),
	}
	report := readiness.New(p.ReadinessChecks(), time.Second).Run(context.Background())
	if !report.Ready {
		t.Fatalf("expected PostgreSQL readiness success: %#v errors=%v", report, report.Errors())
	}
}
