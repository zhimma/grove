package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/request"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
	"github.com/zhimma/grove/pkg/validation"
)

type ArticleHandler struct {
	articleSvc *consoleservice.ArticleService
}

type ListArticlesRequest struct {
	ListQuery
	Category string `form:"category" label:"分类"`
	Status   *int   `form:"status" label:"状态"`
}

type ListArticlesResponse struct {
	List []ArticleListItemResponse `json:"list"`
	Meta ListMeta                  `json:"meta"`
}

type CreateArticleRequest struct {
	Title    string `json:"title" binding:"required,max=200" label:"标题"`
	Slug     string `json:"slug" binding:"omitempty,max=180" label:"标识"`
	Summary  string `json:"summary" binding:"omitempty,max=500" label:"摘要"`
	Content  string `json:"content" binding:"required" label:"内容"`
	Cover    string `json:"cover" binding:"omitempty,max=255" label:"封面"`
	Category string `json:"category" binding:"omitempty,max=80" label:"分类"`
	Status   int    `json:"status" binding:"omitempty,oneof=0 1 2" label:"状态"`
}

type UpdateArticleRequest struct {
	Title    *string `json:"title" binding:"omitempty,max=200" label:"标题"`
	Slug     *string `json:"slug" binding:"omitempty,max=180" label:"标识"`
	Summary  *string `json:"summary" binding:"omitempty,max=500" label:"摘要"`
	Content  *string `json:"content" binding:"omitempty" label:"内容"`
	Cover    *string `json:"cover" binding:"omitempty,max=255" label:"封面"`
	Category *string `json:"category" binding:"omitempty,max=80" label:"分类"`
	Status   *int    `json:"status" binding:"omitempty,oneof=0 1 2" label:"状态"`
}

type UpdateArticleStatusRequest struct {
	Status *int `json:"status" binding:"required,oneof=0 1 2" label:"状态"`
}

type ArticlePathRequest struct {
	ID string `uri:"id" binding:"required" label:"文章ID"`
}

func RegisterArticleRoutes(protected *gin.RouterGroup, dbs database.Connections, policies []consoleservice.PagePolicy, catalog *route.Catalog) {
	h := &ArticleHandler{articleSvc: consoleservice.NewArticleService(dbs, policies...)}
	articles := wrapRoute(protected.Group("/articles"), catalog)
	articles.GET("", h.List).Name("内容管理.文章列表")
	articles.GET("/:id", h.Detail).Name("内容管理.文章详情")
	articles.POST("", h.Create).Name("内容管理.创建文章")
	articles.PUT("/:id", h.Update).Name("内容管理.更新文章")
	articles.PUT("/:id/status", h.UpdateStatus).Name("内容管理.更新文章状态")
	articles.DELETE("/:id", h.Delete).Name("内容管理.删除文章")
}

func (h *ArticleHandler) List(c *gin.Context) {
	var req ListArticlesRequest
	if err := validation.BindQuery(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := h.articleSvc.ListArticles(c.Request.Context(), consoleservice.ListArticlesInput{
		Page: req.Page, PageSize: req.PageSize, Offset: req.Offset, Limit: req.Limit, ListAll: req.ListAll,
		Keyword: req.Keyword, Category: req.Category, Status: req.Status, OrderBy: req.OrderBy,
		CreatedFrom: req.CreatedFrom, CreatedTo: req.CreatedTo,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	items := make([]ArticleListItemResponse, 0, len(result.List))
	for i := range result.List {
		items = append(items, newArticleListItemResponse(&result.List[i]))
	}
	response.Success(c, ListArticlesResponse{List: items, Meta: ListMeta(result.Meta)})
}

func (h *ArticleHandler) Detail(c *gin.Context) {
	var req ArticlePathRequest
	if err := validation.BindURI(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	article, err := h.articleSvc.GetArticle(c.Request.Context(), consoleservice.GetArticleInput{ArticleID: req.ID})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, newArticleResponse(article))
}

func (h *ArticleHandler) Create(c *gin.Context) {
	var req CreateArticleRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	article, err := h.articleSvc.CreateArticle(c.Request.Context(), consoleservice.CreateArticleInput{
		Title: req.Title, Slug: req.Slug, Summary: req.Summary, Content: req.Content, Cover: req.Cover,
		Category: req.Category, Status: req.Status, AuthorID: request.GetAdminID(c),
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "article", article.ID, map[string]any{"title": article.Title, "status": article.Status})
	response.Success(c, newArticleResponse(article))
}

func (h *ArticleHandler) Update(c *gin.Context) {
	var pathReq ArticlePathRequest
	if err := validation.BindURI(c, &pathReq); err != nil {
		response.Fail(c, err)
		return
	}
	var req UpdateArticleRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	article, err := h.articleSvc.UpdateArticle(c.Request.Context(), consoleservice.UpdateArticleInput{
		ArticleID: pathReq.ID, Title: req.Title, Slug: req.Slug, Summary: req.Summary, Content: req.Content,
		Cover: req.Cover, Category: req.Category, Status: req.Status,
	})
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "article", article.ID, map[string]any{"title": article.Title, "status": article.Status})
	response.Success(c, newArticleResponse(article))
}

func (h *ArticleHandler) UpdateStatus(c *gin.Context) {
	var pathReq ArticlePathRequest
	if err := validation.BindURI(c, &pathReq); err != nil {
		response.Fail(c, err)
		return
	}
	var req UpdateArticleStatusRequest
	if err := validation.BindJSON(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	article, err := h.articleSvc.UpdateArticleStatus(c.Request.Context(), pathReq.ID, *req.Status)
	if err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "article", article.ID, map[string]any{"status": article.Status})
	response.Success(c, newArticleResponse(article))
}

func (h *ArticleHandler) Delete(c *gin.Context) {
	var req ArticlePathRequest
	if err := validation.BindURI(c, &req); err != nil {
		response.Fail(c, err)
		return
	}
	if err := h.articleSvc.DeleteArticle(c.Request.Context(), consoleservice.DeleteArticleInput{ArticleID: req.ID}); err != nil {
		response.Fail(c, err)
		return
	}
	setAuditMeta(c, "article", req.ID, map[string]any{"deleted": true})
	response.Success(c, nil)
}
