# 缓存系统

本文档介绍 grove 框架的缓存系统，支持 Memory 和 Redis 两种存储驱动。

## 特性

- 🚀 **多驱动支持** - Memory、Redis 无缝切换
- 💾 **自动序列化** - 支持任意类型存储
- ⏰ **TTL 支持** - 自动过期清理
- 🔄 **Remember 模式** - 缓存不存在时自动获取
- 📈 **原子操作** - Increment/Decrement 支持
- 🧹 **自动清理** - Memory 驱动后台清理过期数据

## 配置

```yaml
cache:
  default: memory           # 默认缓存存储
  prefix: app              # 键前缀
  
  # Memory 配置
  memory:
    driver: memory
    cleanup_interval: 60   # 清理间隔（秒）
  
  # Redis 配置
  redis:
    driver: redis
    prefix: cache          # 额外前缀
```

## 快速开始

### 获取默认缓存

```go
// 通过 Provider 获取
store := provider.Cache.Default()

// 获取指定存储
redisStore := provider.Cache.GetStore("redis")
memoryStore := provider.Cache.GetStore("memory")
```

### 基础操作

```go
ctx := context.Background()

// 存储
err := store.Put(ctx, "key", "value", 3600)  // 3600 秒过期

// 获取
value, err := store.Get(ctx, "key")

// 检查存在
exists, err := store.Has(ctx, "key")

// 删除
err := store.Forget(ctx, "key")

// 清空
err := store.Flush(ctx)
```

## 详细用法

### 存储数据

```go
// 字符串
store.Put(ctx, "name", "John", 3600)

// 数字
store.Put(ctx, "count", 100, 3600)

// 结构体
user := User{ID: 1, Name: "John"}
store.Put(ctx, "user:1", user, 3600)

// 切片
ids := []int{1, 2, 3}
store.Put(ctx, "ids", ids, 3600)

// Map
data := map[string]interface{}{"a": 1, "b": 2}
store.Put(ctx, "data", data, 3600)

// 永久存储（TTL = 0）
store.Put(ctx, "config", config, 0)
```

### 获取数据

```go
// 获取字符串
val, _ := store.Get(ctx, "name")
name := val.(string)

// 获取数字
val, _ := store.Get(ctx, "count")
count := val.(int)

// 获取结构体
val, _ := store.Get(ctx, "user:1")
user := val.(User)

// 获取切片
val, _ := store.Get(ctx, "ids")
ids := val.([]int)

// 类型安全获取（使用 Remember）
user, err := store.Remember(ctx, "user:1", 3600, func() (any, error) {
    return db.First(&User{}, 1)
})
```

### Remember 模式

```go
// 缓存不存在时自动获取
result, err := store.Remember(ctx, "articles:popular", 3600, func() (any, error) {
    // 从数据库获取
    var articles []Article
    err := db.Order("view_count DESC").Limit(10).Find(&articles).Error
    return articles, err
})

// 类型转换
articles := result.([]Article)

// 永久缓存
result, err := store.RememberForever(ctx, "config", func() (any, error) {
    return loadConfig()
})
```

### 原子操作

```go
// 自增
newVal, err := store.Increment(ctx, "counter")      // +1
newVal, err := store.Increment(ctx, "counter", 5)   // +5

// 自减
newVal, err := store.Decrement(ctx, "counter")      // -1
newVal, err := store.Decrement(ctx, "counter", 5)   // -5

// 注意：自增/自减只支持数字类型
// 如果 key 不存在，会从 0 开始
```

### 批量操作

```go
// 批量获取
keys := []string{"key1", "key2", "key3"}
values, err := store.Many(ctx, keys)
// values["key1"], values["key2"], values["key3"]

// 批量存储
items := map[string]any{
    "key1": "value1",
    "key2": "value2",
    "key3": "value3",
}
err := store.PutMany(ctx, items, 3600)

// 批量删除
err := store.ForgetMany(ctx, []string{"key1", "key2"})
```

## 在 Service 中使用

### 基础缓存

```go
type ArticleService struct {
    provider *provider.Provider
}

func (s *ArticleService) Get(ctx context.Context, id uint) (*model.Article, error) {
    cacheKey := fmt.Sprintf("article:%d", id)
    
    // 尝试从缓存获取
    result, err := s.provider.Cache.Default().Remember(ctx, cacheKey, 3600, func() (any, error) {
        article := &model.Article{}
        if err := s.provider.DB.Default().First(article, id).Error; err != nil {
            return nil, err
        }
        return article, nil
    })
    
    if err != nil {
        return nil, err
    }
    
    return result.(*model.Article), nil
}
```

