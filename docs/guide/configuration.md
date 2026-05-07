# 配置说明

本文档详细介绍 grove 框架的配置文件结构和各项配置选项。

## 配置文件位置

框架默认从以下位置加载配置：

1. `config.yaml` - 主配置文件（推荐）
2. `config.example.yaml` - 示例配置文件

## 环境变量

配置文件支持环境变量插值，语法为 `${VAR:default}`：

```yaml
app:
  name: ${APP_NAME:grove}
  env: ${APP_ENV:development}
```

常用环境变量：

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `APP_NAME` | 应用名称 | `grove` |
| `APP_ENV` | 运行环境 | `development` |
| `APP_PORT` | API 服务端口 | `8080` |
| `CONSOLE_PORT` | Console 服务端口 | `8081` |
| `DB_HOST` | 数据库主机 | `127.0.0.1` |
| `DB_PORT` | 数据库端口 | `5432` |
| `REDIS_ADDR` | Redis 地址 | `127.0.0.1:6379` |
| `JWT_SECRET` | JWT 密钥 | `change-me` |

## 完整配置示例

```yaml
# ==================== 应用配置 ====================
app:
  name: ${APP_NAME:grove}
  env: ${APP_ENV:development}  # development, production, testing

# ==================== 端口配置 ====================
port: ${APP_PORT:8080}              # API 服务端口
console_port: ${CONSOLE_PORT:8081}  # Console 服务端口

# ==================== 服务器配置 ====================
server:
  shutdown_timeout: ${SERVER_SHUTDOWN_TIMEOUT:30}     # 优雅关闭超时（秒）
  read_timeout: ${SERVER_READ_TIMEOUT:30}             # 读取超时（秒）
  write_timeout: ${SERVER_WRITE_TIMEOUT:30}           # 写入超时（秒）
  max_header_bytes: ${SERVER_MAX_HEADER_BYTES:1048576} # 最大请求头大小（字节）

# ==================== 日志配置 ====================
log:
  level: ${LOG_LEVEL:info}           # debug, info, warn, error
  path: ${LOG_PATH:./logs}           # 日志文件路径
  console: ${LOG_CONSOLE:true}       # 是否输出到控制台
  service: ${LOG_SERVICE:grove} # 服务名称

# ==================== 数据库配置 ====================
databases:
  default:
    enabled: ${DB_ENABLED:false}
    driver: ${DB_DRIVER:postgres}    # 目前只支持 postgres
    host: ${DB_HOST:127.0.0.1}
    port: ${DB_PORT:5432}
    user: ${DB_USER:postgres}
    password: ${DB_PASSWORD:postgres}
    dbname: ${DB_NAME:golang_web}
    ssl_mode: ${DB_SSLMODE:disable}
    max_connections: ${DB_MAX_CONNECTIONS:20}   # 最大连接数
    max_idle_conns: ${DB_MAX_IDLE_CONNS:10}     # 最大空闲连接
    conn_max_lifetime: ${DB_CONN_MAX_LIFETIME:3600}  # 连接最大生命周期（秒）
  
  # 额外数据源配置
  resources:
    orders:
      enabled: true
      driver: postgres
      host: 127.0.0.1
      port: 5432
      user: postgres
      password: postgres
      dbname: orders_db
      ssl_mode: disable

# ==================== Redis 配置 ====================
redis:
  enabled: ${REDIS_ENABLED:false}
  addr: ${REDIS_ADDR:127.0.0.1:6379}
  password: ${REDIS_PASSWORD:}
  db: ${REDIS_DB:0}

# ==================== JWT 配置 ====================
jwt:
  secret: ${JWT_SECRET:change-me}                    # 密钥（生产环境必须修改）
  issuer: ${JWT_ISSUER:grove}                   # 签发者
  access_expiry_hours: ${JWT_ACCESS_EXPIRY_HOURS:24}     # Access Token 过期时间
  refresh_expiry_hours: ${JWT_REFRESH_EXPIRY_HOURS:168}  # Refresh Token 过期时间（7天）

# ==================== 队列配置 ====================
job:
  enabled: ${WORKER_ENABLED:false}
  concurrency: ${JOB_CONCURRENCY:10}  # 并发数
  queues:
    default: ${JOB_QUEUE_DEFAULT:5}   # 默认队列权重
    critical: ${JOB_QUEUE_CRITICAL:3} # 关键队列权重
    low: ${JOB_QUEUE_LOW:1}           # 低优先级队列权重

# ==================== Casbin 权限配置 ====================
casbin:
  enforcers:
    api:
      enabled: ${CASBIN_API_ENABLED:false}
      database: ${CASBIN_API_DATABASE:default}
      mode: ${CASBIN_API_MODE:rbac}
      table_name: ${CASBIN_API_TABLE:casbin_rules}
    console:
      enabled: ${CASBIN_CONSOLE_ENABLED:false}
      database: ${CASBIN_CONSOLE_DATABASE:default}
      mode: ${CASBIN_CONSOLE_MODE:rbac}
      table_name: ${CASBIN_CONSOLE_TABLE:console_casbin_rules}

# ==================== 文件存储配置 ====================
storage:
  default: ${STORAGE_DEFAULT:local}  # 默认存储磁盘
  disks:
    local:
      driver: ${STORAGE_LOCAL_DRIVER:local}
      root: ${STORAGE_LOCAL_ROOT:./storage}        # 本地存储根目录
      base_url: ${STORAGE_LOCAL_BASE_URL:/storage} # 访问 URL 前缀
      prefix: ${STORAGE_LOCAL_PREFIX:console}      # 文件前缀
    
    s3:
      driver: ${STORAGE_S3_DRIVER:s3}
      endpoint: ${STORAGE_S3_ENDPOINT:s3.ap-southeast-1.amazonaws.com}
      region: ${STORAGE_S3_REGION:ap-southeast-1}
      bucket: ${STORAGE_S3_BUCKET:demo-bucket}
      access_key: ${STORAGE_S3_ACCESS_KEY:}
      secret_key: ${STORAGE_S3_SECRET_KEY:}
      secure: ${STORAGE_S3_SECURE:true}
      base_url: ${STORAGE_S3_BASE_URL:}
      prefix: ${STORAGE_S3_PREFIX:console}
      sts:
        enabled: ${STORAGE_STS_ENABLED:false}
        endpoint: ${STORAGE_STS_ENDPOINT:}
        region: ${STORAGE_STS_REGION:ap-southeast-1}
        role_arn: ${STORAGE_STS_ROLE_ARN:}
        role_session_name: ${STORAGE_STS_ROLE_SESSION_NAME:grove-console}
        duration: ${STORAGE_STS_DURATION:3600}
        allow_prefix:
          - ${STORAGE_STS_ALLOW_PREFIX:console/${user_id}}
        allow_actions:
          - ${STORAGE_STS_ALLOW_ACTION_1:s3:PutObject}
          - ${STORAGE_STS_ALLOW_ACTION_2:s3:GetObject}

# ==================== 缓存配置 ====================
cache:
  default: ${CACHE_DEFAULT:memory}  # 默认缓存存储
  prefix: ${CACHE_PREFIX:app}       # 键前缀

# ==================== 事件配置 ====================
event:
  async: ${EVENT_ASYNC:false}           # 是否启用异步处理
  queue_size: ${EVENT_QUEUE_SIZE:1000}  # 异步队列大小
  workers: ${EVENT_WORKERS:10}          # 工作协程数

# ==================== 调度器配置 ====================
scheduler:
  enabled: ${SCHEDULER_ENABLED:false}   # 是否启用
  timezone: ${SCHEDULER_TIMEZONE:Asia/Shanghai}  # 时区

# ==================== API 文档配置 ====================
docs:
  enabled: ${DOCS_ENABLED:true}
  title: ${DOCS_TITLE:Grove API}
  description: ${DOCS_DESCRIPTION:API framework scaffold}
  version: ${DOCS_VERSION:1.0.0}
  base_path: ${DOCS_BASE_PATH:/api/v1}
  schemes:
    - ${DOCS_SCHEME_1:http}

# ==================== CORS 配置 ====================
cors:
  enabled: ${CORS_ENABLED:true}
  allowed_origins:
    - ${CORS_ALLOWED_ORIGIN_1:*}
  allowed_methods:
    - ${CORS_ALLOWED_METHOD_1:GET}
    - ${CORS_ALLOWED_METHOD_2:POST}
    - ${CORS_ALLOWED_METHOD_3:PUT}
    - ${CORS_ALLOWED_METHOD_4:PATCH}
    - ${CORS_ALLOWED_METHOD_5:DELETE}
    - ${CORS_ALLOWED_METHOD_6:OPTIONS}
  allowed_headers:
    - ${CORS_ALLOWED_HEADER_1:Authorization}
    - ${CORS_ALLOWED_HEADER_2:Content-Type}
    - ${CORS_ALLOWED_HEADER_3:X-Request-Id}
  allow_credentials: ${CORS_ALLOW_CREDENTIALS:false}
  max_age: ${CORS_MAX_AGE:600}

# ==================== API 配置 ====================
api:
  prefix: ${API_PREFIX:/api/v1}
  default_per_page: ${API_DEFAULT_PER_PAGE:20}
  max_per_page: ${API_MAX_PER_PAGE:100}
```

