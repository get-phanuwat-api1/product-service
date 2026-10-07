package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/phanuwat/product-service/internal/app"
	"github.com/phanuwat/product-service/internal/config"
	"github.com/phanuwat/product-service/internal/repository/postgres"
)

//	@title			Product Service API
//	@version		1.0
//	@description	Product REST API built with Clean Architecture, Gin and PostgreSQL.
//	@BasePath		/
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := postgres.Migrate(ctx, pool); err != nil {
		logger.Error("migrate database", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           app.NewHandler(pool, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("server started", "addr", srv.Addr, "swagger", "http://localhost:"+cfg.HTTPPort+"/api-docs")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}
