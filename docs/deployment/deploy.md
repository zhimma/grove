# 部署指南

本文档介绍 grove 框架的生产环境部署。

## 环境要求

### 服务器配置

| 组件 | 最低配置 | 推荐配置 |
|------|---------|---------|
| CPU | 2 核 | 4 核+ |
| 内存 | 4 GB | 8 GB+ |
| 磁盘 | 50 GB SSD | 100 GB SSD |
| 网络 | 10 Mbps | 100 Mbps+ |

### 依赖服务

- **PostgreSQL** 14+
- **Redis** 6+
- **Nginx** (可选，用于反向代理)

## 部署方式

### 1. 二进制部署

#### 编译

```bash
# 克隆代码
git clone <repository-url>
cd grove

# 安装依赖
go mod tidy

# 编译（Linux AMD64）
GOOS=linux GOARCH=amd64 go build -o grove ./app/console/main.go

# 或编译所有服务
make build
```

#### 配置文件

```bash
# 创建配置目录
mkdir -p /etc/grove

# 复制配置
cp config.example.yaml /etc/grove/config.yaml

# 编辑配置
vim /etc/grove/config.yaml
```

生产环境配置示例：

```yaml
app:
  name: grove
  env: production

server:
  shutdown_timeout: 30
  read_timeout: 30
  write_timeout: 30

log:
  level: info
  path: /var/log/grove
  console: false

databases:
  default:
    enabled: true
    driver: postgres
    host: ${DB_HOST}
    port: ${DB_PORT}
    user: ${DB_USER}
    password: ${DB_PASSWORD}
    dbname: ${DB_NAME}
    max_connections: 50
    max_idle_conns: 20
    conn_max_lifetime: 3600

redis:
  enabled: true
  addr: ${REDIS_ADDR}
  password: ${REDIS_PASSWORD}

jwt:
  secret: ${JWT_SECRET}
  issuer: grove
  access_expiry_hours: 24
  refresh_expiry_hours: 168

casbin:
  enforcers:
    console:
      enabled: true
      database: default
      mode: rbac

cache:
  default: redis

event:
  async: true
  queue_size: 10000
  workers: 20

scheduler:
  enabled: true
  timezone: Asia/Shanghai
```

#### Systemd 服务

创建服务文件 `/etc/systemd/system/grove.service`：

```ini
[Unit]
Description=Grove Application
After=network.target

[Service]
Type=simple
User=grove
Group=grove
WorkingDirectory=/opt/grove

# 环境变量
Environment="APP_ENV=production"
Environment="JWT_SECRET=your-secret-key"
Environment="DB_HOST=localhost"
Environment="DB_PORT=5432"
Environment="DB_USER=golang_web"
Environment="DB_PASSWORD=your-db-password"
Environment="DB_NAME=golang_web"
Environment="REDIS_ADDR=localhost:6379"

# 启动命令
ExecStart=/opt/grove/grove

# 重启策略
Restart=always
RestartSec=5

# 资源限制
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
# 创建用户
useradd -r -s /bin/false grove

# 创建目录
mkdir -p /opt/grove
mkdir -p /var/log/grove
chown -R grove:grove /opt/grove
chown -R grove:grove /var/log/grove

# 复制二进制
cp grove /opt/grove/
cp -r config.yaml /opt/grove/

# 加载配置
systemctl daemon-reload

# 启动服务
systemctl enable grove
systemctl start grove

# 查看状态
systemctl status grove

# 查看日志
journalctl -u grove -f
```

### 2. Docker 部署

#### Dockerfile

```dockerfile
# 构建阶段
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 安装依赖
RUN apk add --no-cache git

# 复制依赖文件
copy go.mod go.sum ./
RUN go mod download

# 复制源码
COPY . .

# 编译
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./app/console/main.go

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# 复制二进制
COPY --from=builder /app/main .

# 复制配置
COPY config.yaml .

# 暴露端口
EXPOSE 8081

# 运行
CMD ["./main"]
```

#### Docker Compose

