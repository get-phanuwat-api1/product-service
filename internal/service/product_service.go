// Package service is the business logic layer: it orchestrates models and repositories.
package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/get-phanuwat-api1/product-service/internal/apperror"
	"github.com/get-phanuwat-api1/product-service/internal/model"
	"github.com/get-phanuwat-api1/product-service/internal/repository"
	"github.com/get-phanuwat-api1/product-service/pkg/requestctx"
)

// SystemActor is recorded in audit logs when no actor is present in the context.
const SystemActor = "system"

// CreateProductInput is the input for creating a product.
type CreateProductInput struct {
	Name        string
	Description *string
	Price       float64
	SalePrice   *float64
}

// ProductService defines product business operations.
type ProductService interface {
	// CreateProduct validates and stores a new product, recording a PRODUCT_CREATED audit log.
	CreateProduct(ctx context.Context, in CreateProductInput) (*model.Product, error)
	// PatchProductByID overwrites only the provided fields, recording a PRODUCT_UPDATED audit log
	// when something actually changed.
	PatchProductByID(ctx context.Context, id int64, patch model.ProductPatch) (*model.Product, error)
}

type productService struct {
	productRepo  repository.ProductRepository
	auditLogRepo repository.AuditLogRepository
	tx           repository.Transactor
	logger       *slog.Logger
}

// NewProductService wires the service with its dependencies.
func NewProductService(
	productRepo repository.ProductRepository,
	auditLogRepo repository.AuditLogRepository,
	tx repository.Transactor,
	logger *slog.Logger,
) ProductService {
	return &productService{productRepo: productRepo, auditLogRepo: auditLogRepo, tx: tx, logger: logger}
}

func (s *productService) CreateProduct(ctx context.Context, in CreateProductInput) (*model.Product, error) {
	p, err := model.NewProduct(in.Name, in.Description, in.Price, in.SalePrice)
	if err != nil {
		return nil, err
	}

	err = s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		if err := s.productRepo.CreateProduct(ctx, p); err != nil {
			return fmt.Errorf("create product: %w", err)
		}
		return s.writeProductAuditLog(ctx, model.AuditActionProductCreated, nil, p.Snapshot())
	})
	if err != nil {
		return nil, err
	}

	s.logger.InfoContext(ctx, "product created", slog.Int64("product_id", p.ID))
	return p, nil
}

func (s *productService) PatchProductByID(ctx context.Context, id int64, patch model.ProductPatch) (*model.Product, error) {
	if id <= 0 {
		return nil, apperror.New(apperror.CodeInvalidProductID, "id must be a positive integer")
	}

	var (
		result  *model.Product
		changed bool
	)
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		p, err := s.productRepo.GetProductByIDForUpdate(ctx, id)
		if err != nil {
			return err
		}
		result = p
		if patch.IsEmpty() {
			return nil
		}

		before := p.Snapshot()
		if err := p.ApplyPatch(patch); err != nil {
			return err
		}
		after := p.Snapshot()
		if before.Equal(after) {
			return nil // values identical: nothing to write or audit
		}

		if err := s.productRepo.UpdateProduct(ctx, p); err != nil {
			return fmt.Errorf("update product: %w", err)
		}
		changed = true
		return s.writeProductAuditLog(ctx, model.AuditActionProductUpdated, &before, after)
	})
	if err != nil {
		return nil, err
	}

	if changed {
		s.logger.InfoContext(ctx, "product updated", slog.Int64("product_id", id))
	} else {
		s.logger.DebugContext(ctx, "product patch had no changes", slog.Int64("product_id", id))
	}
	return result, nil
}

func (s *productService) writeProductAuditLog(ctx context.Context, action model.AuditAction, before *model.ProductSnapshot, after model.ProductSnapshot) error {
	actor := requestctx.Actor(ctx)
	if actor == "" {
		actor = SystemActor
	}
	entry, err := model.NewProductAuditLog(action, before, after, actor, requestctx.RequestID(ctx))
	if err != nil {
		return err
	}
	if err := s.auditLogRepo.CreateAuditLog(ctx, entry); err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return nil
}
