// Package transaction propagates a *gorm.DB through context so a service can
// join a transaction its caller opened. Services open transactions with gorm's
// own db.Transaction; this package only carries the handle.
package transaction

import (
	"context"

	"gorm.io/gorm"
)

type dbKey struct{}

func WithDB(ctx context.Context, db *gorm.DB) context.Context {
	return context.WithValue(ctx, dbKey{}, db)
}

// GetDB returns the transaction the caller put in ctx, or defaultDB when the
// call is not inside one. Both are bound to ctx before returning.
func GetDB(ctx context.Context, defaultDB *gorm.DB) *gorm.DB {
	if db := getDBFromContext(ctx); db != nil {
		return db.WithContext(ctx)
	}
	if defaultDB == nil {
		return nil
	}
	return defaultDB.WithContext(ctx)
}

func getDBFromContext(ctx context.Context) *gorm.DB {
	db, _ := ctx.Value(dbKey{}).(*gorm.DB)
	return db
}
