package observability

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

const gormSpanStateKey = "grove:observability:span"

type gormSpanState struct {
	span trace.Span
}

type dbPoolInstruments struct {
	open      metric.Int64ObservableGauge
	inUse     metric.Int64ObservableGauge
	idle      metric.Int64ObservableGauge
	waitCount metric.Int64ObservableGauge
}

func (i *dbPoolInstruments) init(meter metric.Meter) error {
	var err error
	i.open, err = meter.Int64ObservableGauge("db.pool.open", metric.WithDescription("Open database connections"))
	if err != nil {
		return err
	}
	i.inUse, err = meter.Int64ObservableGauge("db.pool.in_use", metric.WithDescription("Database connections currently in use"))
	if err != nil {
		return err
	}
	i.idle, err = meter.Int64ObservableGauge("db.pool.idle", metric.WithDescription("Idle database connections"))
	if err != nil {
		return err
	}
	i.waitCount, err = meter.Int64ObservableGauge("db.pool.wait_count", metric.WithDescription("Database connection wait count"))
	return err
}

func (r *Runtime) ObserveDBPool(name string, db *sql.DB) error {
	if r == nil || db == nil || r.meterProvider == nil {
		return nil
	}
	name = strings.TrimSpace(name)
	registration, err := r.meterProvider.Meter(meterName).RegisterCallback(
		func(_ context.Context, observer metric.Observer) error {
			stats := db.Stats()
			attrs := metric.WithAttributes(attribute.String("database", name))
			observer.ObserveInt64(r.dbPool.open, int64(stats.OpenConnections), attrs)
			observer.ObserveInt64(r.dbPool.inUse, int64(stats.InUse), attrs)
			observer.ObserveInt64(r.dbPool.idle, int64(stats.Idle), attrs)
			observer.ObserveInt64(r.dbPool.waitCount, stats.WaitCount, attrs)
			return nil
		},
		r.dbPool.open,
		r.dbPool.inUse,
		r.dbPool.idle,
		r.dbPool.waitCount,
	)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.registrations = append(r.registrations, registration)
	r.mu.Unlock()
	return nil
}

func (r *Runtime) InstrumentGORM(name string, db *gorm.DB) error {
	if r == nil || db == nil {
		return nil
	}
	name = strings.TrimSpace(name)
	callbacks := []struct {
		operation string
		before    func(string, func(*gorm.DB)) error
		after     func(string, func(*gorm.DB)) error
	}{
		{operation: "create", before: db.Callback().Create().Before("gorm:create").Register, after: db.Callback().Create().After("gorm:create").Register},
		{operation: "query", before: db.Callback().Query().Before("gorm:query").Register, after: db.Callback().Query().After("gorm:query").Register},
		{operation: "update", before: db.Callback().Update().Before("gorm:update").Register, after: db.Callback().Update().After("gorm:update").Register},
		{operation: "delete", before: db.Callback().Delete().Before("gorm:delete").Register, after: db.Callback().Delete().After("gorm:delete").Register},
		{operation: "row", before: db.Callback().Row().Before("gorm:row").Register, after: db.Callback().Row().After("gorm:row").Register},
		{operation: "raw", before: db.Callback().Raw().Before("gorm:raw").Register, after: db.Callback().Raw().After("gorm:raw").Register},
	}
	for _, callback := range callbacks {
		operation := callback.operation
		if err := callback.before("grove:otel:before:"+operation, r.gormBefore(name, operation)); err != nil {
			return fmt.Errorf("register gorm %s before callback: %w", operation, err)
		}
		if err := callback.after("grove:otel:after:"+operation, r.gormAfter(operation)); err != nil {
			return fmt.Errorf("register gorm %s after callback: %w", operation, err)
		}
	}
	return nil
}

func (r *Runtime) gormBefore(databaseName, operation string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		if db == nil || db.Statement == nil {
			return
		}
		ctx := db.Statement.Context
		if ctx == nil {
			ctx = context.Background()
		}
		ctx, span := r.Tracer("github.com/zhimma/grove/gorm").Start(
			ctx,
			"gorm."+operation,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				attribute.String("db.system", databaseSystem(db)),
				attribute.String("db.namespace", databaseName),
				attribute.String("db.operation.name", operation),
			),
		)
		db.Statement.Context = ctx
		db.Statement.Settings.Store(gormSpanStateKey+":"+operation, gormSpanState{span: span})
	}
}

func databaseSystem(db *gorm.DB) string {
	if db == nil || db.Dialector == nil {
		return "unknown"
	}
	switch strings.ToLower(strings.TrimSpace(db.Name())) {
	case "postgres", "postgresql":
		return "postgresql"
	case "mysql":
		return "mysql"
	case "sqlite":
		return "sqlite"
	default:
		return strings.ToLower(strings.TrimSpace(db.Name()))
	}
}

func (r *Runtime) gormAfter(operation string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		if db == nil || db.Statement == nil {
			return
		}
		value, ok := db.Statement.Settings.Load(gormSpanStateKey + ":" + operation)
		if !ok {
			return
		}
		state, ok := value.(gormSpanState)
		if !ok || state.span == nil {
			return
		}
		state.span.SetAttributes(
			attribute.String("db.collection.name", db.Statement.Table),
			attribute.Int64("db.response.returned_rows", db.RowsAffected),
		)
		if db.Error != nil && !errors.Is(db.Error, gorm.ErrRecordNotFound) {
			state.span.RecordError(db.Error)
			state.span.SetStatus(codes.Error, "database operation failed")
		}
		state.span.End(trace.WithTimestamp(time.Now()))
	}
}
