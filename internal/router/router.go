// Package router registers HTTP routes and middleware.
package router

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/get-phanuwat-api1/product-service/docs" // generated swagger spec
	"github.com/get-phanuwat-api1/product-service/internal/handler"
	"github.com/get-phanuwat-api1/product-service/internal/middleware"
)

// NewRouter builds the gin engine with middleware and all routes registered.
func NewRouter(logger *slog.Logger, productHandler *handler.ProductHandler) *gin.Engine {
	r := gin.New()
	// Order matters: request context first, recovery innermost so the access log sees the 500.
	r.Use(
		middleware.RequestID(),
		middleware.Actor(),
		middleware.AccessLog(logger, "/health", "/api-docs"),
		middleware.Recovery(logger),
	)

	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	r.POST("/product", productHandler.CreateProduct)
	r.PATCH("/product/:id", productHandler.PatchProductByID)

	r.GET("/api-docs", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/api-docs/index.html") })
	r.GET("/api-docs/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return r
}
