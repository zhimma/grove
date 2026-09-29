package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
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
	Charset         string
	ParseTime       bool
	Loc             string
	TLS             bool
	MaxConnections  int
	MaxIdleConns    int
	ConnMaxLifetime int
	ConnectTimeout  int
}

// Connections holds the default database and any named resources. Its
// methods are safe on a nil receiver, so an unconfigured database reads as
// "no connection" rather than a panic.
type Connections struct {
	defaultDB *gorm.DB
	resources map[string]*gorm.DB
}

func NewConnections(defaultConfig Config, resourceConfigs map[string]Config) (*Connections, error) {
	dbs := &Connections{
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

func NewConnectionsFromDBs(defaultDB *gorm.DB, resources map[string]*gorm.DB) *Connections {
	dbs := &Connections{
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

func (c *Connections) Default() *gorm.DB {
	if c == nil {
		return nil
	}
	return c.defaultDB
}

func (c *Connections) Get(name string) (*gorm.DB, error) {
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

func (c *Connections) Has(name string) bool {
	if c == nil {
		return false
	}
	_, err := c.Get(name)
	return err == nil
}

func (c *Connections) Names() []string {
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

func (c *Connections) Close() error {
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

	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if driver == "" {
		driver = "postgres"
	}

	var dialector gorm.Dialector
	switch driver {
	case "postgres", "postgresql":
		dialector = postgres.Open(buildPostgresDSN(cfg))
	case "mysql":
		dialector = mysql.Open(buildMySQLDSN(cfg))
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger:               logger.Default.LogMode(logger.Warn),
		DisableAutomaticPing: true,
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	connectTimeout := time.Duration(cfg.ConnectTimeout) * time.Second
	if connectTimeout <= 0 {
		connectTimeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	err = sqlDB.PingContext(ctx)
	cancel()
	if err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping %s database: %w", driver, err)
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

func buildPostgresDSN(cfg Config) string {
	dsn := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(cfg.Host, cfg.Port),
		Path:   "/",
	}
	dsn.User = url.UserPassword(cfg.User, cfg.Password)
	query := url.Values{}
	query.Set("dbname", cfg.DBName)
	if strings.TrimSpace(cfg.SSLMode) != "" {
		query.Set("sslmode", cfg.SSLMode)
	}
	dsn.RawQuery = query.Encode()
	return dsn.String()
}

func buildMySQLDSN(cfg Config) string {
	charset := strings.TrimSpace(cfg.Charset)
	if charset == "" {
		charset = "utf8mb4"
	}
	loc := strings.TrimSpace(cfg.Loc)
	if loc == "" {
		loc = "Local"
	}
	params := map[string]string{
		"charset":         charset,
		"parseTime":       fmt.Sprintf("%t", cfg.ParseTime),
		"loc":             loc,
		"multiStatements": "true",
	}
	if cfg.TLS {
		params["tls"] = "true"
	}
	mysqlCfg := mysqlDriver.Config{
		User:                 cfg.User,
		Passwd:               cfg.Password,
		Net:                  "tcp",
		Addr:                 net.JoinHostPort(cfg.Host, cfg.Port),
		DBName:               cfg.DBName,
		AllowNativePasswords: true,
		Params:               params,
	}
	// FormatDSN performs the required escaping for credentials and query values.
	return mysqlCfg.FormatDSN()
}
