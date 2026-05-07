package transaction

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type txTestUser struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"size:64"`
}

func TestNewManagerRejectsNilDB(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when creating transaction manager with nil db")
		}
	}()

	_ = NewManager(nil)
}

func TestExecuteReusesExistingTransaction(t *testing.T) {
	db := openTransactionTestDB(t)
	manager := NewManager(db)

	var nestedCount int
	err := manager.Execute(context.Background(), func(ctx context.Context) error {
		return manager.Execute(ctx, func(inner context.Context) error {
			nestedCount++
			return GetDB(inner, db).Create(&txTestUser{Name: "nested"}).Error
		})
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}

	if nestedCount != 1 {
		t.Fatalf("expected nested callback once, got %d", nestedCount)
	}

	var count int64
	if err := db.Model(&txTestUser{}).Count(&count).Error; err != nil {
		t.Fatalf("count users failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 created row, got %d", count)
	}
}

func openTransactionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/transaction.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&txTestUser{}); err != nil {
		t.Fatalf("auto migrate transaction test user: %v", err)
	}
	return db
}
