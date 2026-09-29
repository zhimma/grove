BIN_DIR := bin
ADMIN_DIR := web/admin-vben
GO ?= go
# Run pnpm through corepack so the version comes from web/admin-vben's
# packageManager field, the same way CI does. A globally installed pnpm of a
# different major otherwise fails every admin.* target with ERR_PNPM_UNSUPPORTED_ENGINE.
PNPM ?= corepack pnpm
GOLANGCI_LINT ?= golangci-lint
GOVULNCHECK ?= govulncheck
GROVE := $(GO) run ./cmd/grove

.DEFAULT_GOAL := help

.PHONY: \
	help \
	run.api run.console run.worker \
	test test.race contracts fmt tidy build verify ci \
	quality quality.go.fmt quality.go.vet quality.go.lint quality.govuln docs.check diff.check \
	admin.install admin.dev admin.build admin.typecheck admin.lint admin.circular admin.test \
	admin.contract \
	migrate.up migrate.down migrate.status \
	seed.bootstrap seed.demo

help: ## 显示常用命令
	@awk 'BEGIN {FS = ":.*## "; printf "\nUsage:\n  make <target>\n\nTargets:\n"} /^[a-zA-Z0-9_.-]+:.*## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

run.api: ## 启动 API 服务（:8080）
	$(GO) run ./app/api/cmd

run.console: ## 启动 Console 服务（:8081）
	$(GO) run ./app/console/cmd

run.worker: ## 启动 Worker 服务（:8082）
	$(GO) run ./app/worker/cmd

test: ## 运行全部 Go 测试
	$(GO) test ./...

fmt: ## 格式化全部 Go 代码
	$(GO) fmt ./...

tidy: ## 整理 Go 模块依赖
	$(GO) mod tidy

build: ## 构建 API、Console、Worker 和 Grove CLI 二进制
	mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/api ./app/api/cmd
	$(GO) build -o $(BIN_DIR)/console ./app/console/cmd
	$(GO) build -o $(BIN_DIR)/worker ./app/worker/cmd
	$(GO) build -o $(BIN_DIR)/grove ./cmd/grove

verify: ## 运行 Go 测试、后端构建和 Console 类型检查
	$(GO) test ./...
	$(MAKE) build
	$(MAKE) admin.typecheck

test.race: ## 运行全部 Go race 测试
	$(GO) test -race ./...

contracts: ## 检查 API 和 Console 路由/OpenAPI 合同
	$(GO) test ./app/api/internal/docs ./app/console/internal/docs -run 'Contract' -v

quality: quality.go.fmt quality.go.any quality.go.password quality.go.vet docs.check diff.check admin.lint admin.circular ## 运行本地可用的格式、静态与前端质量检查

docs.check: ## 检查 canonical 文档中的架构示例是否与当前代码一致
	@files='docs/01-开发规范.md docs/02-console-架构与权限.md docs/03-console-新增模块指南.md docs/guide/service.md docs/guide/pkg-components.md docs/guide/cache.md docs/guide/permission.md docs/guide/httpclient.md docs/guide/event.md'; \
	for file in $$files; do test -f "$$file" || { echo "缺少文档文件：$$file"; exit 1; }; done; \
	if rg -n -F \
		-e 'provider *provider.Provider' \
		-e 'RegisterArticleRoutes(protected, p)' \
		-e 'route.WrapWithCatalog(' \
		-e 'catalogs ...*route.Catalog' \
		-e 'response.OK(' \
		-e 'response.Error(' \
		-e 'httpclient.NewWithConfig(' \
		-e 'event.NewDispatcher(' \
		-e 'event.NewAsync(' \
		-e '业务代码优先通过 `internal/provider.Provider`' \
		-e '数据库通过 `provider.DB` 的命名资源访问' \
		$$files; then \
		echo '文档架构示例已过期：请使用显式依赖和 route.Wrap(group, catalog)。'; \
		exit 1; \
	fi
	@if rg -n -e '/Users/[a-z]' -e '/home/[a-z]' --glob 'docs/**/*.md' --glob '!docs/plans/**' --glob 'README.md' --glob 'AGENTS.md' .; then \
		echo '文档中出现了个人机器路径：fork 本仓库的人无法照做，请改成通用命令或占位符。'; \
		exit 1; \
	fi