### 缓存更新策略

```go
func (s *ArticleService) Update(ctx context.Context, req UpdateRequest) (*model.Article, error) {
    // 1. 更新数据库
    article := &model.Article{}
    if err := s.provider.DB.Default().First(article, req.ID).Error; err != nil {
        return nil, err
    }
    
    article.Title = req.Title
    article.Content = req.Content
    
    if err := s.provider.DB.Default().Save(article).Error; err != nil {
        return nil, err
    }
    
    // 2. 删除缓存（而不是更新，避免并发问题）
    cacheKey := fmt.Sprintf("article:%d", req.ID)
    s.provider.Cache.Default().Forget(ctx, cacheKey)
    
    // 3. 删除列表缓存（通配符删除）
    s.provider.Cache.Default().Forget(ctx, "articles:list:*")
    
    return article, nil
}
```

### 缓存预热

```go
func (s *ArticleService) WarmupCache(ctx context.Context) error {
    // 获取热门文章
    var articles []model.Article
    if err := s.provider.DB.Default().
        Where("status = ?", 1).
        Order("view_count DESC").
        Limit(100).
        Find(&articles).Error; err != nil {
        return err
    }
    
    // 预热缓存
    for _, article := range articles {
        cacheKey := fmt.Sprintf("article:%d", article.ID)
        s.provider.Cache.Default().Put(ctx, cacheKey, article, 3600)
    }
    
    return nil
}
```

## 高级用法

### 自定义序列化

```go
// 默认使用 JSON 序列化
// 可以自定义序列化器

type CacheStore interface {
    // ...
    Serialize(value any) ([]byte, error)
    Deserialize(data []byte, target any) error
}

// 使用 MessagePack 序列化
import "github.com/vmihailenco/msgpack/v5"

func (s *Store) Serialize(value any) ([]byte, error) {
    return msgpack.Marshal(value)
}

func (s *Store) Deserialize(data []byte, target any) error {
    return msgpack.Unmarshal(data, target)
}
```

### 缓存标签（Tagging）

```go
// 给缓存打标签，方便批量删除
func (s *ArticleService) Create(ctx context.Context, req CreateRequest) (*model.Article, error) {
    article := &model.Article{...}
    
    // 保存文章
    if err := s.provider.DB.Default().Create(article).Error; err != nil {
        return nil, err
    }
    
    // 缓存并打标签
    cacheKey := fmt.Sprintf("article:%d", article.ID)
    s.provider.Cache.Default().Put(ctx, cacheKey, article, 3600)
    s.provider.Cache.Default().Tag(ctx, cacheKey, []string{"article", fmt.Sprintf("user:%d", article.UserID)})
    
    return article, nil
}

// 按标签删除
func (s *ArticleService) DeleteByUser(ctx context.Context, userID uint) error {
    // 删除该用户的所有文章缓存
    tag := fmt.Sprintf("user:%d", userID)
    s.provider.Cache.Default().FlushByTag(ctx, tag)
    
    // ... 删除数据库
}
```

### 缓存统计

```go
// 获取缓存统计
type Stats struct {
    Hits       int64
    Misses     int64
    HitRate    float64
    KeysCount  int64
    MemorySize int64
}

stats := store.Stats()
fmt.Printf("Hit rate: %.2f%%\n", stats.HitRate*100)
fmt.Printf("Keys: %d\n", stats.KeysCount)
```

## 驱动对比

| 特性 | Memory | Redis |
|------|--------|-------|
| 持久化 | ❌ | ✅ |
| 分布式 | ❌ | ✅ |
| 内存占用 | 应用内存 | Redis 内存 |
| 性能 | 极高 | 高 |
| 适用场景 | 单机、临时缓存 | 分布式、持久化缓存 |
| 过期清理 | 后台 goroutine | Redis 自动 |

## 最佳实践

### 1. 键命名规范

```go
// ✅ 使用冒号分隔，带前缀
"app:user:123"
"app:articles:list:page:1"
"app:config:database"

// ❌ 避免模糊命名
"user123"
"articles"
"config"
```

### 2. TTL 设置

