# 项目结构

本文档详细介绍 grove 框架的目录结构和组织方式。

## 目录概览

```
grove/
├── app/                    # 应用层（多服务）
├── cmd/                    # 命令行工具
├── internal/               # 内部实现
├── pkg/                    # 公共包
├── database/               # 数据库相关
├── docs/                   # 文档
├── web/                    # 前端代码
├── config.yaml             # 配置文件
├── go.mod                  # Go 模块
└── Makefile               # 构建脚本
```

## 详细说明

### app/ - 应用层

包含多个独立的服务（应用），每个服务可以独立部署。

```
app/
├── api/                    # 对外 API 服务
│   ├── handler/           # HTTP 处理器
│   ├── internal/          # 内部实现
│   │   ├── middleware/    # 中间件
│   │   └── router/        # 路由定义
│   └── service/           # 业务服务
│
├── console/               # 管理后台服务（主服务）
│   ├── handler/           # HTTP 处理器
│   ├── internal/
│   │   ├── middleware/    # 认证、权限等中间件
│   │   └── router/        # 路由定义
│   └── service/           # 业务服务
│
└── worker/                # 异步任务服务
    ├── handler/           # 任务处理器
    └── internal/
        └── router/        # 任务路由
```

### cmd/ - 命令行工具

```
cmd/
└── artisan/               # CLI 工具
    └── main.go            # 入口文件
```

Artisan 提供以下命令：
- `migrate create` - 创建迁移文件
- `migrate up/down` - 执行/回滚迁移
- `make:module` - 生成完整模块
- `make:model` - 生成模型
- `make:service` - 生成服务
- `make:handler` - 生成处理器
- `seed run` - 运行种子数据

### internal/ - 内部实现

```
internal/
├── bootstrap/             # 启动加载
│   └── middleware.go      # 全局中间件加载
│
├── config/                # 配置加载
│   ├── load.go           # 配置文件解析
│   └── types.go          # 配置类型定义
│
├── middleware/            # 中间件
│   ├── access_log.go     # 访问日志
│   ├── cors.go           # 跨域处理
│   ├── recovery.go       # Panic 恢复
│   ├── request_id.go     # 请求 ID
│   └── request_meta.go   # 请求元信息
│
├── model/                 # 数据模型
│   ├── admin.go          # 管理员模型
│   ├── role.go           # 角色模型
│   └── ...               # 其他模型
│
└── provider/              # 依赖注入容器
    └── provider.go       # Provider 定义和初始化
```

### pkg/ - 公共包

可复用的公共组件，不依赖具体业务逻辑。

```
pkg/
├── auth/                  # JWT 认证
│   └── token.go          # Token 生成和验证
│
├── cache/                 # 缓存封装
│   ├── store.go          # 缓存接口
│   ├── memory.go         # 内存缓存实现
│   └── redis.go          # Redis 缓存实现
│
├── casbin/                # 权限控制
│   └── casbin.go         # Casbin 封装
│
├── database/              # 数据库
│   └── database.go       # 多数据源支持
│
├── errors/                # 错误处理
│   └── errors.go         # HTTPError 定义
│
├── event/                 # 事件系统
│   └── dispatcher.go     # 事件分发器
│
├── httpclient/            # HTTP 客户端
│   └── client.go         # 链式 HTTP 客户端
│
├── job/                   # 队列任务
│   └── job.go            # Asynq 封装
│
├── logger/                # 日志
│   └── logger.go         # Zerolog 封装
│
├── migrate/               # 数据库迁移
│   └── migrate.go        # 迁移工具
│
├── permission/            # 权限
│   ├── permission.go     # API 权限标识与运行时路由收集
│   └── menu_catalog.go   # Console 菜单权限静态清单
│
├── reqctx/                # 请求上下文
│   └── context.go        # Gin 上下文扩展
│
├── response/              # 响应封装
│   └── response.go       # 统一响应格式
│
├── route/                 # 路由工具
│   └── wrap.go           # 路由包装器
│
├── scheduler/             # 计划任务
│   └── scheduler.go      # Cron 任务调度
│
├── storage/               # 文件存储
│   ├── manager.go        # 存储管理器
│   ├── local.go          # 本地存储
│   └── s3.go             # S3 存储
│
├── transaction/           # 事务管理
│   └── manager.go        # GORM 事务封装
│
└── validation/            # 参数验证
    └── request.go        # 请求验证
```

### database/ - 数据库相关

```
database/
├── migrations/            # 迁移文件
│   ├── 20240101000001_create_users_table.up.sql
│   ├── 20240101000001_create_users_table.down.sql
│   └── ...
│
└── seeds/                 # 种子数据
    └── admin_seed.go     # 管理员种子数据
```

### docs/ - 文档

