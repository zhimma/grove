BIN_DIR := bin
ADMIN_DIR := web/admin-vben
GO ?= go
PNPM ?= pnpm
GROVE := $(GO) run ./cmd/grove

.DEFAULT_GOAL := help

.PHONY: \
	help \
	run.api run.console run.worker \
	test fmt tidy build verify \
	admin.install admin.dev admin.build admin.typecheck \
	migrate.up migrate.down migrate.status \
	seed.bootstrap seed.demo

help: ## 显示常用命令
	@awk 'BEGIN {FS = ":.*## "; printf "\nUsage:\n  make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_.-]+:.*## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

run.api: ## 启动 API 服务（:8080）
	$(GO) run ./app/api/cmd/main.go

run.console: ## 启动 Console 服务（:8081）
	$(GO) run ./app/console/cmd/main.go

run.worker: ## 启动 Worker 服务（:8082）
	$(GO) run ./app/worker/cmd/main.go

test: ## 运行全部 Go 测试
	$(GO) test ./...

fmt: ## 格式化全部 Go 代码
	$(GO) fmt ./...

tidy: ## 整理 Go 模块依赖
	$(GO) mod tidy

build: ## 构建 API、Console、Worker 二进制
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/api ./app/api/cmd/main.go
	$(GO) build -o $(BIN_DIR)/console ./app/console/cmd/main.go
	$(GO) build -o $(BIN_DIR)/worker ./app/worker/cmd/main.go

verify: ## 运行 Go 测试、后端构建和 Console 类型检查
	$(GO) test ./...
	$(MAKE) build
	$(MAKE) admin.typecheck

admin.install: ## 安装管理后台依赖
	cd $(ADMIN_DIR) && $(PNPM) install --frozen-lockfile

admin.dev: ## 启动管理后台前端开发服务（:5666）
	cd $(ADMIN_DIR) && $(PNPM) dev:console

admin.build: ## 构建管理后台前端
	cd $(ADMIN_DIR) && $(PNPM) build:console

admin.typecheck: ## 检查管理后台 TypeScript 类型
	cd $(ADMIN_DIR) && $(PNPM) --filter @grove/console typecheck

migrate.up: ## 执行数据库迁移
	$(GROVE) migrate up

migrate.down: ## 回滚最近一次数据库迁移
	$(GROVE) migrate down

migrate.status: ## 查看数据库迁移状态
	$(GROVE) migrate status

seed.bootstrap: ## 创建基础配置和 root 管理员
	$(GROVE) seed bootstrap

seed.demo: ## 写入开发/测试演示数据（production 禁止）
	$(GROVE) seed demo
