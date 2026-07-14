package main

import "fmt"

func modelTemplate(name, snake string) string {
	return fmt.Sprintf(`package model

type %s struct {
	Base
}

func (%s) TableName() string {
	return "%s"
}
`, name, name, toSnakePlural(snake))
}

func consoleServiceTemplate(name, snake string) string {
	return fmt.Sprintf(`package service

import (
	"context"

	"github.com/zhimma/grove/pkg/database"
	"github.com/zhimma/grove/pkg/errx"
)

type %sService struct {
	dbRepo database.Repo
}

type %sListInput struct{}

type %sListOutput struct {
	Message string `+"`json:\"message\"`"+`
}

func New%sService(dbRepo database.Repo) *%sService {
	return &%sService{dbRepo: dbRepo}
}

func (s *%sService) List(_ context.Context, _ %sListInput) (*%sListOutput, error) {
	if s.dbRepo == nil || s.dbRepo.Default() == nil {
		return nil, errx.ServiceUnavailable().WithMessage("默认数据库未配置")
	}
	return &%sListOutput{Message: "%s 模块已就绪"}, nil
}
`, name, name, name, name, name, name, name, name, name, name, name)
}

func consoleHandlerTemplate(name, snake string) string {
	routePath := "/" + toKebabPlural(snake)
	return fmt.Sprintf(`package handler

import (
	"github.com/gin-gonic/gin"

	consoleservice "github.com/zhimma/grove/app/console/internal/service"
	"github.com/zhimma/grove/internal/provider"
	"github.com/zhimma/grove/pkg/response"
	"github.com/zhimma/grove/pkg/route"
)

type %sHandler struct {
	%sSvc *consoleservice.%sService
}

func Register%sRoutes(protected *gin.RouterGroup, p *provider.Provider) {
	h := &%sHandler{
		%sSvc: consoleservice.New%sService(p.DB),
	}

	group := route.Wrap(protected.Group("%s"))
	group.GET("", h.List).Name("%s.列表")
}

func (h *%sHandler) List(c *gin.Context) {
	out, err := h.%sSvc.List(c.Request.Context(), consoleservice.%sListInput{})
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Success(c, out)
}
`, name, snake, name, name, name, snake, name, routePath, name, name, snake, name)
}
