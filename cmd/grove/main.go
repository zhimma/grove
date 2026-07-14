package main

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/config"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/migrate"
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

注意：
- make:module 只生成 console 后台的 model/service/handler，并自动注册后端路由
- 不会自动生成数据库迁移
- 不会自动生成前端页面或菜单`,
	}
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "配置文件路径")

	rootCmd.AddCommand(newAboutCmd())
	rootCmd.AddCommand(newDoctorCmd())
	rootCmd.AddCommand(newMigrateCmd())
	rootCmd.AddCommand(newSeedCmd())
	rootCmd.AddCommand(newMakeModelCmd())
	rootCmd.AddCommand(newMakeServiceCmd())
	rootCmd.AddCommand(newMakeHandlerCmd())
	rootCmd.AddCommand(newMakeModuleCmd())

	return rootCmd
}

func newAboutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "about",
		Short: "显示当前框架约定与可用能力",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "Grove 基础框架")
			fmt.Fprintln(out, "- 单仓服务：api / console / worker")
			fmt.Fprintln(out, "- CLI 入口：grove")
			fmt.Fprintln(out, "- 默认日志：pkg/logger + zerolog")
			fmt.Fprintln(out, "- 默认校验：make verify")
			fmt.Fprintln(out, "- make:module 仅生成 console 后台后端模板，不生成迁移和前端页面")
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
				Service:    "grove",
				ConfigFile: configFile,
			})
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "应用名称: %s\n", cfg.App.Name)
			fmt.Fprintf(out, "运行环境: %s\n", cfg.App.Env)
			fmt.Fprintf(out, "API 端口: %s\n", cfg.Port)
			fmt.Fprintf(out, "Console 端口: %s\n", cfg.ConsolePort)
			fmt.Fprintf(out, "默认数据库: %s\n", statusText(cfg.Databases.Default.Enabled))
			fmt.Fprintf(out, "Redis: %s\n", statusText(cfg.Redis.Enabled))
			fmt.Fprintf(out, "任务队列: %s\n", statusText(cfg.Job.Enabled))
			fmt.Fprintf(out, "API 权限控制: %s\n", statusText(cfg.Casbin.Enforcers["api"].Enabled))
			fmt.Fprintf(out, "Console 权限控制: %s\n", statusText(cfg.Casbin.Enforcers["console"].Enabled))
			fmt.Fprintf(out, "默认存储磁盘: %s\n", cfg.Storage.Default)
			fmt.Fprintf(out, "文档服务: %s\n", statusText(cfg.Docs.Enabled))
			return nil
		},
	}
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
		Short: "创建新的迁移文件对",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			upPath, downPath, err := migrate.CreateFiles(migrationPath, args[0])
			if err != nil {
				return err
			}
			fmt.Println(upPath)
			fmt.Println(downPath)
			return nil
		},
	})

	return cmd
}

func newSeedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "执行 SQL seed",
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "bootstrap",
		Short: "执行生产安全的基础 seed",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBootstrapSeeds(cmd)
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

			count, err := migrate.RunSQLDir(db, "database/seeds/demo")
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "已执行 %d 个 demo seed 文件\n", count)
			return nil
		},
	})

	return cmd
}

func runBootstrapSeeds(cmd *cobra.Command) error {
	cfg, err := loadCLIConfig()
	if err != nil {
		return err
	}

	password, generated, err := resolveRootPassword()
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
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
	count, err := migrate.RunSQLDirWithReplacements(tx, "database/seeds/bootstrap", map[string]string{
		rootPasswordPlaceholder: string(hash),
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
	createdWithCurrentPassword := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(password)) == nil

	if err := tx.Commit().Error; err != nil {
		return err
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "已执行 %d 个 bootstrap seed 文件\n", count)
	if generated && createdWithCurrentPassword {
		fmt.Fprintln(out, "Root 一次性初始密码（仅显示本次，请立即保存并登录修改）：")
		fmt.Fprintln(out, password)
	}
	return nil
}

func resolveRootPassword() (string, bool, error) {
	if password := strings.TrimSpace(os.Getenv(rootPasswordEnv)); password != "" {
		return password, false, nil
	}

	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", false, fmt.Errorf("生成 root 初始密码: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), true, nil
}

func newMakeModelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "make:model [name]",
		Short: "生成共享 model 模板",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := toPascal(args[0])
			path := filepath.Join("internal/model", toSnake(args[0])+".go")
			content := fmt.Sprintf(`package model

