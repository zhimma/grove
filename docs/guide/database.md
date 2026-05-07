# 数据库与模型

本文档介绍 grove 框架的数据库操作和模型定义，基于 GORM 封装。

## 数据库配置

### 多数据源支持

框架支持多个数据库连接：

```yaml
databases:
  default:
    enabled: true
    driver: postgres
    host: 127.0.0.1
    port: 5432
    user: postgres
    password: secret
    dbname: golang_web
    
  orders:
    enabled: true
    driver: postgres
    host: orders-db.internal
    port: 5432
    user: postgres
    password: secret
    dbname: orders_db
```

### 使用数据库

```go
// 使用默认连接
db := provider.DB.Default()

// 使用指定连接
orderDB := provider.DB.Get("orders")
```

## 模型定义

### 基础模型

```go
package model

import (
    "time"
    "gorm.io/gorm"
)

// Article 文章模型
type Article struct {
    ID          uint           `gorm:"primarykey" json:"id"`
    Title       string         `gorm:"size:255;not null;index" json:"title"`
    Content     string         `gorm:"type:text" json:"content"`
    Status      int            `gorm:"default:1;index" json:"status"` // 1: 正常, 0: 禁用
    UserID      uint           `gorm:"not null;index" json:"user_id"`
    ViewCount   int64          `gorm:"default:0" json:"view_count"`
    CreatedAt   time.Time      `json:"created_at"`
    UpdatedAt   time.Time      `json:"updated_at"`
    DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (Article) TableName() string {
    return "articles"
}
```

### 字段标签

```go
type User struct {
    // 主键
    ID uint `gorm:"primarykey"`
    
    // 普通字段
    Name string `gorm:"size:100;not null"`
    
    // 唯一索引
    Email string `gorm:"uniqueIndex;size:255"`
    
    // 普通索引
    Phone string `gorm:"index;size:20"`
    
    // 复合索引
    FirstName string `gorm:"index:idx_name"`
    LastName  string `gorm:"index:idx_name"`
    
    // 默认值
    Status int `gorm:"default:1"`
    
    // 类型指定
    Content string `gorm:"type:text"`
    
    // 忽略字段
    Password string `gorm:"-" json:"-"`
    
    // 嵌入模型
    gorm.Model  // 包含 ID, CreatedAt, UpdatedAt, DeletedAt
}
```

### 关联关系

#### 一对一

```go
type User struct {
    ID      uint
    Profile Profile // 关联 Profile
}

type Profile struct {
    ID     uint
    UserID uint  // 外键
    Bio    string
}

// 预加载
var user User
db.Preload("Profile").First(&user, 1)
```

#### 一对多

```go
type User struct {
    ID       uint
    Articles []Article // 用户有多篇文章
}

type Article struct {
    ID     uint
    UserID uint   // 外键
    Title  string
}

// 预加载
var user User
db.Preload("Articles").First(&user, 1)

// 条件预加载
db.Preload("Articles", "status = ?", 1).First(&user, 1)
```

#### 多对多

```go
type Article struct {
    ID    uint
    Title string
    Tags  []Tag `gorm:"many2many:article_tags;"`
}

type Tag struct {
    ID       uint
    Name     string
    Articles []Article `gorm:"many2many:article_tags;"`
}

// 预加载
db.Preload("Tags").First(&article, 1)

// 关联操作
db.Model(&article).Association("Tags").Append(&tag)
db.Model(&article).Association("Tags").Delete(&tag)
db.Model(&article).Association("Tags").Clear()
```

## CRUD 操作

### 创建

```go
// 单条创建
article := &model.Article{Title: "Hello", Content: "World"}
result := db.Create(article)
// article.ID 会自动填充

// 批量创建
articles := []*model.Article{
    {Title: "A1"},
    {Title: "A2"},
}
db.Create(&articles)

// 指定字段
db.Select("Title", "Content").Create(&article)

// 忽略字段
db.Omit("Content").Create(&article)
```

### 查询

