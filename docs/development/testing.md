# 测试指南

本文档介绍 grove 框架的测试策略和实践。

## 测试类型

```
tests/
├── unit/           # 单元测试
├── integration/    # 集成测试
├── e2e/            # 端到端测试
└── benchmark/      # 性能测试
```

## 单元测试

### 测试文件命名

```
article.go          # 源文件
article_test.go     # 测试文件
```

### 基础测试

```go
package service

import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestArticleService_Create(t *testing.T) {
    // 准备
    mockDB := new(mock.Database)
    mockCache := new(mock.Cache)
    
    provider := &provider.Provider{
        DB: mockDB,
        Cache: mockCache,
    }
    
    service := NewArticleService(provider)
    
    // 设置 mock 期望
    mockDB.On("Create", mock.Anything).Return(nil)
    
    // 执行
    req := CreateRequest{
        Title:   "Test Article",
        Content: "Test Content",
        UserID:  1,
    }
    
    article, err := service.Create(context.Background(), req)
    
    // 验证
    assert.NoError(t, err)
    assert.NotNil(t, article)
    assert.Equal(t, "Test Article", article.Title)
    
    mockDB.AssertExpectations(t)
}
```

### 表驱动测试

```go
func TestArticleService_Create_Validation(t *testing.T) {
    tests := []struct {
        name    string
        req     CreateRequest
        wantErr bool
        errMsg  string
    }{
        {
            name: "valid request",
            req: CreateRequest{
                Title:   "Valid Title",
                Content: "Valid Content",
                UserID:  1,
            },
            wantErr: false,
        },
        {
            name: "empty title",
            req: CreateRequest{
                Title:   "",
                Content: "Content",
                UserID:  1,
            },
            wantErr: true,
            errMsg:  "标题不能为空",
        },
        {
            name: "title too long",
            req: CreateRequest{
                Title:   strings.Repeat("a", 256),
                Content: "Content",
                UserID:  1,
            },
            wantErr: true,
            errMsg:  "标题长度不能超过255",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            service := NewArticleService(mockProvider())
            
            _, err := service.Create(context.Background(), tt.req)
            
            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Mock 使用

```go
// 定义 mock
type MockDatabase struct {
    mock.Mock
}

func (m *MockDatabase) Create(value interface{}) *gorm.DB {
    args := m.Called(value)
    return args.Get(0).(*gorm.DB)
}

func (m *MockDatabase) First(dest interface{}, conds ...interface{}) *gorm.DB {
    args := m.Called(dest, conds)
    return args.Get(0).(*gorm.DB)
}

// 使用 mock
func TestService_WithMock(t *testing.T) {
    mockDB := new(MockDatabase)
    
    // 设置返回值
    mockDB.On("First", mock.Anything, mock.Anything).
        Return(&gorm.DB{Error: gorm.ErrRecordNotFound})
    
    // 执行测试
    // ...
    
    // 验证调用
    mockDB.AssertCalled(t, "First", mock.Anything, uint(1))
    mockDB.AssertNumberOfCalls(t, "First", 1)
}
```

## 集成测试

### 数据库测试

```go
func TestArticleService_Integration(t *testing.T) {
    // 使用测试数据库
    testDB := setupTestDB(t)
    defer teardownTestDB(t, testDB)
    
    provider := &provider.Provider{
        DB: database.NewRepo(testDB),
    }
    
    service := NewArticleService(provider)
    
    ctx := context.Background()
    
    // 测试创建
    article, err := service.Create(ctx, CreateRequest{
        Title:   "Test",
        Content: "Content",
        UserID:  1,
    })
    require.NoError(t, err)
    assert.NotZero(t, article.ID)
    
    // 测试查询
    found, err := service.Get(ctx, article.ID)
    require.NoError(t, err)
    assert.Equal(t, article.Title, found.Title)
    
    // 测试更新
    updated, err := service.Update(ctx, UpdateRequest{
        ID:      article.ID,
        Title:   "Updated",
        Content: "Updated Content",
    })
    require.NoError(t, err)
    assert.Equal(t, "Updated", updated.Title)
    
    // 测试删除
    err = service.Delete(ctx, article.ID)
    require.NoError(t, err)
    
    _, err = service.Get(ctx, article.ID)
    assert.Error(t, err)
}

func setupTestDB(t *testing.T) *gorm.DB {
    // 使用 SQLite 内存数据库或测试 PostgreSQL
    db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
    require.NoError(t, err)
    
    // 自动迁移
    err = db.AutoMigrate(&model.Article{}, &model.User{})
    require.NoError(t, err)
    
    return db
}

