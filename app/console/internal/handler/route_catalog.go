package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/zhimma/grove/pkg/route"
)

func routeCatalog(catalogs []*route.Catalog) *route.Catalog {
	if len(catalogs) == 0 {
		return nil
	}
	return catalogs[0]
}

func wrapRoute(group *gin.RouterGroup, catalog *route.Catalog) *route.Group {
	return route.WrapWithCatalog(group, catalog)
}
