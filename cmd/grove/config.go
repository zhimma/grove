package main

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/pkg/database"
)

func openDefaultDB() (*gorm.DB, func(), error) {
	cfg, err := loadCLIConfig()
	if err != nil {
		return nil, nil, err
	}
	return openDefaultDBWithConfig(cfg)
}

func loadCLIConfig() (*config.Config, error) {
	return config.LoadWithOptions(config.LoadOptions{
		ConfigFile: configFile,
	})
}

func openDefaultDBWithConfig(cfg *config.Config) (*gorm.DB, func(), error) {
	dbs, err := database.NewConnections(database.Config{
		Enabled:         cfg.Databases.Default.Enabled,
		Driver:          cfg.Databases.Default.Driver,
		Host:            cfg.Databases.Default.Host,
		Port:            cfg.Databases.Default.Port,
		User:            cfg.Databases.Default.User,
		Password:        cfg.Databases.Default.Password,
		DBName:          cfg.Databases.Default.DBName,
		SSLMode:         cfg.Databases.Default.SSLMode,
		Charset:         cfg.Databases.Default.Charset,
		ParseTime:       cfg.Databases.Default.ParseTime,
		Loc:             cfg.Databases.Default.Loc,
		TLS:             cfg.Databases.Default.TLS,
		MaxConnections:  cfg.Databases.Default.MaxConnections,
		MaxIdleConns:    cfg.Databases.Default.MaxIdleConns,
		ConnMaxLifetime: cfg.Databases.Default.ConnMaxLifetime,
		ConnectTimeout:  cfg.Databases.Default.ConnectTimeout,
	}, nil)
	if err != nil {
		return nil, nil, err
	}
	if dbs.Default() == nil {
		return nil, nil, fmt.Errorf("默认数据库未启用")
	}

	return dbs.Default(), func() {
		_ = dbs.Close()
	}, nil
}