```go
// ✅ 根据数据变化频率设置 TTL
// 很少变化的数据
store.Put(ctx, "config", config, 86400)  // 1天

// 经常变化的数据
store.Put(ctx, "article:123", article, 300)  // 5分钟

// 热点数据
store.Put(ctx, "popular:articles", articles, 60)  // 1分钟

// ❌ 不要所有数据都用相同 TTL
```

### 3. 缓存穿透防护

```go
func (s *ArticleService) Get(ctx context.Context, id uint) (*model.Article, error) {
    cacheKey := fmt.Sprintf("article:%d", id)
    
    // 使用 Remember，自动处理缓存穿透
    result, err := s.provider.Cache.Default().Remember(ctx, cacheKey, 3600, func() (any, error) {
        article := &model.Article{}
        if err := s.provider.DB.Default().First(article, id).Error; err != nil {
            // 数据库也不存在，缓存空值（短期）
            if errors.Is(err, gorm.ErrRecordNotFound) {
                return nil, errors.NewNotFound(fmt.Sprintf("article %d", id))
            }
            return nil, err
        }
        return article, nil
    })
    
    if err != nil {
        return nil, err
    }
    
    return result.(*model.Article), nil
}
```

### 4. 缓存雪崩防护

```go
// 随机 TTL，避免同时过期
func randomTTL(baseTTL int) int {
    return baseTTL + rand.Intn(300)  // 基础 TTL + 0-5分钟随机
}

store.Put(ctx, key, value, randomTTL(3600))
```

### 5. 大对象处理

```go
// ✅ 分页缓存大列表
func (s *ArticleService) List(ctx context.Context, page, perPage int) ([]Article, error) {
    cacheKey := fmt.Sprintf("articles:list:page:%d:per_page:%d", page, perPage)
    
    result, err := s.provider.Cache.Default().Remember(ctx, cacheKey, 300, func() (any, error) {
        var articles []Article
        err := s.provider.DB.Default().
            Offset((page - 1) * perPage).
            Limit(perPage).
            Find(&articles).Error
        return articles, err
    })
    
    return result.([]Article), err
}

// ❌ 不要缓存整个大列表
// store.Put(ctx, "all:articles", allArticles, 3600)  // 不好
```

### 6. 错误处理

```go
// ✅ 缓存失败不应该影响主流程
func (s *ArticleService) Get(ctx context.Context, id uint) (*model.Article, error) {
    // 先尝试从缓存获取
    val, err := s.provider.Cache.Default().Get(ctx, cacheKey)
    if err == nil {
        return val.(*model.Article), nil
    }
    
    // 缓存失败，记录日志但继续从数据库获取
    if err != nil && !errors.Is(err, cache.ErrNotFound) {
        logger.Warn().Err(err).Str("key", cacheKey).Msg("缓存读取失败")
    }
    
    // 从数据库获取
    article := &model.Article{}
    if err := s.provider.DB.Default().First(article, id).Error; err != nil {
        return nil, err
    }
    
    // 异步更新缓存
    go func() {
        s.provider.Cache.Default().Put(ctx, cacheKey, article, 3600)
    }()
    
    return article, nil
}
```

## 调试

### 查看缓存内容

```go
// Memory 缓存可以直接查看
memoryStore := provider.Cache.GetStore("memory").(*cache.MemoryStore)
keys := memoryStore.Keys()
for _, key := range keys {
    val, _ := memoryStore.Get(ctx, key)
    fmt.Printf("%s: %v\n", key, val)
}

// Redis 使用命令行
// redis-cli KEYS "app:*"
// redis-cli GET "app:user:1"
```

### 日志记录

```go
// 开启缓存操作日志
logger.Debug().
    Str("action", "cache_get").
    Str("key", key).
    Bool("hit", err == nil).
    Msg("缓存操作")
```

## 常见问题

### Q: Memory 和 Redis 如何选择？

- **Memory**: 单机部署、临时缓存、开发环境
- **Redis**: 分布式部署、需要持久化、生产环境推荐

### Q: 缓存不生效？

检查：
1. 缓存配置是否正确
2. TTL 是否设置
3. 键名是否正确
4. 序列化是否成功

### Q: 如何清空所有缓存？

```go
// 清空默认存储
provider.Cache.Default().Flush(ctx)

// 清空指定存储
provider.Cache.GetStore("redis").Flush(ctx)
```

### Q: 缓存数据不一致？

解决方案：
1. 使用 Cache-Aside 模式（先删缓存再更新数据库）
2. 设置合理的 TTL
3. 使用消息队列同步缓存更新