func teardownTestDB(t *testing.T, db *gorm.DB) {
    sqlDB, err := db.DB()
    require.NoError(t, err)
    sqlDB.Close()
}
```

### HTTP 测试

```go
func TestArticleHandler_Create(t *testing.T) {
    // 设置 Gin 测试模式
gin.SetMode(gin.TestMode)
    
    // 创建路由
    r := gin.New()
    
    // 创建 mock provider
    mockProvider := new(mock.Provider)
    mockService := new(mock.ArticleService)
    mockProvider.On("ArticleService").Return(mockService)
    
    handler := NewArticleHandler(mockProvider)
    r.POST("/articles", handler.Create)
    
    // 构建请求
    body := `{"title":"Test","content":"Content"}`
    req := httptest.NewRequest("POST", "/articles", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    
    // 执行请求
    w := httptest.NewRecorder()
    r.ServeHTTP(w, req)
    
    // 验证响应
    assert.Equal(t, 200, w.Code)
    
    var response map[string]interface{}
    json.Unmarshal(w.Body.Bytes(), &response)
    
    assert.Equal(t, float64(0), response["code"])
    assert.NotNil(t, response["data"])
}
```

## 端到端测试

```go
func TestE2E_ArticleFlow(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping e2e test")
    }
    
    // 启动完整服务
    server := setupServer(t)
    defer server.Close()
    
    baseURL := server.URL
    
    // 1. 登录获取 token
    token := login(t, baseURL, "admin", "password")
    
    // 2. 创建文章
    article := createArticle(t, baseURL, token, CreateRequest{
        Title:   "E2E Test",
        Content: "E2E Content",
    })
    
    // 3. 获取文章
    fetched := getArticle(t, baseURL, token, article.ID)
    assert.Equal(t, article.Title, fetched.Title)
    
    // 4. 更新文章
    updated := updateArticle(t, baseURL, token, article.ID, UpdateRequest{
        Title:   "Updated",
        Content: "Updated Content",
    })
    assert.Equal(t, "Updated", updated.Title)
    
    // 5. 删除文章
    deleteArticle(t, baseURL, token, article.ID)
    
    // 6. 验证删除
    _, err := getArticle(t, baseURL, token, article.ID)
    assert.Error(t, err)
}

func createArticle(t *testing.T, baseURL, token string, req CreateRequest) *Article {
    body, _ := json.Marshal(req)
    
    httpReq, _ := http.NewRequest("POST", baseURL+"/api/v1/articles", bytes.NewReader(body))
    httpReq.Header.Set("Authorization", "Bearer "+token)
    httpReq.Header.Set("Content-Type", "application/json")
    
    resp, err := http.DefaultClient.Do(httpReq)
    require.NoError(t, err)
    defer resp.Body.Close()
    
    assert.Equal(t, 200, resp.StatusCode)
    
    var result struct {
        Code int      `json:"code"`
        Data Article  `json:"data"`
    }
    json.NewDecoder(resp.Body).Decode(&result)
    
    return &result.Data
}
```

## 性能测试

### 基准测试

```go
func BenchmarkArticleService_Create(b *testing.B) {
    service := setupBenchmarkService(b)
    ctx := context.Background()
    req := CreateRequest{
        Title:   "Benchmark",
        Content: "Content",
        UserID:  1,
    }
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _, err := service.Create(ctx, req)
            if err != nil {
                b.Fatal(err)
            }
        }
    })
}