```go
// 主键查询
var article model.Article
db.First(&article, 1)  // SELECT * FROM articles WHERE id = 1

// 条件查询
db.First(&article, "title = ?", "Hello")
db.Where("title = ?", "Hello").First(&article)

// 多条查询
var articles []model.Article
db.Where("status = ?", 1).Find(&articles)

// 排序
db.Order("created_at desc").Find(&articles)
db.Order("created_at desc, id asc").Find(&articles)

// 分页
db.Offset((page - 1) * perPage).Limit(perPage).Find(&articles)

// 选择字段
db.Select("id", "title").Find(&articles)

// 去重
db.Distinct("status").Find(&articles)

// 原生 SQL
db.Raw("SELECT * FROM articles WHERE id > ?", 10).Scan(&articles)
```

### 更新

```go
// 更新所有字段
article.Title = "New Title"
db.Save(&article)

// 更新指定字段
db.Model(&article).Update("title", "New Title")

// 更新多个字段
db.Model(&article).Updates(model.Article{Title: "New", Status: 2})

// 条件更新
db.Model(&model.Article{}).Where("status = ?", 0).Update("status", 1)

// 自增
db.Model(&article).UpdateColumn("view_count", gorm.Expr("view_count + ?", 1))
```

### 删除

```go
// 软删除（需要 DeletedAt 字段）
db.Delete(&article)

// 永久删除
db.Unscoped().Delete(&article)

// 条件删除
db.Where("status = ?", 0).Delete(&model.Article{})

// 批量删除
var ids []uint{1, 2, 3}
db.Where("id IN ?", ids).Delete(&model.Article{})
```

## 高级查询

### 条件构建

```go
// 链式条件
db.Where("status = ?", 1).Where("created_at > ?", lastWeek).Find(&articles)

// Or 条件
db.Where("status = ? OR status = ?", 1, 2).Find(&articles)

// In 条件
db.Where("id IN ?", []uint{1, 2, 3}).Find(&articles)

// Between
db.Where("created_at BETWEEN ? AND ?", start, end).Find(&articles)

// Like
db.Where("title LIKE ?", "%keyword%").Find(&articles)

// 复杂条件
query := db.Where("status = ?", 1)
if keyword != "" {
    query = query.Where("title LIKE ?", "%"+keyword+"%")
}
if userID > 0 {
    query = query.Where("user_id = ?", userID)
}
query.Find(&articles)
```

### 聚合查询

```go
// 计数
var count int64
db.Model(&model.Article{}).Where("status = ?", 1).Count(&count)

// 求和
var totalViews int64
db.Model(&model.Article{}).Select("SUM(view_count)").Scan(&totalViews)

// 分组统计
type StatusCount struct {
    Status int
    Count  int64
}
var results []StatusCount
db.Model(&model.Article{}).Select("status, COUNT(*) as count").Group("status").Scan(&results)
```

### 子查询

```go
// 子查询作为条件
subQuery := db.Model(&model.Article{}).Select("user_id").Where("status = ?", 1)
db.Where("id IN (?)", subQuery).Find(&users)

// 子查询作为字段
var users []struct {
    ID          uint
    ArticleCount int64
}
db.Model(&model.User{}).Select(
    "users.*",
    "(SELECT COUNT(*) FROM articles WHERE articles.user_id = users.id) as article_count",
).Scan(&users)
```

## 事务管理

### 使用 Transaction Manager

```go
func (s *ArticleService) Transfer(ctx context.Context, fromID, toID uint, amount int64) error {
    return s.provider.Transaction.Execute(ctx, func(tx *gorm.DB) error {
        // 扣减
        if err := tx.Model(&model.Account{}).Where("id = ?", fromID).
            UpdateColumn("balance", gorm.Expr("balance - ?", amount)).Error; err != nil {
            return err
        }
        
        // 增加
        if err := tx.Model(&model.Account{}).Where("id = ?", toID).
            UpdateColumn("balance", gorm.Expr("balance + ?", amount)).Error; err != nil {
            return err
        }
        
        return nil
    })
}
```

### 手动事务

```go
tx := db.Begin()

if err := tx.Create(&article).Error; err != nil {
    tx.Rollback()
    return err
}

if err := tx.Create(&tag).Error; err != nil {
    tx.Rollback()
    return err
}

tx.Commit()
```

### 嵌套事务（SavePoint）

