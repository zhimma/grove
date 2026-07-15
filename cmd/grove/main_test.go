package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhimma/grove/internal/config"
)

func TestAboutCommandPrintsFrameworkSummary(t *testing.T) {
	cmd := newAboutCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("about command failed: %v", err)
	}

	content := out.String()
	assertContains(t, content, "Grove 基础框架")
	assertContains(t, content, "api / console / worker")
	assertContains(t, content, "make verify")
}

func TestRootCommandIncludesGroveHelp(t *testing.T) {
	cmd := newRootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("root help failed: %v", err)
	}

	content := out.String()
	assertContains(t, content, "grove 是当前仓库唯一保留的 CLI 入口")
	assertContains(t, content, "about")
	assertContains(t, content, "doctor")
	assertContains(t, content, "rbac")
	assertContains(t, content, "make:module")
}

func TestRepositoryCommandsUseGroveCLI(t *testing.T) {
	root := filepath.Join("..", "..")
	makefile := mustRead(t, filepath.Join(root, "Makefile"))
	readme := mustRead(t, filepath.Join(root, "README.md"))

	assertContains(t, makefile, "GROVE := $(GO) run ./cmd/grove")
	assertNotContains(t, makefile, "cmd/artisan")
	assertContains(t, readme, "go run ./cmd/grove")
	assertNotContains(t, readme, "cmd/artisan")
}

func TestMakefileUsesCurrentConsoleScripts(t *testing.T) {
	makefile := mustRead(t, filepath.Join("..", "..", "Makefile"))

	assertContains(t, makefile, "ADMIN_DIR := web/admin-vben")
	assertContains(t, makefile, "cd $(ADMIN_DIR) && $(PNPM) install --frozen-lockfile")
	assertContains(t, makefile, "cd $(ADMIN_DIR) && $(PNPM) dev:console")
	assertContains(t, makefile, "cd $(ADMIN_DIR) && $(PNPM) build:console")
	assertContains(t, makefile, "cd $(ADMIN_DIR) && $(PNPM) --filter @grove/console typecheck")
	assertNotContains(t, makefile, "install:admin-vben")
	assertNotContains(t, makefile, "dev:admin")
	assertNotContains(t, makefile, "build:admin")
	assertNotContains(t, makefile, "typecheck:admin")
}

func TestSeedCommandListsExplicitSafetyModes(t *testing.T) {
	cmd := newSeedCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("seed help failed: %v", err)
	}

	content := out.String()
	assertContains(t, content, "bootstrap")
	assertContains(t, content, "demo")
}

func TestMigrateCommandSupportsCustomSourcePath(t *testing.T) {
	cmd := newMigrateCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("migrate help failed: %v", err)
	}
	assertContains(t, out.String(), "--path")
}

func TestMigrateCreateUsesConfiguredDialectDirectory(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "migrations")
	mustMkdir(t, filepath.Join(base, "postgres"))
	mustMkdir(t, filepath.Join(base, "mysql"))
	configPath := filepath.Join(root, "config.yaml")
	mustWrite(t, configPath, `databases:
  default:
    driver: mysql
`)

	previousConfigFile := configFile
	configFile = configPath
	t.Cleanup(func() { configFile = previousConfigFile })

	cmd := newMigrateCmd()
	cmd.SetArgs([]string{"--path", base, "create", "create_articles"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("create mysql migration: %v", err)
	}
	files, err := filepath.Glob(filepath.Join(base, "mysql", "*.sql"))
	if err != nil {
		t.Fatalf("list created mysql migrations: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected two mysql migration files, got %d", len(files))
	}
	postgresFiles, _ := filepath.Glob(filepath.Join(base, "postgres", "*.sql"))
	if len(postgresFiles) != 0 {
		t.Fatalf("migration create wrote PostgreSQL files: %v", postgresFiles)
	}
}

func TestDemoSeedRefusesProduction(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, "config.yaml")
	mustWrite(t, configPath, `app:
  env: production
jwt:
  secret: 0123456789abcdef0123456789abcdef
`)
	t.Setenv("APP_ENV", "")
	t.Setenv("JWT_SECRET", "")

	previousConfigFile := configFile
	configFile = configPath
	t.Cleanup(func() { configFile = previousConfigFile })

	cmd := newSeedCmd()
	cmd.SetArgs([]string{"demo"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected production demo seed to be rejected")
	}
	assertContains(t, err.Error(), "production")
}

func TestBootstrapSeedDoesNotOverwriteRootPassword(t *testing.T) {
	root := filepath.Join("..", "..")
	seed := mustRead(t, filepath.Join(root, "database", "seeds", "postgres", "bootstrap", "202604150002_console_root_super_admin.sql"))

	assertContains(t, seed, "{{GROVE_ROOT_PASSWORD_HASH}}")
	assertContains(t, seed, "ON CONFLICT DO NOTHING")
	assertNotContains(t, seed, "password = EXCLUDED.password")
}