type %s struct {
	Base
}

func (%s) TableName() string {
	return "%s"
}
`, name, name, toSnakePlural(args[0]))
			return writeFile(path, content)
		},
	}
}

func newMakeServiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "make:service [name]",
		Short: "生成 console service 模板",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := toPascal(args[0])
			snake := toSnake(args[0])
			path := filepath.Join("app/console/internal/service", snake+".go")
			return writeFile(path, consoleServiceTemplate(name, snake))
		},
	}
}

func newMakeHandlerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "make:handler [name]",
		Short: "生成 console handler 模板",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := toPascal(args[0])
			snake := toSnake(args[0])
			path := filepath.Join("app/console/internal/handler", snake+".go")
			return writeFile(path, consoleHandlerTemplate(name, snake))
		},
	}
}

func newMakeModuleCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "make:module [name]",
		Short: "生成 console model、service、handler，并自动注册后端路由",
		Long: `生成当前 console-first 约定下的最小后台模块模板。

会生成：
- internal/model
- app/console/service
- app/console/handler
- app/console/router 路由注册

不会生成：
- database/migrations
- web/admin-vben 前端页面
- 菜单或权限数据`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := toPascal(args[0])
			snake := toSnake(args[0])

			modelPath := filepath.Join("internal/model", snake+".go")
			if err := writeFile(modelPath, modelTemplate(name, snake)); err != nil {
				return err
			}

			servicePath := filepath.Join("app/console/internal/service", snake+".go")
			if err := writeFile(servicePath, consoleServiceTemplate(name, snake)); err != nil {
				return err
			}

			handlerPath := filepath.Join("app/console/internal/handler", snake+".go")
			if err := writeFile(handlerPath, consoleHandlerTemplate(name, snake)); err != nil {
				return err
			}

			line := fmt.Sprintf("\thandler.Register%sRoutes(protected, r.p)\n", name)
			if err := insertRouteRegistration("app/console/internal/router/router.go", line); err != nil {
				return err
			}

			fmt.Println("已生成以下文件：")
			fmt.Println(modelPath)
			fmt.Println(servicePath)
			fmt.Println(handlerPath)
			fmt.Println("已自动写入 console 路由注册，请继续补充迁移、前端页面和业务逻辑。")
			return nil
		},
	}
}

func modelTemplate(name, snake string) string {
	return fmt.Sprintf(`package model

type %s struct {
	Base
}

func (%s) TableName() string {
	return "%s"
}
`, name, name, toSnakePlural(snake))
}

func consoleServiceTemplate(name, snake string) string {
	return fmt.Sprintf(`package service

import (
	"context"

	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
)

type %sService struct {
	dbRepo database.Repo
}

type %sListInput struct{}

type %sListOutput struct {
	Message string `+"`json:\"message\"`"+`
}

func New%sService(dbRepo database.Repo) *%sService {
	return &%sService{dbRepo: dbRepo}
}

func (s *%sService) List(_ context.Context, _ %sListInput) (*%sListOutput, error) {
	if s.dbRepo == nil || s.dbRepo.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}
	return &%sListOutput{Message: "%s 模块已就绪"}, nil
}
`, name, name, name, name, name, name, name, name, name, name, name)
}

