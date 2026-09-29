package service

import (
	"context"
	"testing"

	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/internal/testkit"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
	"github.com/zhimma/grove/pkg/pagination"
)

func TestArticleServiceCRUDAndPublishing(t *testing.T) {
	db := testkit.OpenDB(t, &model.Article{})
	service := NewArticleService(database.NewConnectionsFromDBs(db, nil), pagination.Policy{Default: 10, Max: 100})
	ctx := context.Background()

	draft, err := service.CreateArticle(ctx, CreateArticleInput{
		Title: "第一篇文章", Slug: "first-article", Content: "正文", Category: "公告", Status: model.ArticleStatusDraft,
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if draft.PublishedAt != nil {
		t.Fatal("draft must not have published_at")
	}
	if _, err := service.CreateArticle(ctx, CreateArticleInput{
		Title: "重复标识", Slug: "first-article", Content: "正文", Status: model.ArticleStatusDraft,
	}); err == nil || errx.Normalize(err).HTTPStatus != 409 {
		t.Fatalf("expected duplicate slug conflict, got %v", err)
	}

	published, err := service.UpdateArticleStatus(ctx, draft.ID, model.ArticleStatusPublished)
	if err != nil {
		t.Fatalf("publish article: %v", err)
	}
	if !published.IsPublished() || published.PublishedAt == nil {
		t.Fatalf("published article missing publication time: %#v", published)
	}

	result, err := service.ListArticles(ctx, ListArticlesInput{Status: ptrInt(model.ArticleStatusPublished)})
	if err != nil || len(result.List) != 1 {
		t.Fatalf("list published articles: len=%d err=%v", len(result.List), err)
	}
	if err := service.DeleteArticle(ctx, DeleteArticleInput{ArticleID: draft.ID}); err != nil {
		t.Fatalf("delete article: %v", err)
	}
	result, err = service.ListArticles(ctx, ListArticlesInput{Request: pagination.Request{ListAll: true}})
	if err != nil || len(result.List) != 0 {
		t.Fatalf("soft deleted article should be hidden: len=%d err=%v", len(result.List), err)
	}
}

func TestArticleServiceRejectsInvalidInput(t *testing.T) {
	db := testkit.OpenDB(t, &model.Article{})
	service := NewArticleService(database.NewConnectionsFromDBs(db, nil), pagination.Policy{})
	for _, input := range []CreateArticleInput{
		{Title: "", Content: "正文"},
		{Title: "标题", Content: ""},
		{Title: "标题", Content: "正文", Status: 9},
	} {
		_, err := service.CreateArticle(context.Background(), input)
		if err == nil || errx.Normalize(err) == nil || errx.Normalize(err).HTTPStatus != 422 {
			t.Fatalf("expected invalid params, got %v", err)
		}
	}
}