func TestSeedFilesAreSplitBySafetyBoundary(t *testing.T) {
	root := filepath.Join("..", "..", "database", "seeds")
	for _, dialect := range []string{"postgres", "mysql"} {
		for _, path := range []string{
			filepath.Join(root, dialect, "bootstrap", "202604150001_system_configs.sql"),
			filepath.Join(root, dialect, "bootstrap", "202604150002_console_root_super_admin.sql"),
			filepath.Join(root, dialect, "demo", "202604150001_api_user.sql"),
			filepath.Join(root, dialect, "demo", "202604150002_console_admin.sql"),
		} {
			if _, err := os.Stat(path); err != nil {
				t.Errorf("expected seed file %s: %v", path, err)
			}
		}
	}
}

func TestResolveRootPasswordUsesEnvironment(t *testing.T) {
	t.Setenv(rootPasswordEnv, "configured-root-password")

	password, generated, err := resolveRootPassword(nil)
	if err != nil {
		t.Fatalf("resolve configured root password: %v", err)
	}
	if generated {
		t.Fatal("configured root password must not be marked as generated")
	}
	if password != "configured-root-password" {
		t.Fatalf("unexpected configured password: %q", password)
	}
}

func TestResolveRootPasswordPrefersConfig(t *testing.T) {
	t.Setenv(rootPasswordEnv, "environment-root-password")
	cfg := &config.Config{}
	cfg.Security.InitialRootPassword = "configured-root-password"

	password, generated, err := resolveRootPassword(cfg)
	if err != nil {
		t.Fatalf("resolve config root password: %v", err)
	}
	if generated {
		t.Fatal("config root password must not be marked as generated")
	}
	if password != "configured-root-password" {
		t.Fatalf("unexpected config password: %q", password)
	}
}

func TestResolveRootPasswordGeneratesRandomValue(t *testing.T) {
	t.Setenv(rootPasswordEnv, "")

	password, generated, err := resolveRootPassword(nil)
	if err != nil {
		t.Fatalf("generate root password: %v", err)
	}
	if !generated {
		t.Fatal("missing root password must generate a one-time value")
	}
	if len(password) < 24 {
		t.Fatalf("generated password is unexpectedly short: %d", len(password))
	}
}

func TestDoctorCommandPrintsConfigSummary(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "config.yaml"), `app:
  name: demo-app
  env: test
port: "18080"
console_port: "18081"
databases:
  default:
    enabled: true
    driver: postgres
    host: 127.0.0.1
    port: "5432"
    user: grove
    dbname: grove
redis:
  enabled: true
job:
  enabled: false
casbin:
  enforcers:
    api:
      enabled: true
    console:
      enabled: false
storage:
  default: local
docs:
  enabled: true
`)

	cmd := newDoctorCmd()
	cmd.Flags().StringVarP(&configFile, "config", "c", "", "配置文件路径")
	cmd.SetArgs([]string{"-c", filepath.Join(root, "config.yaml")})

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor command failed: %v", err)
	}

	content := out.String()
	assertContains(t, content, "应用名称: demo-app")
	assertContains(t, content, "默认数据库: 已启用")
	assertContains(t, content, "Redis: 已启用")
	assertContains(t, content, "任务队列: 未启用")
}

func TestMakeModuleGeneratesConsoleModuleTemplate(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "app/console/internal/router"))
	mustMkdir(t, filepath.Join(root, "app/console/internal/service"))
	mustMkdir(t, filepath.Join(root, "app/console/internal/handler"))
	mustMkdir(t, filepath.Join(root, "internal/model"))
	mustWrite(t, filepath.Join(root, "app/console/internal/router/router.go"), `package router

func register() {
	// grove:register-routes
}
`)

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir temp root: %v", err)
	}
	defer func() {
		if err := os.Chdir(previousWD); err != nil {
			t.Fatalf("restore wd: %v", err)
		}
	}()

	cmd := newMakeModuleCmd()
	cmd.SetArgs([]string{"ProductCategory"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("make:module failed: %v", err)
	}

	service := mustRead(t, filepath.Join(root, "app/console/internal/service/product_category.go"))
	assertContains(t, service, "package service")
	assertContains(t, service, "type ProductCategoryService struct")
	assertContains(t, service, "database.Connections")
	assertContains(t, service, "errx.ServiceUnavailable")

	model := mustRead(t, filepath.Join(root, "internal/model/product_category.go"))
	assertContains(t, model, `return "product_categories"`)

	handler := mustRead(t, filepath.Join(root, "app/console/internal/handler/product_category.go"))
	assertContains(t, handler, "package handler")
	assertContains(t, handler, "RegisterProductCategoryRoutes")
	assertContains(t, handler, "route.Wrap(protected.Group(\"/product-categories\"))")
	assertContains(t, handler, ".Name(\"ProductCategory.列表\")")
	assertContains(t, handler, "response.Success")

	router := mustRead(t, filepath.Join(root, "app/console/internal/router/router.go"))
	assertContains(t, router, "\thandler.RegisterProductCategoryRoutes(protected, r.p)\n")
}