func consoleHandlerTemplate(name, snake string) string {
	routePath := "/" + toKebabPlural(snake)
	return fmt.Sprintf(`package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/service"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
)

type %sHandler struct {
	%sSvc *consoleservice.%sService
}

func Register%sRoutes(protected *gin.RouterGroup, p *provider.Provider) {
	h := &%sHandler{
		%sSvc: consoleservice.New%sService(p.DB),
	}

	group := route.Wrap(protected.Group("%s"))
	group.GET("", h.List).Name("%s.列表")
}

func (h *%sHandler) List(c *gin.Context) {
	out, err := h.%sSvc.List(c.Request.Context(), consoleservice.%sListInput{})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, out)
}
`, name, snake, name, name, name, snake, name, routePath, name, name, snake, name)
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
		Service:    "grove",
		ConfigFile: configFile,
	})
}

func openDefaultDBWithConfig(cfg *config.Config) (*gorm.DB, func(), error) {
	repo, err := database.NewRepo(database.Config{
		Enabled:         cfg.Databases.Default.Enabled,
		Driver:          cfg.Databases.Default.Driver,
		Host:            cfg.Databases.Default.Host,
		Port:            cfg.Databases.Default.Port,
		User:            cfg.Databases.Default.User,
		Password:        cfg.Databases.Default.Password,
		DBName:          cfg.Databases.Default.DBName,
		SSLMode:         cfg.Databases.Default.SSLMode,
		MaxConnections:  cfg.Databases.Default.MaxConnections,
		MaxIdleConns:    cfg.Databases.Default.MaxIdleConns,
		ConnMaxLifetime: cfg.Databases.Default.ConnMaxLifetime,
	}, nil)
	if err != nil {
		return nil, nil, err
	}
	if repo.Default() == nil {
		return nil, nil, fmt.Errorf("默认数据库未启用")
	}

	return repo.Default(), func() {
		_ = repo.Close()
	}, nil
}

func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("文件已存在: %s", path)
	}
	return os.WriteFile(filepath.Clean(path), []byte(content), 0o600)
}

func insertRouteRegistration(path, line string) error {
	const marker = "\t// grove:register-routes\n"

	body, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}
	if bytes.Contains(body, []byte(line)) {
		return nil
	}
	if !bytes.Contains(body, []byte(marker)) {
		return fmt.Errorf("未在 %s 中找到 grove 路由标记", path)
	}

	updated := strings.Replace(string(body), marker, line+marker, 1)
	return os.WriteFile(filepath.Clean(path), []byte(updated), 0o600)
}

func toSnake(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	var out []rune
	for i, r := range input {
		if unicode.IsUpper(r) {
			if i > 0 && out[len(out)-1] != '_' {
				out = append(out, '_')
			}
			out = append(out, unicode.ToLower(r))
			continue
		}
		if r == '-' || r == ' ' {
			if len(out) > 0 && out[len(out)-1] != '_' {
				out = append(out, '_')
			}
			continue
		}
		out = append(out, unicode.ToLower(r))
	}
	return strings.Trim(string(out), "_")
}

func toKebabPlural(input string) string {
	base := strings.ReplaceAll(toSnake(input), "_", "-")
	return pluralize(base)
}

func toSnakePlural(input string) string {
	return pluralize(toSnake(input))
}

func pluralize(base string) string {
	if base == "" {
		return ""
	}
	if strings.HasSuffix(base, "y") && len(base) > 1 {
		prev := base[len(base)-2]
		if !strings.ContainsRune("aeiou", rune(prev)) {
			return strings.TrimSuffix(base, "y") + "ies"
		}
	}
	if strings.HasSuffix(base, "s") {
		return base
	}
	return base + "s"
}

func toPascal(input string) string {
	parts := strings.Split(toSnake(input), "_")
	var out strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(strings.ToLower(part))
		runes[0] = unicode.ToUpper(runes[0])
		out.WriteString(string(runes))
	}
	if out.Len() == 0 {
		return ""
	}
	return out.String()
}

func statusText(enabled bool) string {
	if enabled {
		return "已启用"
	}
	return "未启用"
}