```yaml
version: '3.8'

services:
  app:
    build: .
    ports:
      - "8081:8081"
    environment:
      - APP_ENV=production
      - JWT_SECRET=${JWT_SECRET}
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_USER=golang_web
      - DB_PASSWORD=${DB_PASSWORD}
      - DB_NAME=golang_web
      - REDIS_ADDR=redis:6379
    depends_on:
      - postgres
      - redis
    networks:
      - grove
    restart: unless-stopped

  postgres:
    image: postgres:14-alpine
    environment:
      - POSTGRES_USER=golang_web
      - POSTGRES_PASSWORD=${DB_PASSWORD}
      - POSTGRES_DB=golang_web
    volumes:
      - postgres_data:/var/lib/postgresql/data
    networks:
      - grove
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data
    networks:
      - grove
    restart: unless-stopped

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
      - ./ssl:/etc/nginx/ssl
    depends_on:
      - app
    networks:
      - grove
    restart: unless-stopped

volumes:
  postgres_data:
  redis_data:

networks:
  grove:
    driver: bridge
```

部署：

```bash
# 创建环境变量文件
cat > .env << EOF
JWT_SECRET=your-secret-key
DB_PASSWORD=your-db-password
EOF

# 启动
docker-compose up -d

# 查看日志
docker-compose logs -f app

# 停止
docker-compose down
```

### 3. Kubernetes 部署

#### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: grove
  labels:
    app: grove
spec:
  replicas: 3
  selector:
    matchLabels:
      app: grove
  template:
    metadata:
      labels:
        app: grove
    spec:
      containers:
        - name: grove
          image: your-registry/grove:latest
          ports:
            - containerPort: 8081
          env:
            - name: APP_ENV
              value: "production"
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: grove-secrets
                  key: jwt-secret
            - name: DB_HOST
              valueFrom:
                configMapKeyRef:
                  name: grove-config
                  key: db-host
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: grove-secrets
                  key: db-password
          resources:
            requests:
              memory: "256Mi"
              cpu: "250m"
            limits:
              memory: "512Mi"
              cpu: "500m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8081
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /ready
              port: 8081
            initialDelaySeconds: 5
            periodSeconds: 5
```

#### Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: grove
spec:
  selector:
    app: grove
  ports:
    - port: 80
      targetPort: 8081
  type: ClusterIP
```

#### Ingress

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: grove
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
    cert-manager.io/cluster-issuer: "letsencrypt"
spec:
  tls:
    - hosts:
        - api.example.com
      secretName: grove-tls
  rules:
    - host: api.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: grove
                port:
                  number: 80
```

部署：

```bash
# 应用配置
kubectl apply -f k8s/

# 查看状态
kubectl get pods -l app=grove

# 查看日志
kubectl logs -l app=grove -f

# 扩缩容
kubectl scale deployment grove --replicas=5
```

## 反向代理配置

### Nginx

```nginx
upstream golang_web {
    server 127.0.0.1:8081;
    keepalive 32;
}

server {
    listen 80;
    server_name api.example.com;
    
    # 重定向到 HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name api.example.com;
    
    # SSL 配置
    ssl_certificate /etc/nginx/ssl/cert.pem;
    ssl_certificate_key /etc/nginx/ssl/key.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    
    # 日志
    access_log /var/log/nginx/grove-access.log;
    error_log /var/log/nginx/grove-error.log;
    
    # 静态文件
    location /storage/ {
        alias /opt/grove/storage/;
        expires 30d;
    }
    
    # API 代理
    location / {
        proxy_pass http://golang_web;
        proxy_http_version 1.1;
        
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        proxy_connect_timeout 30s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;
        
        proxy_buffering off;
        proxy_request_buffering off;
    }
    
    # 健康检查
    location /health {
        proxy_pass http://golang_web/health;
        access_log off;
    }
}
```

### Caddy

```
api.example.com {
    reverse_proxy localhost:8081
    
    tls your-email@example.com
    
    log {
        output file /var/log/caddy/access.log
    }
}
```

## 数据库迁移

### 生产环境迁移

```bash
# 备份数据库
pg_dump -h localhost -U golang_web golang_web > backup_$(date +%Y%m%d).sql

