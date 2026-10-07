package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/phanuwat/product-service/docs" // generated swagger spec
)

// NewRouter builds the gin engine with all routes registered.
func NewRouter(h *ProductHandler) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	r.POST("/product", h.Create)
	r.PATCH("/product/:id", h.Patch)

	r.GET("/api-docs", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/api-docs/index.html") })
	r.GET("/api-docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return r
}
