APP_MODULE := github.com/zhimma/grove
BIN_DIR := bin
ADMIN_DIR := web/admin-vben
GO ?= go
PNPM ?= pnpm
GROVE := $(GO) run ./cmd/grove

.DEFAULT_GOAL := help

.PHONY: \
	help \
	run run.api run.console run.worker \
	dev dev.console dev.worker \
	test test.go \
	fmt fmt.go \
	tidy \
	build build.api build.console build.worker \
	admin.install admin.dev admin.build admin.typecheck admin.verify \
	verify verify.go \
	migrate.up migrate.down migrate.status seed.bootstrap seed.demo

help: ## 显示常用命令
	@awk 'BEGIN {FS = ":.*## "; printf "\nUsage:\n  make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_.-]+:.*## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

run: run.api ## 运行 API 服务

run.api: ## 运行 API 服务
	$(GO) run ./app/api/cmd/main.go

run.console: ## 运行 Console 服务
	$(GO) run ./app/console/cmd/main.go

run.worker: ## 运行 Worker 服务
	$(GO) run ./app/worker/cmd/main.go

dev: run.api ## 兼容旧命令，运行 API 服务

dev.console: run.console ## 兼容旧命令，运行 Console 服务

dev.worker: run.worker ## 兼容旧命令，运行 Worker 服务

test: test.go ## 运行全部 Go 测试

test.go: ## 运行全部 Go 测试
	$(GO) test ./...

fmt: fmt.go ## 格式化全部 Go 代码

fmt.go: ## 格式化全部 Go 代码
	$(GO) fmt ./...

tidy: ## 整理 Go 模块依赖
	$(GO) mod tidy

build: build.api build.console build.worker ## 构建全部 Go 二进制

build.api: ## 构建 API 二进制到 bin/api
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/api ./app/api/cmd/main.go

build.console: ## 构建 Console 二进制到 bin/console
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/console ./app/console/cmd/main.go

build.worker: ## 构建 Worker 二进制到 bin/worker
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/worker ./app/worker/cmd/main.go

admin.install: ## 安装后台前端依赖
	cd $(ADMIN_DIR) && $(PNPM) install --frozen-lockfile

admin.dev: ## 启动后台前端开发服务
	cd $(ADMIN_DIR) && $(PNPM) dev:console

admin.build: ## 构建后台前端
	cd $(ADMIN_DIR) && $(PNPM) build:console

admin.typecheck: ## 执行后台前端类型检查
	cd $(ADMIN_DIR) && $(PNPM) --filter @grove/console typecheck

admin.verify: admin.typecheck ## 校验后台前端

verify: verify.go build admin.typecheck ## 校验后端测试、构建与前端类型检查

verify.go: test.go ## 校验 Go 代码

migrate.up: ## 执行数据库迁移
	$(GROVE) migrate up

migrate.down: ## 回滚最近一次数据库迁移
	$(GROVE) migrate down

migrate.status: ## 查看数据库迁移状态
	$(GROVE) migrate status

seed.bootstrap: ## 执行生产安全的基础种子
	$(GROVE) seed bootstrap

seed.demo: ## 执行开发和测试演示种子（生产环境禁止）
	$(GROVE) seed demo