func BenchmarkArticleService_List(b *testing.B) {
    service := setupBenchmarkService(b)
    ctx := context.Background()
    
    // 准备数据
    for i := 0; i < 1000; i++ {
        service.Create(ctx, CreateRequest{
            Title:   fmt.Sprintf("Article %d", i),
            Content: "Content",
            UserID:  1,
        })
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _, err := service.List(ctx, ListRequest{Page: 1, PerPage: 20})
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

### 负载测试

```go
func TestLoad_ArticleAPI(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping load test")
    }
    
    server := setupServer(t)
    defer server.Close()
    
    // 使用 vegeta 进行负载测试
    rate := vegeta.Rate{Freq: 100, Per: time.Second}
    duration := 30 * time.Second
    
    targeter := vegeta.NewStaticTargeter(vegeta.Target{
        Method: "GET",
        URL:    server.URL + "/api/v1/articles",
    })
    
    attacker := vegeta.NewAttacker()
    
    var metrics vegeta.Metrics
    for res := range attacker.Attack(targeter, rate, duration, "Load Test") {
        metrics.Add(res)
    }
    metrics.Close()
    
    // 验证结果
    assert.Greater(t, metrics.Success*100, 99.0)  // 成功率 > 99%
    assert.Less(t, metrics.Latencies.P99, 100*time.Millisecond)  // P99 < 100ms
}
```

## 测试工具

### testify

```go
import "github.com/stretchr/testify/assert"

// 相等
assert.Equal(t, expected, actual)
assert.NotEqual(t, unexpected, actual)

// 空值
assert.Nil(t, obj)
assert.NotNil(t, obj)
assert.Empty(t, slice)
assert.NotEmpty(t, slice)

// 错误
assert.NoError(t, err)
assert.Error(t, err)
assert.EqualError(t, err, "expected error")

// 包含
assert.Contains(t, "hello world", "world")
assert.Contains(t, []int{1, 2, 3}, 2)
```

### gomock

```go
import "github.com/golang/mock/gomock"

ctrl := gomock.NewController(t)
defer ctrl.Finish()

mockDB := mock.NewMockDatabase(ctrl)
mockDB.EXPECT().Create(gomock.Any()).Return(nil).Times(1)
```

### httpexpect

```go
import "github.com/gavv/httpexpect/v2"

e := httpexpect.New(t, server.URL)

e.POST("/articles").
    WithJSON(map[string]string{
        "title": "Test",
        "content": "Content",
    }).
    Expect().
    Status(200).
    JSON().
    Object().
    Value("code").Number().Equal(0)
```

## 测试覆盖率

### 生成覆盖率报告

```bash
# 运行测试并生成覆盖率
go test -coverprofile=coverage.out ./...

# 查看覆盖率
go tool cover -func=coverage.out

# 生成 HTML 报告
go tool cover -html=coverage.out -o coverage.html
```

### 排除文件

```bash
# 使用 -coverpkg 指定包
go test -coverpkg=./pkg/...,./internal/... -coverprofile=coverage.out ./...

# 在代码中排除
//go:build test
// +build test
```

## 最佳实践

### 1. 测试命名

```go
// ✅ 描述性命名
func TestArticleService_Create_Success(t *testing.T)
func TestArticleService_Create_EmptyTitle(t *testing.T)
func TestArticleService_Create_DuplicateTitle(t *testing.T)

// ❌ 避免模糊命名
func Test1(t *testing.T)
func TestCreate(t *testing.T)
```

### 2. 测试独立性

```go
// ✅ 每个测试独立
func TestService_Create(t *testing.T) {
    db := setupTestDB(t)
    defer teardownTestDB(t, db)
    
    // 测试逻辑
}

// ❌ 不要共享状态
var globalDB *gorm.DB  // 不好

func Test1(t *testing.T) {
    globalDB.Create(...)  // 影响其他测试
}
```

### 3. 测试数据

```go
// ✅ 使用工厂函数
func createTestArticle(t *testing.T, db *gorm.DB) *model.Article {
    article := &model.Article{
        Title:   "Test",
        Content: "Content",
    }
    require.NoError(t, db.Create(article).Error)
    return article
}

// ✅ 使用 fixtures
func loadFixtures(t *testing.T, db *gorm.DB) {
    fixtures, err := testfixtures.New(
        testfixtures.Database(db),
        testfixtures.Dialect("postgres"),
        testfixtures.Directory("testdata/fixtures"),
    )
    require.NoError(t, err)
    require.NoError(t, fixtures.Load())
}
```

### 4. 并行测试

```go
// ✅ 并行执行
func TestService_Create(t *testing.T) {
    t.Parallel()
    
    // 每个并行测试使用独立的数据库
    db := setupTestDB(t)
    
    // 测试逻辑
}

// 控制并行数量
t.SetParallelism(4)
```

### 5. 跳过测试

```go
// 短模式跳过
func TestSlow(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping slow test")
    }
    // 慢测试
}

// 条件跳过
func TestIntegration(t *testing.T) {
    if os.Getenv("INTEGRATION") != "true" {
        t.Skip("set INTEGRATION=true to run")
    }
}
```

## CI/CD 集成

### GitHub Actions

```yaml
name: Test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    
    services:
      postgres:
        image: postgres:14
        env:
          POSTGRES_USER: test
          POSTGRES_PASSWORD: test
          POSTGRES_DB: test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5432:5432
      
      redis:
        image: redis:7
        ports:
          - 6379:6379
    
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out ./...
        env:
          DB_HOST: localhost
          DB_PORT: 5432
          REDIS_ADDR: localhost:6379
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out
```

## 常见问题

### Q: 测试数据库太慢？

```go
// 使用 SQLite 内存数据库
sqlDB, err := sql.Open("sqlite3", ":memory:")

// 或使用测试容器
postgresC, err := postgres.RunContainer(ctx,
    testcontainers.WithImage("docker.io/postgres:14-alpine"),
)
```

### Q: 如何处理时间相关测试？

```go
// 优先把时间来源做成可注入函数或独立 helper
type Service struct {
    now func() time.Time
}

func TestWithFixedTime(t *testing.T) {
    fixedTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
    service := &Service{
        now: func() time.Time { return fixedTime },
    }
    _ = service
}
```

### Q: 如何测试随机数？

```go
// 注入随机数生成器
type Service struct {
    randFunc func() int64
}

func (s *Service) GenerateID() int64 {
    return s.randFunc()
}

// 测试时使用固定值
service := &Service{
    randFunc: func() int64 { return 12345 },
}
```
