package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/route"
)

func wrapRoute(group *gin.RouterGroup, catalog *route.Catalog) *route.Group {
	return route.Wrap(group, catalog)
}
