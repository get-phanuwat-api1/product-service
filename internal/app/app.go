// Package app is the composition root: it wires every layer together (dependency injection).
package app

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	httpdelivery "github.com/phanuwat/product-service/internal/delivery/http"
	"github.com/phanuwat/product-service/internal/repository/postgres"
	"github.com/phanuwat/product-service/internal/usecase"
)

// NewHandler builds the full HTTP handler graph from a database pool.
func NewHandler(pool *pgxpool.Pool, logger *slog.Logger) *gin.Engine {
	repo := postgres.NewProductRepository(pool)
	tx := postgres.NewTransactor(pool)
	productUC := usecase.NewProductUsecase(repo, tx)
	productHandler := httpdelivery.NewProductHandler(productUC, logger)
	return httpdelivery.NewRouter(productHandler)
}
