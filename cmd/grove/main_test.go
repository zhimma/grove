package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	assertContains(t, content, "make:module")
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
	assertContains(t, service, "database.Repo")
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
