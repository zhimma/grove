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

type Repo interface {
	Default() *gorm.DB
	Get(name string) (*gorm.DB, error)
	Has(name string) bool
	Names() []string
	Close() error
}

type repo struct {
	defaultDB *gorm.DB
	resources map[string]*gorm.DB
}

func NewRepo(defaultConfig Config, resourceConfigs map[string]Config) (Repo, error) {
	r := &repo{
		resources: map[string]*gorm.DB{},
	}

	defaultDB, err := open(defaultConfig)
	if err != nil {
		return nil, err
	}
	r.defaultDB = defaultDB
	if defaultDB != nil {
		r.resources["default"] = defaultDB
	}

	for name, cfg := range resourceConfigs {
		resourceName := strings.TrimSpace(strings.ToLower(name))
		if resourceName == "" || resourceName == "default" {
			continue
		}
		db, err := open(cfg)
		if err != nil {
			_ = r.Close()
			return nil, fmt.Errorf("open database resource %q: %w", resourceName, err)
		}
		if db != nil {
			r.resources[resourceName] = db
		}
	}

	return r, nil
}

func NewRepoWithConnections(defaultDB *gorm.DB, resources map[string]*gorm.DB) Repo {
	r := &repo{
		defaultDB: defaultDB,
		resources: map[string]*gorm.DB{},
	}
	if defaultDB != nil {
		r.resources["default"] = defaultDB
	}
	for name, db := range resources {
		resourceName := strings.TrimSpace(strings.ToLower(name))
		if resourceName == "" || db == nil {
			continue
		}
		if resourceName == "default" {
			r.defaultDB = db
		}
		r.resources[resourceName] = db
	}
	return r
}

func (r *repo) Default() *gorm.DB {
	if r == nil {
		return nil
	}
	return r.defaultDB
}

func (r *repo) Get(name string) (*gorm.DB, error) {
	if r == nil {
		return nil, fmt.Errorf("database repo is nil")
	}
	resourceName := strings.TrimSpace(strings.ToLower(name))
	if resourceName == "" || resourceName == "default" {
		if r.defaultDB == nil {
			return nil, fmt.Errorf("default database is not configured")
		}
		return r.defaultDB, nil
	}
	db, ok := r.resources[resourceName]
	if !ok || db == nil {
		return nil, fmt.Errorf("database resource %q is not configured", resourceName)
	}
	return db, nil
}

func (r *repo) Has(name string) bool {
	if r == nil {
		return false
	}
	_, err := r.Get(name)
	return err == nil
}

func (r *repo) Names() []string {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.resources))
	for name, db := range r.resources {
		if db == nil {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *repo) Close() error {
	if r == nil {
		return nil
	}

	closed := map[*sql.DB]struct{}{}
	for _, db := range r.resources {
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
