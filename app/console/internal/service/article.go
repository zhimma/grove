package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/pagination"
	"github.com/zhimma/grove/pkg/ulid"
)

type ArticleService struct {
	dbs   *database.Connections
	pages pagination.Policy
}

type ListArticlesInput struct {
	pagination.Request
	Keyword     string
	Category    string
	Status      *int
	OrderBy     []string
	CreatedFrom string
	CreatedTo   string
}

type ListArticlesOutput struct {
	List []model.Article
	Meta pagination.Meta
}

type GetArticleInput struct{ ArticleID string }

type CreateArticleInput struct {
	Title    string
	Slug     string
	Summary  string
	Content  string
	Cover    string
	Category string
	Status   int
	AuthorID string
}

type UpdateArticleInput struct {
	ArticleID string
	Title     *string
	Slug      *string
	Summary   *string
	Content   *string
	Cover     *string
	Category  *string
	Status    *int
}

type DeleteArticleInput struct{ ArticleID string }

func NewArticleService(dbs *database.Connections, pages pagination.Policy) *ArticleService {
	return &ArticleService{dbs: dbs, pages: pages}
}

func (s *ArticleService) ListArticles(ctx context.Context, in ListArticlesInput) (*ListArticlesOutput, error) {
	db, err := s.defaultDB()
	if err != nil {
		return nil, err
	}
	page := s.pages.Resolve(in.Request)

	query := db.WithContext(ctx).Model(&model.Article{})
	// 列表不读取 Markdown 正文，避免后台分页接口把大字段重复传输。
	query = query.Select("id, title, slug, summary, cover, category, status, published_at, author_id, created_at, updated_at")
	if keyword := strings.TrimSpace(in.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR slug LIKE ? OR summary LIKE ?", like, like, like)
	}
	if category := strings.TrimSpace(in.Category); category != "" {
		query = query.Where("category = ?", category)
	}
	if in.Status != nil {
		if !isArticleStatusValid(*in.Status) {
			return nil, invalidArticleParams("文章状态不合法")
		}
		query = query.Where("status = ?", *in.Status)
	}
	query, err = applyTimeRange(query, "created_at", in.CreatedFrom, in.CreatedTo)
	if err != nil {
		return nil, errx.InvalidParams().WithHTTPStatus(422).WithMessage("时间范围格式不正确")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	if len(in.OrderBy) == 0 {
		query = query.Order("created_at DESC")
	} else {
		for _, item := range in.OrderBy {
			field, direction := parseOrderBy(item)
			switch field {
			case "title", "category", "status", "published_at", "created_at", "updated_at":
				query = query.Order(field + " " + direction)
			}
		}
	}
	query = page.Apply(query)

	list := make([]model.Article, 0)
	if err := query.Find(&list).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	return &ListArticlesOutput{List: list, Meta: pagination.NewMeta(total, page)}, nil
}

func (s *ArticleService) GetArticle(ctx context.Context, in GetArticleInput) (*model.Article, error) {
	return s.loadArticle(ctx, in.ArticleID)
}

func (s *ArticleService) CreateArticle(ctx context.Context, in CreateArticleInput) (*model.Article, error) {
	db, err := s.defaultDB()
	if err != nil {
		return nil, err
	}
	title, slug, summary, content, cover, category, err := normalizeArticleFields(
		in.Title, in.Slug, in.Summary, in.Content, in.Cover, in.Category,
	)
	if err != nil {
		return nil, err
	}
	if slug == "" {
		slug = "article-" + strings.ToLower(ulid.New())
	}
	if !isArticleStatusValid(in.Status) {
		return nil, invalidArticleParams("文章状态不合法")
	}
	if err := s.ensureSlugAvailable(ctx, "", slug); err != nil {
		return nil, err
	}
	article := &model.Article{
		Title: title, Slug: slug, Summary: summary, Content: content, Cover: cover,
		Category: category, Status: in.Status, AuthorID: strings.TrimSpace(in.AuthorID),
	}
	if in.Status == model.ArticleStatusPublished {
		now := time.Now()
		article.PublishedAt = &now
	}
	if err := db.WithContext(ctx).Create(article).Error; err != nil {
		return nil, errx.Internal().WithCause(err)
	}
	return s.loadArticle(ctx, article.ID)
}

func (s *ArticleService) UpdateArticle(ctx context.Context, in UpdateArticleInput) (*model.Article, error) {
	article, err := s.loadArticle(ctx, in.ArticleID)
	if err != nil {
		return nil, err
	}
	updates := make(map[string]any)
	newSlug := article.Slug
	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if title == "" || len(title) > 200 {
			return nil, invalidArticleParams("文章标题不能为空且不能超过200个字符")
		}
		updates["title"] = title
	}
	if in.Slug != nil {
		newSlug = strings.TrimSpace(*in.Slug)
		if newSlug == "" || len(newSlug) > 180 {
			return nil, invalidArticleParams("文章标识不能为空且不能超过180个字符")
		}
		updates["slug"] = newSlug
	}
	if in.Summary != nil {
		summary := strings.TrimSpace(*in.Summary)
		if len(summary) > 500 {
			return nil, invalidArticleParams("文章摘要不能超过500个字符")
		}
		updates["summary"] = summary
	}
	if in.Content != nil {
		content := strings.TrimSpace(*in.Content)
		if content == "" {
			return nil, invalidArticleParams("文章内容不能为空")
		}
		updates["content"] = content
	}
	if in.Cover != nil {
		cover := strings.TrimSpace(*in.Cover)
		if len(cover) > 255 {
			return nil, invalidArticleParams("封面地址不能超过255个字符")
		}
		updates["cover"] = cover
	}
	if in.Category != nil {
		category := strings.TrimSpace(*in.Category)
		if len(category) > 80 {
			return nil, invalidArticleParams("文章分类不能超过80个字符")
		}
		updates["category"] = category
	}
	if in.Status != nil {
		if !isArticleStatusValid(*in.Status) {
			return nil, invalidArticleParams("文章状态不合法")
		}
		updates["status"] = *in.Status
		if *in.Status == model.ArticleStatusPublished {
			if article.PublishedAt == nil {
				updates["published_at"] = time.Now()
			}
		} else {
			updates["published_at"] = nil
		}
	}
	if err := s.ensureSlugAvailable(ctx, in.ArticleID, newSlug); err != nil {
		return nil, err
	}
	if len(updates) > 0 {
		db, dbErr := s.defaultDB()
		if dbErr != nil {
			return nil, dbErr
		}
		if err := db.WithContext(ctx).Model(&model.Article{}).Where("id = ?", article.ID).Updates(updates).Error; err != nil {
			return nil, errx.Internal().WithCause(err)
		}
	}
	return s.loadArticle(ctx, article.ID)
}

