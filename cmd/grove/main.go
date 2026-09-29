package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/migrate"
	pkgpassword "github.com/zhimma/grove/pkg/password"
)

var configFile string

const (
	rootPasswordEnv         = "GROVE_ROOT_PASSWORD"
	rootPasswordPlaceholder = "{{GROVE_ROOT_PASSWORD_HASH}}"
)

func main() {
	rootCmd := newRootCmd()
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "grove",
		Short:        "框架工具：迁移、数据填充、代码生成与环境信息查看",
		SilenceUsage: true,
		Long: `grove 是当前仓库唯一保留的 CLI 入口。

适合做三类事情：
1. 迁移与 seed
2. 生成 console 后台约定代码
3. 查看当前框架约定与环境信息

make:module 一条命令生成可运行的后台模块：双方言迁移、模型、
分页 CRUD 与测试、接口文档，并注册路由。前端页面暂需手写。`,
	}
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "配置文件路径")

	rootCmd.AddCommand(newAboutCmd())
	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.AddCommand(newMigrateCmd())
	rootCmd.AddCommand(newSeedCmd())
	rootCmd.AddCommand(newRBACCmd())
	rootCmd.AddCommand(newMakeModuleCmd())

	return rootCmd
}

func newAboutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "about",
		Short: "显示当前框架约定与可用能力",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			for _, line := range []string{
				"Grove 基础框架",
				"- 单仓服务：api / console / worker",
				"- CLI 入口：grove",
				"- 默认日志：pkg/logger + zerolog",
				"- 默认校验：make verify",
				"- make:module 仅生成 console 后台后端模板，不生成迁移和前端页面",
			} {
				if err := writeLine(out, line); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "检查当前配置与组件启用状态",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadWithOptions(config.LoadOptions{
				ConfigFile: configFile,
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			lines := []string{
				fmt.Sprintf("应用名称: %s", cfg.App.Name),
				fmt.Sprintf("运行环境: %s", cfg.App.Env),
				fmt.Sprintf("API 端口: %s", cfg.Port),
				fmt.Sprintf("Console 端口: %s", cfg.ConsolePort),
				fmt.Sprintf("默认数据库: %s (%s)", statusText(cfg.Databases.Default.Enabled), cfg.Databases.Default.Driver),
				fmt.Sprintf("Redis: %s", statusText(cfg.Redis.Enabled)),
				fmt.Sprintf("任务队列: %s", statusText(cfg.Job.Enabled)),
				fmt.Sprintf("API 权限控制: %s", statusText(cfg.Casbin.Enforcers["api"].Enabled)),
				fmt.Sprintf("Console 权限控制: %s", statusText(cfg.Casbin.Enforcers["console"].Enabled)),
				fmt.Sprintf("默认存储磁盘: %s", cfg.Storage.Default),
				fmt.Sprintf("文档服务: %s", statusText(cfg.Docs.Enabled)),
			}
			for _, line := range lines {
				if err := writeLine(out, line); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func writeLine(out io.Writer, line string) error {
	_, err := fmt.Fprintln(out, line)
	return err
}

func newMigrateCmd() *cobra.Command {
	migrationPath := "database/migrations"
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "管理 SQL 迁移",
	}
	cmd.PersistentFlags().StringVar(&migrationPath, "path", migrationPath, "迁移文件目录")

	cmd.AddCommand(&cobra.Command{
		Use:   "up",
		Short: "执行所有待执行迁移",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, cleanup, err := openDefaultDB()
			if err != nil {
				return err
			}
			defer cleanup()

			m := migrate.NewManager(db, migrationPath)
			count, err := m.Up()
			if err != nil {
				return err
			}
			fmt.Printf("已执行 %d 个迁移文件\n", count)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "down",
		Short: "回滚最近一次迁移",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, cleanup, err := openDefaultDB()
			if err != nil {
				return err
			}
			defer cleanup()

			m := migrate.NewManager(db, migrationPath)
			name, err := m.Down()
			if err != nil {
				return err
			}
			if name == "" {
				fmt.Println("没有可回滚的迁移")
				return nil
			}
			fmt.Printf("已回滚迁移 %s\n", name)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "查看迁移状态",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, cleanup, err := openDefaultDB()
			if err != nil {
				return err
			}
			defer cleanup()

			m := migrate.NewManager(db, migrationPath)
			statuses, err := m.Status()
			if err != nil {
				return err
			}
			if len(statuses) == 0 {
				fmt.Println("未找到迁移文件")
				return nil
			}
			for _, status := range statuses {
				state := "待执行"
				if status.Applied {
					state = "已执行"
				}
				fmt.Printf("%-10s %s\n", state, status.Name)
			}
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "create [name]",
		Short: "为 postgres 与 mysql 各创建一对同版本的迁移文件",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := migrate.CreateFiles(migrationPath, args[0], nil)
			if err != nil {
				return err
			}
			for _, path := range paths {
				fmt.Println(path)
			}
			return nil
		},
	})

	return cmd
}

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

func newMakeModuleCmd() *cobra.Command {
	var fieldSpec, label string
	cmd := &cobra.Command{
		Use:   "make:module [name]",
		Short: "生成可运行的 console 模块：迁移、模型、CRUD、接口文档与测试",
		Long: `生成一个可以直接运行、并通过 make contracts 的 console 模块。

会生成：
- database/migrations/{postgres,mysql} 同版本迁移
- internal/model 模型
- app/console/internal/service 分页 CRUD 与对应测试
- app/console/internal/handler 请求、响应与路由（含权限名）
- app/console/internal/docs OpenAPI 操作
- 路由注册与 OpenAPI 注册

字段写法：name:type[:required]，逗号分隔。
类型：string、text、int、bool、time；required 仅用于 string、text、time。

示例：
  grove make:module Invoice --label 发票 --fields "title:string:required,amount:int,paid:bool,due_at:time"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := generateConsoleModule(args[0], fieldSpec, label)
			if err != nil {
				return err
			}

			fmt.Println("已生成以下文件：")
			for _, path := range paths {
				fmt.Println(path)
			}
			fmt.Println("已注册路由与 OpenAPI 操作。下一步：make migrate.up，然后按业务补充校验规则与前端页面。")
			return nil
		},
	}
	cmd.Flags().StringVar(&fieldSpec, "fields", "", `字段列表，默认 "`+defaultFields+`"`)
	cmd.Flags().StringVar(&label, "label", "", "模块显示名，用于权限名与接口分组，默认同模块名")
	return cmd
}

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

func statusText(enabled bool) string {
	if enabled {
		return "已启用"
	}
	return "未启用"
}