```
docs/
├── README.md             # 文档首页
├── guide/                # 使用指南
│   ├── structure.md     # 本文档
│   ├── configuration.md
│   ├── quickstart.md
│   ├── routing.md
│   ├── database.md
│   ├── service.md
│   ├── permission.md
│   ├── cache.md
│   ├── event.md
│   ├── scheduler.md
│   ├── httpclient.md
│   └── queue.md
│
├── api/                  # API 参考
│   ├── config.md
│   ├── database.md
│   ├── cache.md
│   ├── event.md
│   └── scheduler.md
│
├── deployment/           # 部署文档
│   ├── deploy.md
│   ├── docker.md
│   └── monitoring.md
│
└── development/          # 开发规范
    ├── code-style.md
    ├── testing.md
    └── error-handling.md
```

### web/ - 前端代码

```
web/
└── admin-vben/           # 管理后台前端（Vue3 + Vben Admin）
    └── apps/
        └── console/      # Console 对应的前端
```

## 架构原则

### 1. 分层架构

```
HTTP Request
    ↓
Handler (HTTP 层)
    ↓
Service (业务层)
    ↓
Model (数据层)
    ↓
Database
```

- **Handler**: 只处理 HTTP 相关逻辑，不直接操作数据库
- **Service**: 封装业务逻辑，管理事务
- **Model**: 定义数据结构和查询方法

### 2. 依赖注入

使用 Provider 模式管理依赖：

```go
type Provider struct {
    Config       *config.Config
    DB           database.Repo
    RedisClient  *redis.Client
    TokenManager *auth.Manager
    Cache        *cache.Manager
    HTTPClient   *httpclient.Client
    Event        *event.Dispatcher
    Scheduler    *scheduler.Scheduler
    // ...
}
```

### 3. 代码组织

**按功能组织而非按类型组织**：

```
# 推荐 ✅
app/console/handler/article.go
app/console/service/article.go
internal/model/article.go

# 不推荐 ❌
handler/console/article.go
service/console/article.go
model/article.go
```

### 4. 命名规范

| 类型 | 命名规则 | 示例 |
|------|---------|------|
| 包名 | 小写，简短 | `handler`, `service` |
| 文件名 | 小写，下划线分隔 | `article_handler.go` |
| 结构体 | 大驼峰 | `ArticleHandler` |
| 接口 | 大驼峰，名词 | `Service`, `Repository` |
| 函数 | 大驼峰 | `CreateArticle` |
| 常量 | 大驼峰 | `DefaultPageSize` |
| 变量 | 小驼峰 | `articleID` |

## 数据流

### 请求处理流程

```
1. HTTP Request
   ↓
2. Gin Router
   ↓
3. Middleware Chain
   - RequestID
   - AccessLog
   - Recovery
   - CORS
   - Auth (if protected)
   - Permission (if protected)
   ↓
4. Handler
   - Bind params
   - Validate input
   - Call Service
   - Return response
   ↓
5. Service
   - Business logic
   - Transaction (if needed)
   - Call Model
   - Dispatch Event (if needed)
   ↓
6. Model
   - Database query
   ↓
7. Database
```

### 响应格式

```json
{
  "code": 0,
  "message": "ok",
  "data": {},
  "request_id": "uuid"
}
```

错误响应：

```json
{
  "code": -1,
  "message": "错误信息",
  "data": {
    "error_code": "invalid_params",
    "errors": {
      "field": ["错误详情"]
    }
  },
  "request_id": "uuid"
}
```

## 配置文件

### config.yaml 结构

```yaml
app:
  name: grove
  env: development

server:
  shutdown_timeout: 30
  read_timeout: 30
  write_timeout: 30

databases:
  default:
    enabled: true
    driver: postgres
    host: 127.0.0.1
    # ...

redis:
  enabled: true
  addr: 127.0.0.1:6379

jwt:
  secret: your-secret
  issuer: grove

cache:
  default: redis
  stores:
    memory:
      driver: memory
    redis:
      driver: redis

event:
  async: true
  queue_size: 1000

scheduler:
  enabled: true
  timezone: Asia/Shanghai
```

## 扩展开发

### 添加新的 Provider Option

```go
func WithCustomService() Option {
    return func(p *Provider) error {
        p.CustomService = NewCustomService(p)
        return nil
    }
}
```

### 添加新的中间件

```go
func CustomMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 前置处理
        
        c.Next()
        
        // 后置处理
    }
}
```

### 添加新的 Artisan 命令

```go
// cmd/artisan/command/custom.go
var CustomCmd = &cobra.Command{
    Use:   "custom",
    Short: "Custom command",
    Run: func(cmd *cobra.Command, args []string) {
        // 命令逻辑
    },
}
```

## 最佳实践

1. **保持 Handler 简洁** - 只处理 HTTP 相关逻辑
2. **Service 层处理业务** - 复杂的业务逻辑放在 Service
3. **Model 层处理数据** - 数据库查询封装在 Model
4. **使用 Provider** - 通过 Provider 获取依赖，不要全局变量
5. **错误处理** - 使用 `pkg/errors` 包的错误类型
6. **日志记录** - 使用 `pkg/logger` 记录日志
7. **参数验证** - 使用 `pkg/validation` 验证请求参数
8. **事务管理** - 使用 `pkg/transaction` 管理数据库事务