func (s *ArticleService) UpdateArticleStatus(ctx context.Context, articleID string, status int) (*model.Article, error) {
	return s.UpdateArticle(ctx, UpdateArticleInput{ArticleID: articleID, Status: &status})
}

func (s *ArticleService) DeleteArticle(ctx context.Context, in DeleteArticleInput) error {
	article, err := s.loadArticle(ctx, in.ArticleID)
	if err != nil {
		return err
	}
	db, err := s.defaultDB()
	if err != nil {
		return err
	}
	if err := db.WithContext(ctx).Delete(&model.Article{}, "id = ?", article.ID).Error; err != nil {
		return errx.Internal().WithCause(err)
	}
	return nil
}

func (s *ArticleService) loadArticle(ctx context.Context, articleID string) (*model.Article, error) {
	articleID = strings.TrimSpace(articleID)
	if articleID == "" {
		return nil, invalidArticleParams("文章ID不能为空")
	}
	db, err := s.defaultDB()
	if err != nil {
		return nil, err
	}
	var article model.Article
	if err := db.WithContext(ctx).Where("id = ?", articleID).First(&article).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errx.NotFound().WithMessage("文章不存在")
		}
		return nil, errx.Internal().WithCause(err)
	}
	return &article, nil
}

func (s *ArticleService) ensureSlugAvailable(ctx context.Context, articleID, slug string) error {
	db, err := s.defaultDB()
	if err != nil {
		return err
	}
	query := db.WithContext(ctx).Model(&model.Article{}).Where("slug = ?", slug)
	if articleID = strings.TrimSpace(articleID); articleID != "" {
		query = query.Where("id <> ?", articleID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return errx.Internal().WithCause(err)
	}
	if count > 0 {
		return errx.Conflict().WithCode("article_slug_exists").WithMessage("文章标识已被使用")
	}
	return nil
}

func (s *ArticleService) defaultDB() (*gorm.DB, error) {
	if s == nil || s.dbs == nil || s.dbs.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}
	return s.dbs.Default(), nil
}

func normalizeArticleFields(title, slug, summary, content, cover, category string) (string, string, string, string, string, string, error) {
	title = strings.TrimSpace(title)
	if title == "" || len(title) > 200 {
		return "", "", "", "", "", "", invalidArticleParams("文章标题不能为空且不能超过200个字符")
	}
	slug = strings.TrimSpace(slug)
	if len(slug) > 180 {
		return "", "", "", "", "", "", invalidArticleParams("文章标识不能超过180个字符")
	}
	summary = strings.TrimSpace(summary)
	if len(summary) > 500 {
		return "", "", "", "", "", "", invalidArticleParams("文章摘要不能超过500个字符")
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return "", "", "", "", "", "", invalidArticleParams("文章内容不能为空")
	}
	cover = strings.TrimSpace(cover)
	if len(cover) > 255 {
		return "", "", "", "", "", "", invalidArticleParams("封面地址不能超过255个字符")
	}
	category = strings.TrimSpace(category)
	if len(category) > 80 {
		return "", "", "", "", "", "", invalidArticleParams("文章分类不能超过80个字符")
	}
	return title, slug, summary, content, cover, category, nil
}

func invalidArticleParams(message string) error {
	return errx.InvalidParams().WithHTTPStatus(422).WithMessage(message)
}

func isArticleStatusValid(status int) bool {
	return status == model.ArticleStatusDraft || status == model.ArticleStatusPublished || status == model.ArticleStatusArchived
}
