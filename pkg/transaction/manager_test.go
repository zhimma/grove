package transaction

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type txTestUser struct {
	ID   uint   `gorm:"primaryKey"`
	Name string `gorm:"size:64"`
}

func TestGetDBHandlesNilDefault(t *testing.T) {
	if db := GetDB(context.Background(), nil); db != nil {
		t.Fatal("expected nil when no transaction, override, or default database exists")
	}

	defaultDB := openTransactionTestDB(t)
	overrideDB := openTransactionTestDB(t)
	ctx := WithDB(context.Background(), overrideDB)

	if db := GetDB(ctx, defaultDB); db == nil || db.Statement.ConnPool != overrideDB.Statement.ConnPool {
		t.Fatal("expected override database from context")
	}
}

// This is how services actually use the package: the caller opens a gorm
// transaction and hands the handle down through context, and the callee joins
// it via GetDB. If GetDB returned the default connection instead, the callee's
// writes would land outside the transaction and survive a rollback.
func TestGetDBJoinsTheCallersTransaction(t *testing.T) {
	db := openTransactionTestDB(t)
	wantErr := errors.New("caller aborted")

	err := db.Transaction(func(tx *gorm.DB) error {
		ctx := WithDB(context.Background(), tx)
		if err := GetDB(ctx, db).Create(&txTestUser{Name: "joined"}).Error; err != nil {
			return err
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("transaction error = %v, want %v", err, wantErr)
	}

	var count int64
	if err := db.Model(&txTestUser{}).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 0 {
		t.Fatalf("callee wrote outside the caller's transaction: %d rows survived the rollback", count)
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
