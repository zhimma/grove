package handler

import "github.com/zhimma/grove/internal/model"

type ArticleResponse struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Summary     string `json:"summary"`
	Content     string `json:"content"`
	Cover       string `json:"cover"`
	Category    string `json:"category"`
	Status      int    `json:"status"`
	StatusText  string `json:"status_text"`
	PublishedAt string `json:"published_at,omitempty"`
	AuthorID    string `json:"author_id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type ArticleListItemResponse struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Summary     string `json:"summary"`
	Cover       string `json:"cover"`
	Category    string `json:"category"`
	Status      int    `json:"status"`
	StatusText  string `json:"status_text"`
	PublishedAt string `json:"published_at,omitempty"`
	AuthorID    string `json:"author_id"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func newArticleListItemResponse(article *model.Article) ArticleListItemResponse {
	if article == nil {
		return ArticleListItemResponse{}
	}
	result := ArticleListItemResponse{
		ID: article.ID, Title: article.Title, Slug: article.Slug, Summary: article.Summary,
		Cover: article.Cover, Category: article.Category, Status: article.Status,
		StatusText: articleStatusToText(article.Status), AuthorID: article.AuthorID,
		CreatedAt: article.CreatedAt.Format("2006-01-02 15:04:05"), UpdatedAt: article.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
	if article.PublishedAt != nil && !article.PublishedAt.IsZero() {
		result.PublishedAt = article.PublishedAt.Format("2006-01-02 15:04:05")
	}
	return result
}

func newArticleResponse(article *model.Article) ArticleResponse {
	if article == nil {
		return ArticleResponse{}
	}
	result := ArticleResponse{
		ID: article.ID, Title: article.Title, Slug: article.Slug, Summary: article.Summary,
		Content: article.Content, Cover: article.Cover, Category: article.Category, Status: article.Status,
		StatusText: articleStatusToText(article.Status), AuthorID: article.AuthorID,
		CreatedAt: article.CreatedAt.Format("2006-01-02 15:04:05"), UpdatedAt: article.UpdatedAt.Format("2006-01-02 15:04:05"),
	}
	if article.PublishedAt != nil && !article.PublishedAt.IsZero() {
		result.PublishedAt = article.PublishedAt.Format("2006-01-02 15:04:05")
	}
	return result
}

func articleStatusToText(status int) string {
	switch status {
	case model.ArticleStatusDraft:
		return "草稿"
	case model.ArticleStatusPublished:
		return "已发布"
	case model.ArticleStatusArchived:
		return "已归档"
	default:
		return "未知"
	}
}
