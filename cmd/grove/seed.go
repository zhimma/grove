package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/pkg/migrate"
	pkgpassword "github.com/zhimma/grove/pkg/password"
)

const (
	rootPasswordEnv         = "GROVE_ROOT_PASSWORD"
	rootPasswordPlaceholder = "{{GROVE_ROOT_PASSWORD_HASH}}"
)

func newSeedCmd() *cobra.Command {
	seedPath := "database/seeds"
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "执行 SQL seed",
	}
	cmd.PersistentFlags().StringVar(&seedPath, "path", seedPath, "seed 基础目录")

	cmd.AddCommand(&cobra.Command{
		Use:   "bootstrap",
		Short: "执行生产安全的基础 seed",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBootstrapSeeds(cmd, seedPath)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "demo",
		Short: "执行仅供开发和测试使用的演示 seed",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadCLIConfig()
			if err != nil {
				return err
			}
			if strings.EqualFold(strings.TrimSpace(cfg.App.Env), "production") {
				return fmt.Errorf("production 环境禁止执行 demo seed")
			}

			db, cleanup, err := openDefaultDBWithConfig(cfg)
			if err != nil {
				return err
			}
			defer cleanup()

			seedDir, err := resolveSeedDir(seedPath, cfg.Databases.Default.Driver, "demo")
			if err != nil {
				return err
			}
			count, err := migrate.RunSQLDir(db, seedDir)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "已执行 %d 个 demo seed 文件\n", count); err != nil {
				return err
			}
			return nil
		},
	})

	return cmd
}

func runBootstrapSeeds(cmd *cobra.Command, seedBasePath string) error {
	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}

	password, generated, err := resolveRootPassword(cfg)
	if err != nil {
		return err
	}
	if err := config.ValidateInitialRootPassword(password); err != nil {
		return fmt.Errorf("root 初始密码不符合要求: %w", err)
	}
	hash, err := pkgpassword.Hash(password)
	if err != nil {
		return fmt.Errorf("生成 root 密码哈希: %w", err)
	}

	db, cleanup, err := openDefaultDBWithConfig(cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	seedDir, err := resolveSeedDir(seedBasePath, cfg.Databases.Default.Driver, "bootstrap")
	if err != nil {
		if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("rollback bootstrap transaction: %w", rollbackErr))
		}
		return err
	}
	count, err := migrate.RunSQLDirWithReplacements(tx, seedDir, map[string]string{
		rootPasswordPlaceholder: hash,
	})
	if err != nil {
		tx.Rollback()
		return err
	}

	var storedHash string
	if err := tx.Raw(`SELECT password FROM console_admins WHERE id = ?`, "console-admin-root").Scan(&storedHash).Error; err != nil {
		tx.Rollback()
		return err
	}
	if storedHash == "" {
		tx.Rollback()
		return fmt.Errorf("bootstrap seed 未创建或找到 root 管理员")
	}
	createdWithCurrentPassword := pkgpassword.Verify(storedHash, password)

	if err := tx.Commit().Error; err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	if _, err := fmt.Fprintf(out, "已执行 %d 个 bootstrap seed 文件\n", count); err != nil {
		return err
	}
	if generated && createdWithCurrentPassword {
		if err := writeLine(out, "Root 一次性初始密码（仅显示本次，请立即保存并登录修改）："); err != nil {
			return err
		}
		if err := writeLine(out, password); err != nil {
			return err
		}
	}
	return nil
}

func resolveSeedDir(baseDir, driver, kind string) (string, error) {
	baseDir = strings.TrimSpace(baseDir)
	driver = strings.ToLower(strings.TrimSpace(driver))
	if driver == "postgresql" {
		driver = "postgres"
	}
	kind = strings.TrimSpace(kind)
	if baseDir == "" || kind == "" {
		return "", fmt.Errorf("seed 基础目录和类型不能为空")
	}
	candidates := []string{
		filepath.Join(baseDir, driver, kind),
		filepath.Join(baseDir, kind),
	}
	for _, candidate := range candidates {
		entries, err := os.ReadDir(filepath.Clean(candidate))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
				return candidate, nil
			}
		}
	}
	return "", fmt.Errorf("database %s seed directory not found under %s", driver, baseDir)
}

func resolveRootPassword(cfg *config.Config) (string, bool, error) {
	if cfg != nil {
		if password := strings.TrimSpace(cfg.Security.InitialRootPassword); password != "" {
			return password, false, nil
		}
	}
	if password := strings.TrimSpace(os.Getenv(rootPasswordEnv)); password != "" {
		return password, false, nil
	}

	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", false, fmt.Errorf("生成 root 初始密码: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), true, nil
}
