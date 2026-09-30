package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/internal/model"
	"github.com/zhimma/grove/internal/testkit"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/pagination"
	"github.com/zhimma/grove/pkg/validation"
)

func bindTextRequest(t *testing.T, input, target any) error {
	t.Helper()
	body, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	return validation.BindJSON(c, target)
}

func TestUserTextLimitsMatchHTTPValidation(t *testing.T) {
	ctx := context.Background()
	db := testkit.OpenDB(t, &model.User{})
	svc := service.NewUserService(database.NewConnectionsFromDBs(db, nil), pagination.Policy{})
	name := strings.Repeat("中", 120)
	var req CreateUserRequest
	if err := bindTextRequest(t, map[string]any{"name": name, "email": "length@example.com"}, &req); err != nil {
		t.Fatal(err)
	}
	user, err := svc.CreateUser(ctx, service.CreateUserInput{Name: req.Name, Email: req.Email})
	if err != nil {
		t.Fatalf("valid HTTP request rejected by service: %v", err)
	}
	updated := strings.Repeat("文", 120)
	if _, err := svc.UpdateUser(ctx, service.UpdateUserInput{UserID: user.ID, Name: &updated}); err != nil {
		t.Fatal(err)
	}
	tooLong := name + "字"
	if err := bindTextRequest(t, map[string]any{"name": tooLong, "email": "other@example.com"}, &CreateUserRequest{}); err == nil {
		t.Fatal("HTTP accepted an oversized name")
	}
	if _, err := svc.CreateUser(ctx, service.CreateUserInput{Name: tooLong, Email: "other@example.com"}); err == nil {
		t.Fatal("service accepted an oversized name")
	}
	if _, err := svc.UpdateUser(ctx, service.UpdateUserInput{UserID: user.ID, Name: &tooLong}); err == nil {
		t.Fatal("update accepted an oversized name")
	}
}

func TestArticleTextLimitsMatchHTTPValidation(t *testing.T) {
	ctx := context.Background()
	db := testkit.OpenDB(t, &model.Article{})
	svc := service.NewArticleService(database.NewConnectionsFromDBs(db, nil), pagination.Policy{})
	var req CreateArticleRequest
	if err := bindTextRequest(t, map[string]any{
		"title": strings.Repeat("题", 200), "slug": strings.Repeat("文", 180),
		"summary": strings.Repeat("摘", 500), "content": "正文",
		"cover": strings.Repeat("图", 255), "category": strings.Repeat("类", 80),
	}, &req); err != nil {
		t.Fatal(err)
	}
	article, err := svc.CreateArticle(ctx, service.CreateArticleInput{
		Title: req.Title, Slug: req.Slug, Summary: req.Summary, Content: req.Content, Cover: req.Cover, Category: req.Category,
	})
	if err != nil {
		t.Fatalf("valid HTTP request rejected by service: %v", err)
	}
	if _, err := svc.UpdateArticle(ctx, service.UpdateArticleInput{
		ArticleID: article.ID, Title: &req.Title, Slug: &req.Slug, Summary: &req.Summary, Cover: &req.Cover, Category: &req.Category,
	}); err != nil {
		t.Fatal(err)
	}
	tooLong := req.Title + "字"
	if err := bindTextRequest(t, map[string]any{"title": tooLong, "content": "正文"}, &CreateArticleRequest{}); err == nil {
		t.Fatal("HTTP accepted an oversized title")
	}
	if _, err := svc.CreateArticle(ctx, service.CreateArticleInput{Title: tooLong, Content: "正文"}); err == nil {
		t.Fatal("service accepted an oversized title")
	}
	if _, err := svc.UpdateArticle(ctx, service.UpdateArticleInput{ArticleID: article.ID, Title: &tooLong}); err == nil {
		t.Fatal("update accepted an oversized title")
	}
}