quality.go.any: ## 检查 Go 代码统一使用 any 而非 interface{}
	@files=$$(find . -name '*.go' -not -path './web/*' -not -path './vendor/*' -print); \
	if [ -n "$$files" ] && rg -n -F 'interface{}' $$files; then \
		echo '请使用 any 代替 interface{}（Go 1.18 起为惯例）。'; \
		exit 1; \
	fi

quality.go.password: ## 检查密码哈希只在 pkg/password 中实现
	@files=$$(find . -name '*.go' -not -name '*_test.go' -not -path './web/*' -not -path './vendor/*' -not -path './pkg/password/*' -print); \
	if [ -n "$$files" ] && rg -n -F 'bcrypt.' $$files; then \
		echo '密码哈希请通过 pkg/password 调用，不要直接使用 bcrypt（cost 与恒定时间比较需保持一致）。测试可用 bcrypt.MinCost 保持快速。'; \
		exit 1; \
	fi

quality.go.fmt: ## 检查 Go 格式（不改写文件）
	@files=$$(find . -name '*.go' -not -path './web/*' -not -path './vendor/*' -print); \
	if [ -n "$$files" ] && [ -n "$$(gofmt -l $$files)" ]; then \
		gofmt -l $$files; \
		exit 1; \
	fi

quality.go.vet: ## 运行 Go vet
	$(GO) vet ./...

quality.go.lint: ## 运行 golangci-lint（需预先安装或在 CI 提供）
	@command -v "$(GOLANGCI_LINT)" >/dev/null 2>&1 || { echo "缺少 $(GOLANGCI_LINT)：请安装后重试，或仅运行 quality。"; exit 127; }
	$(GOLANGCI_LINT) run

quality.govuln: ## 运行 govulncheck（需预先安装或在 CI 提供）
	@command -v "$(GOVULNCHECK)" >/dev/null 2>&1 || { echo "缺少 $(GOVULNCHECK)：请安装后重试，或仅运行 quality。"; exit 127; }
	$(GOVULNCHECK) ./...

diff.check: ## 检查当前改动或 DIFF_BASE 到 HEAD 的空白错误
	@if [ -n "$(DIFF_BASE)" ] && git rev-parse --verify -q "$(DIFF_BASE)^{commit}" >/dev/null; then \
		git diff --check "$(DIFF_BASE)...HEAD"; \
	else \
		git diff --check; \
	fi

ci: quality quality.go.lint quality.govuln test test.race contracts build admin.typecheck admin.contract admin.test admin.build ## 运行完整 CI 质量门禁（先执行 admin.install）

admin.install: ## 安装管理后台依赖
	cd $(ADMIN_DIR) && $(PNPM) install --frozen-lockfile

admin.dev: ## 启动管理后台前端开发服务（:5666）
	cd $(ADMIN_DIR) && $(PNPM) dev:console

admin.build: ## 构建管理后台前端
	cd $(ADMIN_DIR) && $(PNPM) build:console

admin.typecheck: ## 检查管理后台 TypeScript 类型
	cd $(ADMIN_DIR) && $(PNPM) --filter @grove/console typecheck

admin.contract: ## 检查 Console 前端 API 合同注册表
	cd $(ADMIN_DIR) && $(PNPM) check:console-api-contract

admin.lint: ## 检查管理后台 ESLint、Stylelint 与格式
	cd $(ADMIN_DIR) && $(PNPM) lint

admin.circular: ## 检查管理后台循环依赖
	cd $(ADMIN_DIR) && $(PNPM) check:circular

admin.test: ## 运行管理后台单元测试
	cd $(ADMIN_DIR) && $(PNPM) test:unit

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
