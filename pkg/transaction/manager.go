package transaction

import (
	"context"

	"gorm.io/gorm"
)

type Manager interface {
	Execute(ctx context.Context, fn func(ctx context.Context) error) error
	ExecuteWithResult(ctx context.Context, fn func(ctx context.Context) (any, error)) (any, error)
}

type GormManager struct {
	db *gorm.DB
}

func NewManager(db *gorm.DB) *GormManager {
	if db == nil {
		panic("transaction: gorm db is nil")
	}
	return &GormManager{db: db}
}

func (m *GormManager) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	if tx := FromContext(ctx); tx != nil {
		return fn(ctx)
	}

	db := m.db
	if override := getDBFromContext(ctx); override != nil {
		db = override
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

func (m *GormManager) ExecuteWithResult(ctx context.Context, fn func(ctx context.Context) (any, error)) (any, error) {
	if tx := FromContext(ctx); tx != nil {
		return fn(ctx)
	}

	db := m.db
	if override := getDBFromContext(ctx); override != nil {
		db = override
	}

	var result any
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var execErr error
		result, execErr = fn(context.WithValue(ctx, txKey{}, tx))
		return execErr
	})
	return result, err
}

type txKey struct{}
type dbKey struct{}

func FromContext(ctx context.Context) *gorm.DB {
	tx, _ := ctx.Value(txKey{}).(*gorm.DB)
	return tx
}

func WithDB(ctx context.Context, db *gorm.DB) context.Context {
	return context.WithValue(ctx, dbKey{}, db)
}

func GetDB(ctx context.Context, defaultDB *gorm.DB) *gorm.DB {
	if tx := FromContext(ctx); tx != nil {
		return tx
	}
	if db := getDBFromContext(ctx); db != nil {
		return db.WithContext(ctx)
	}
	return defaultDB.WithContext(ctx)
}

func getDBFromContext(ctx context.Context) *gorm.DB {
	db, _ := ctx.Value(dbKey{}).(*gorm.DB)
	return db
}
