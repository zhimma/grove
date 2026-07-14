package database

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Config struct {
	Enabled         bool
	Driver          string
	Host            string
	Port            string
	User            string
	Password        string
	DBName          string
	SSLMode         string
	MaxConnections  int
	MaxIdleConns    int
	ConnMaxLifetime int
}

type Connections interface {
	Default() *gorm.DB
	Get(name string) (*gorm.DB, error)
	Has(name string) bool
	Names() []string
	Close() error
}

type connections struct {
	defaultDB *gorm.DB
	resources map[string]*gorm.DB
}

func NewConnections(defaultConfig Config, resourceConfigs map[string]Config) (Connections, error) {
	dbs := &connections{
		resources: map[string]*gorm.DB{},
	}

	defaultDB, err := open(defaultConfig)
	if err != nil {
		return nil, err
	}
	dbs.defaultDB = defaultDB
	if defaultDB != nil {
		dbs.resources["default"] = defaultDB
	}

	for name, cfg := range resourceConfigs {
		resourceName := strings.TrimSpace(strings.ToLower(name))
		if resourceName == "" || resourceName == "default" {
			continue
		}
		db, err := open(cfg)
		if err != nil {
			_ = dbs.Close()
			return nil, fmt.Errorf("open database resource %q: %w", resourceName, err)
		}
		if db != nil {
			dbs.resources[resourceName] = db
		}
	}

	return dbs, nil
}

func NewConnectionsFromDBs(defaultDB *gorm.DB, resources map[string]*gorm.DB) Connections {
	dbs := &connections{
		defaultDB: defaultDB,
		resources: map[string]*gorm.DB{},
	}
	if defaultDB != nil {
		dbs.resources["default"] = defaultDB
	}
	for name, db := range resources {
		resourceName := strings.TrimSpace(strings.ToLower(name))
		if resourceName == "" || db == nil {
			continue
		}
		if resourceName == "default" {
			dbs.defaultDB = db
		}
		dbs.resources[resourceName] = db
	}
	return dbs
}

func (c *connections) Default() *gorm.DB {
	if c == nil {
		return nil
	}
	return c.defaultDB
}

func (c *connections) Get(name string) (*gorm.DB, error) {
	if c == nil {
		return nil, fmt.Errorf("database connections are nil")
	}
	resourceName := strings.TrimSpace(strings.ToLower(name))
	if resourceName == "" || resourceName == "default" {
		if c.defaultDB == nil {
			return nil, fmt.Errorf("default database is not configured")
		}
		return c.defaultDB, nil
	}
	db, ok := c.resources[resourceName]
	if !ok || db == nil {
		return nil, fmt.Errorf("database resource %q is not configured", resourceName)
	}
	return db, nil
}

func (c *connections) Has(name string) bool {
	if c == nil {
		return false
	}
	_, err := c.Get(name)
	return err == nil
}

func (c *connections) Names() []string {
	if c == nil {
		return nil
	}
	names := make([]string, 0, len(c.resources))
	for name, db := range c.resources {
		if db == nil {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (c *connections) Close() error {
	if c == nil {
		return nil
	}

	closed := map[*sql.DB]struct{}{}
	for _, db := range c.resources {
		if db == nil {
			continue
		}
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		if _, exists := closed[sqlDB]; exists {
			continue
		}
		if err := sqlDB.Close(); err != nil {
			return err
		}
		closed[sqlDB] = struct{}{}
	}
	return nil
}

func open(cfg Config) (*gorm.DB, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	if cfg.Driver == "" {
		cfg.Driver = "postgres"
	}
	if cfg.Driver != "postgres" {
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.DBName,
		cfg.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	if cfg.MaxConnections > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxConnections)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	}

	return db, nil
}
