// Package container is the composition root: it creates every dependency
// and injects it into the layer above (repository -> service -> handler -> router).
package container

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/get-phanuwat-api1/product-service/internal/handler"
	"github.com/get-phanuwat-api1/product-service/internal/repository"
	"github.com/get-phanuwat-api1/product-service/internal/router"
	"github.com/get-phanuwat-api1/product-service/internal/service"
)

// Container holds the wired application graph.
type Container struct {
	ProductRepository  repository.ProductRepository
	AuditLogRepository repository.AuditLogRepository
	ProductService     service.ProductService
	ProductHandler     *handler.ProductHandler
	Router             *gin.Engine
}

// NewContainer wires all dependencies.
func NewContainer(pool *pgxpool.Pool, logger *slog.Logger) *Container {
	productRepo := repository.NewProductRepository(pool)
	auditLogRepo := repository.NewAuditLogRepository(pool)
	transactor := repository.NewTransactor(pool)

	productSvc := service.NewProductService(productRepo, auditLogRepo, transactor, logger.With("layer", "service"))
	productHandler := handler.NewProductHandler(productSvc, logger.With("layer", "handler"))

	return &Container{
		ProductRepository:  productRepo,
		AuditLogRepository: auditLogRepo,
		ProductService:     productSvc,
		ProductHandler:     productHandler,
		Router:             router.NewRouter(logger.With("layer", "http"), productHandler),
	}
}