## 配置加载

### 代码中访问配置

```go
import "github.com/zhimma/grove/internal/config"

// 通过 Provider 访问
func (s *Service) DoSomething() {
    // 获取数据库配置
    dbConfig := s.provider.Config.Databases.Default
    
    // 获取 JWT 配置
    jwtSecret := s.provider.Config.JWT.Secret
    
    // 获取应用名称
    appName := s.provider.Config.App.Name
}
```

### 配置结构定义

```go
// internal/config/types.go
type Config struct {
    App        AppConfig        `yaml:"app"`
    Port       string           `yaml:"port"`
    ConsolePort string          `yaml:"console_port"`
    Server     ServerConfig     `yaml:"server"`
    Log        LogConfig        `yaml:"log"`
    Databases  DatabaseConfigs  `yaml:"databases"`
    Redis      RedisConfig      `yaml:"redis"`
    JWT        JWTConfig        `yaml:"jwt"`
    Job        JobConfig        `yaml:"job"`
    Casbin     CasbinConfig     `yaml:"casbin"`
    Storage    StorageConfig    `yaml:"storage"`
    Docs       DocsConfig       `yaml:"docs"`
    CORS       CORSConfig       `yaml:"cors"`
    API        APIConfig        `yaml:"api"`
}
```

## 环境特定配置