func TestMakeModuleGeneratedPackagesCompile(t *testing.T) {
	root := prepareModuleCompileWorkspace(t)

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir temp root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousWD) })

	cmd := newMakeModuleCmd()
	cmd.SetArgs([]string{"ProductCategory"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("make:module failed: %v", err)
	}

	goTest := exec.Command("go", "test", "./internal/model", "./app/console/internal/service", "./app/console/internal/handler", "./app/console/internal/router")
	goTest.Dir = root
	output, err := goTest.CombinedOutput()
	if err != nil {
		t.Fatalf("generated packages do not compile: %v\n%s", err, output)
	}
}

func TestMakeModulePreflightsBeforeWriting(t *testing.T) {
	root := prepareModuleWorkspace(t, `package router

func register() {
	// marker missing on purpose
}
`)

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir temp root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousWD) })

	cmd := newMakeModuleCmd()
	cmd.SetArgs([]string{"ProductCategory"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected missing router marker error")
	}

	for _, path := range []string{
		"internal/model/product_category.go",
		"app/console/internal/service/product_category.go",
		"app/console/internal/handler/product_category.go",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("preflight failure left generated file %s", path)
		}
	}
}

func TestMakeModuleRejectsInvalidNameBeforeWriting(t *testing.T) {
	root := prepareModuleWorkspace(t, `package router

func register() {
	// grove:register-routes
}
`)

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir temp root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousWD) })

	cmd := newMakeModuleCmd()
	cmd.SetArgs([]string{"123-product"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected invalid Go identifier error")
	}

	if _, err := os.Stat(filepath.Join(root, "internal/model/123_product.go")); !os.IsNotExist(err) {
		t.Fatal("invalid module name left a generated file")
	}
}

func TestMakeModuleChecksAllTargetsBeforeWriting(t *testing.T) {
	root := prepareModuleWorkspace(t, `package router

func register() {
	// grove:register-routes
}
`)
	existingService := filepath.Join(root, "app/console/internal/service/product_category.go")
	mustWrite(t, existingService, "package service\n")

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get wd: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir temp root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousWD) })

	cmd := newMakeModuleCmd()
	cmd.SetArgs([]string{"ProductCategory"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected existing service error")
	}
	for _, path := range []string{
		"internal/model/product_category.go",
		"app/console/internal/handler/product_category.go",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
			t.Fatalf("target preflight left generated file %s", path)
		}
	}
}

func TestCommitGeneratedModuleRollsBackFilesWhenRouterWriteFails(t *testing.T) {
	root := t.TempDir()
	routerPath := filepath.Join(root, "router")
	mustMkdir(t, routerPath)
	sources := []generatedSource{
		{path: filepath.Join(root, "model.go"), content: []byte("package model\n")},
		{path: filepath.Join(root, "service.go"), content: []byte("package service\n")},
	}

	if err := commitGeneratedModule(sources, routerPath, []byte("package router\n")); err == nil {
		t.Fatal("expected router replacement failure")
	}
	for _, source := range sources {
		if _, err := os.Stat(source.path); !os.IsNotExist(err) {
			t.Fatalf("router failure left generated file %s", source.path)
		}
	}
}

func prepareModuleWorkspace(t *testing.T, router string) string {
	t.Helper()
	root := t.TempDir()
	repoRoot := filepath.Join("..", "..")
	for _, dir := range []string{"internal", "pkg", "app/console"} {
		copyTree(t, filepath.Join(repoRoot, dir), filepath.Join(root, dir))
	}
	mustMkdir(t, filepath.Join(root, "app/console/internal/router"))
	mustWrite(t, filepath.Join(root, "app/console/internal/router/router.go"), router)
	copyFile(t, filepath.Join(repoRoot, "go.mod"), filepath.Join(root, "go.mod"))
	copyFile(t, filepath.Join(repoRoot, "go.sum"), filepath.Join(root, "go.sum"))
	return root
}

func prepareModuleCompileWorkspace(t *testing.T) string {
	return prepareModuleWorkspace(t, `package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zhimma/grove/app/console/internal/handler"
	"github.com/zhimma/grove/internal/provider"
)

type Router struct { p *provider.Provider }

func (r *Router) register(protected *gin.RouterGroup) {
	// grove:register-routes
	_ = handler.RegisterProductCategoryRoutes
}
`)
}

func copyTree(t *testing.T, source, target string) {
	t.Helper()
	err := filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, rel)
		if info.IsDir() {
			return os.MkdirAll(destination, info.Mode().Perm())
		}
		return copyFile(t, path, destination)
	})
	if err != nil {
		t.Fatalf("copy tree %s: %v", source, err)
	}
}

func copyFile(t *testing.T, source, target string) error {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}
	in, err := os.Open(filepath.Clean(source))
	if err != nil {
		t.Fatalf("open source %s: %v", source, err)
	}
	defer in.Close()
	out, err := os.OpenFile(filepath.Clean(target), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("create target %s: %v", target, err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatalf("copy %s: %v", source, err)
	}
	return nil
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

func assertContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Fatalf("expected content to contain %q\ncontent:\n%s", needle, haystack)
	}
}

func assertNotContains(t *testing.T, haystack string, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Fatalf("expected content not to contain %q\ncontent:\n%s", needle, haystack)
	}
}