```go
func (s *ArticleService) CreateWithTags(ctx context.Context, article *model.Article, tags []string) error {
    return s.provider.Transaction.Execute(ctx, func(tx *gorm.DB) error {
        // 创建文章
        if err := tx.Create(article).Error; err != nil {
            return err
        }
        
        // 创建标签（使用 SavePoint）
        for _, tagName := range tags {
            err := s.provider.Transaction.Execute(ctx, func(tx2 *gorm.DB) error {
                tag := &model.Tag{Name: tagName}
                return tx2.FirstOrCreate(tag, "name = ?", tagName).Error
            })
            if err != nil {
                return err
            }
        }
        
        return nil
    })
}
```

## 数据库迁移

### 创建迁移

```bash
go run ./cmd/artisan/main.go migrate create create_articles_table
```

生成文件：
- `database/migrations/20240101000001_create_articles_table.up.sql`
- `database/migrations/20240101000001_create_articles_table.down.sql`

### 编写迁移

```sql
-- .up.sql
CREATE TABLE IF NOT EXISTS articles (
    id SERIAL PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    content TEXT,
    status INTEGER DEFAULT 1,
    user_id INTEGER NOT NULL,
    view_count BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

CREATE INDEX idx_articles_status ON articles(status);
CREATE INDEX idx_articles_user_id ON articles(user_id);

-- .down.sql
DROP TABLE IF EXISTS articles;
```

### 执行迁移

```bash
# 执行所有未执行的迁移
go run ./cmd/artisan/main.go migrate up

# 回滚最近一次迁移
go run ./cmd/artisan/main.go migrate down

# 查看迁移状态
go run ./cmd/artisan/main.go migrate status
```

## 最佳实践

### 1. 模型定义

```go
// ✅ 使用明确的数据类型
type Article struct {
    ID        uint      `gorm:"primarykey"`
    Title     string    `gorm:"size:255;not null"`
    Content   string    `gorm:"type:text"`
    Status    int       `gorm:"default:1;index"`
    CreatedAt time.Time `gorm:"index"`
}

// ❌ 避免模糊定义
type Article struct {
    ID      uint
    Title   string  // 没有长度限制
    Content string  // 没有类型指定
}
```

### 2. 查询优化

```go
// ✅ 只查询需要的字段
db.Select("id", "title").Find(&articles)

// ✅ 使用索引字段查询
db.Where("status = ?", 1).Find(&articles)  // status 有索引

// ✅ 限制返回数量
db.Limit(100).Find(&articles)

// ❌ 避免全表扫描
db.Where("content LIKE ?", "%keyword%").Find(&articles)  // content 无索引
```

### 3. 批量操作

```go
// ✅ 批量插入
articles := []*model.Article{{Title: "A1"}, {Title: "A2"}}
db.Create(&articles)

// ❌ 避免循环单条插入
for _, a := range articles {
    db.Create(&a)  // N 次查询
}
```

### 4. 错误处理

```go
// ✅ 检查错误
result := db.First(&article, id)
if result.Error != nil {
    if errors.Is(result.Error, gorm.ErrRecordNotFound) {
        return nil, errors.NewNotFound("article")
    }
    return nil, result.Error
}

// ✅ 检查 RowsAffected
result := db.Where("status = ?", 0).Delete(&model.Article{})
if result.Error != nil {
    return result.Error
}
if result.RowsAffected == 0 {
    return errors.New("no records deleted")
}
```

### 5. 连接池配置

```yaml
databases:
  default:
    max_connections: 20      # 最大连接数
    max_idle_conns: 10       # 最大空闲连接
    conn_max_lifetime: 3600  # 连接最大生命周期（秒）
```

## 调试

### 开启 SQL 日志

```yaml
log:
  level: debug
```

或在代码中：

```go
db = db.Debug()  // 打印所有 SQL
```

### 查看执行计划

```go
// 使用 EXPLAIN
var result []map[string]interface{}
db.Raw("EXPLAIN ANALYZE SELECT * FROM articles WHERE status = 1").Scan(&result)
```

### 慢查询日志

```go
// 记录慢查询（> 100ms）
db.Callback().Query().After("gorm:query").Register("slow_query", func(db *gorm.DB) {
    if db.Statement.SQL.String() != "" {
        elapsed := db.Statement.Elapsed
        if elapsed > 100*time.Millisecond {
            log.Printf("Slow query: %s (%v)", db.Statement.SQL.String(), elapsed)
        }
    }
})
```
