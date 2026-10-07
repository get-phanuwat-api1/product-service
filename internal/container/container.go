// Package container is the composition root: it creates every dependency
// and injects it into the layer above (repository -> service -> handler -> router).
package container

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phanuwat/product-service/internal/handler"
	"github.com/phanuwat/product-service/internal/repository"
	"github.com/phanuwat/product-service/internal/router"
	"github.com/phanuwat/product-service/internal/service"
)

// Container holds the wired application graph.
type Container struct {
	ProductRepository repository.ProductRepository
	ProductService    service.ProductService
	ProductHandler    *handler.ProductHandler
	Router            *gin.Engine
}

// New wires all dependencies.
func New(pool *pgxpool.Pool, logger *slog.Logger) *Container {
	productRepo := repository.NewProductRepository(pool)
	transactor := repository.NewTransactor(pool)
	productSvc := service.NewProductService(productRepo, transactor)
	productHandler := handler.NewProductHandler(productSvc, logger)

	return &Container{
		ProductRepository: productRepo,
		ProductService:    productSvc,
		ProductHandler:    productHandler,
		Router:            router.New(productHandler),
	}
}
