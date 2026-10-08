package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/gin-gonic/gin"

	"github.com/get-phanuwat-api1/product-service/config"
	"github.com/get-phanuwat-api1/product-service/internal/container"
	"github.com/get-phanuwat-api1/product-service/migrations"
	"github.com/get-phanuwat-api1/product-service/pkg/database"
	"github.com/get-phanuwat-api1/product-service/pkg/logger"
)

// @title			Product Service API
// @version		1.0
// @description	Product REST API (handler -> service -> repository) with Gin and PostgreSQL.
// @BasePath		/
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log, err := logger.NewLogger(os.Stdout, logger.Options{
		Level:   cfg.Log.Level,
		Format:  cfg.Log.Format,
		Service: cfg.App.Name,
	})
	if err != nil {
		return err
	}
	slog.SetDefault(log)
	if cfg.App.Env != "local" {
		gin.SetMode(gin.ReleaseMode)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.Database.AutoMigrate {
		log.Info("running database migrations")
		version, err := database.MigrateUp(migrations.FS, cfg.Database.DSN())
		if err != nil {
			return fmt.Errorf("auto-migrate: %w", err)
		}
		log.Info("database schema is up to date", "version", version)
	}

	pool, err := database.NewPostgresPool(ctx, cfg.Database.DSN())
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	c := container.NewContainer(pool, log)

	port := strconv.Itoa(cfg.Server.Port)
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      c.Router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("server started", "addr", srv.Addr, "swagger", "http://localhost:"+port+"/api-docs")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Info("server stopped")
	return nil
}