### 开发环境

```yaml
app:
  env: development

log:
  level: debug
  console: true

databases:
  default:
    enabled: true
    host: localhost
```

### 生产环境

```yaml
app:
  env: production

log:
  level: warn
  console: false
  path: /var/log/grove

databases:
  default:
    enabled: true
    host: prod-db.internal
    max_connections: 100
```

### 测试环境

```yaml
app:
  env: testing

databases:
  default:
    enabled: true
    dbname: golang_web_test
```

## 配置验证

框架启动时会自动验证配置：

```go
// 检查必填配置
if cfg.JWT.Secret == "" || cfg.JWT.Secret == "change-me" {
    logger.Fatal().Msg("令牌密钥不能为空")
}

// 检查依赖关系
if cfg.Job.Enabled && !cfg.Redis.Enabled {
    logger.Fatal().Msg("任务队列需要启用 Redis")
}
```

## 配置热更新

目前框架不支持配置热更新。修改配置后需要重启服务。

如需热更新，可以考虑：
1. 使用配置中心（如 Consul, etcd）
2. 监听配置文件变化
3. 使用环境变量（12-Factor 推荐）

## 安全配置

### 生产环境检查清单

- [ ] 修改 JWT Secret（至少 32 位随机字符串）
- [ ] 关闭 Debug 模式
- [ ] 配置正确的数据库密码
- [ ] 启用 HTTPS
- [ ] 配置 CORS 白名单（不要使用 `*`）
- [ ] 限制日志级别为 warn 或更高
- [ ] 配置合适的连接池大小

### 敏感信息处理

不要将敏感信息提交到代码仓库：

```bash
# .gitignore
config.yaml
.env
*.pem
*.key
```

使用环境变量或密钥管理服务：

```yaml
# 使用 AWS Secrets Manager
jwt:
  secret: ${AWS_SECRET:jwt_secret}

# 或使用 HashiCorp Vault
jwt:
  secret: ${VAULT_SECRET:jwt_secret}
```

## 常见问题

### Q: 配置不生效？

检查：
1. 配置文件路径是否正确
2. 环境变量是否覆盖
3. YAML 格式是否正确（缩进使用空格）

### Q: 如何添加自定义配置？

1. 在 `internal/config/types.go` 添加配置结构
2. 在 `config.yaml` 添加配置项
3. 在代码中通过 `provider.Config` 访问

### Q: 支持多环境配置？

可以使用环境变量区分：

```bash
# 开发
APP_ENV=development go run main.go

# 生产
APP_ENV=production ./main
```

或使用不同的配置文件：

```go
configFile := os.Getenv("CONFIG_FILE")
if configFile == "" {
    configFile = "config.yaml"
}
cfg, err := config.Load(configFile)
```