# 运行迁移
go run ./cmd/artisan/main.go migrate up

# 回滚（如有问题）
go run ./cmd/artisan/main.go migrate down
```

### 零停机部署

```bash
# 1. 部署新版本（不启动）
cp grove-new /opt/grove/grove-new

# 2. 运行迁移
go run ./cmd/artisan/main.go migrate up

# 3. 热切换
systemctl reload grove

# 4. 验证
curl http://localhost:8081/health
```

## 监控告警

### Prometheus 指标

```go
// 暴露指标端点
r.GET("/metrics", gin.WrapH(promhttp.Handler()))
```

### 健康检查

```go
// 健康检查端点
r.GET("/health", func(c *gin.Context) {
    // 检查数据库
    if err := db.DB().Ping(); err != nil {
        c.JSON(503, gin.H{"status": "unhealthy", "error": err.Error()})
        return
    }
    
    // 检查 Redis
    if err := redisClient.Ping(ctx).Err(); err != nil {
        c.JSON(503, gin.H{"status": "unhealthy", "error": err.Error()})
        return
    }
    
    c.JSON(200, gin.H{"status": "healthy"})
})
```

## 日志管理

### 日志轮转

```bash
# 安装 logrotate
apt-get install logrotate

# 配置 /etc/logrotate.d/grove
/var/log/grove/*.log {
    daily
    rotate 30
    compress
    delaycompress
    missingok
    notifempty
    create 0644 grove grove
    sharedscripts
    postrotate
        /bin/kill -HUP $(cat /var/run/syslogd.pid 2> /dev/null) 2> /dev/null || true
    endscript
}
```

### 集中式日志

```yaml
# 使用 Filebeat 发送到 ELK
filebeat.inputs:
  - type: log
    enabled: true
    paths:
      - /var/log/grove/*.log
    fields:
      service: grove
    fields_under_root: true

output.elasticsearch:
  hosts: ["localhost:9200"]
```

## 安全建议

### 1. 防火墙配置

```bash
# 只允许必要端口
ufw default deny incoming
ufw default allow outgoing
ufw allow 22/tcp    # SSH
ufw allow 80/tcp    # HTTP
ufw allow 443/tcp   # HTTPS
ufw enable
```

### 2. 文件权限

```bash
# 设置权限
chown -R grove:grove /opt/grove
chmod 750 /opt/grove
chmod 640 /opt/grove/config.yaml
```

### 3. 敏感信息

```bash
# 使用环境变量或密钥管理服务
# 不要提交到代码仓库

# 检查配置文件
grep -r "password" config/
grep -r "secret" config/
grep -r "key" config/
```

## 性能优化

### 1. 数据库连接池

```yaml
databases:
  default:
    max_connections: 50      # 根据 CPU 核心数调整
    max_idle_conns: 20
    conn_max_lifetime: 3600
```

### 2. 系统参数

```bash
# /etc/sysctl.conf
net.core.somaxconn = 65535
net.ipv4.tcp_max_syn_backlog = 65535
net.ipv4.ip_local_port_range = 1024 65535

# /etc/security/limits.conf
* soft nofile 65535
* hard nofile 65535
```

### 3. Golang 调优

```bash
# 设置 GOMAXPROCS
export GOMAXPROCS=$(nproc)

# 设置 GC 目标
export GOGC=100
```

## 常见问题

### Q: 服务启动失败？

检查：
1. 配置文件是否正确
2. 数据库连接是否正常
3. 端口是否被占用
4. 日志输出

```bash
journalctl -u grove -n 100 --no-pager
```

### Q: 如何更新配置？

```bash
# 修改配置
vim /etc/grove/config.yaml

# 重启服务
systemctl restart grove

# 或热重载（如果支持）
systemctl reload grove
```

### Q: 如何查看性能指标？

```bash
# 查看资源使用
top -p $(pgrep grove)

# 查看连接数
ss -tuln | grep 8081
netstat -an | grep 8081 | wc -l

# 查看数据库连接
psql -U golang_web -c "SELECT count(*) FROM pg_stat_activity;"
```
