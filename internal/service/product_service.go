// Package service is the business logic layer: it orchestrates models and repositories.
package service

import (
	"context"
	"fmt"

	"github.com/phanuwat/product-service/internal/apperror"
	"github.com/phanuwat/product-service/internal/model"
	"github.com/phanuwat/product-service/internal/repository"
)

// CreateProductInput is the input for creating a product.
type CreateProductInput struct {
	Name        string
	Description *string
	Price       float64
	SalePrice   *float64
}

// ProductService defines product business operations.
type ProductService interface {
	Create(ctx context.Context, in CreateProductInput) (*model.Product, error)
	Patch(ctx context.Context, id int64, patch model.ProductPatch) (*model.Product, error)
}

type productService struct {
	repo repository.ProductRepository
	tx   repository.Transactor
}

// NewProductService wires the service with its dependencies.
func NewProductService(repo repository.ProductRepository, tx repository.Transactor) ProductService {
	return &productService{repo: repo, tx: tx}
}

func (s *productService) Create(ctx context.Context, in CreateProductInput) (*model.Product, error) {
	p, err := model.NewProduct(in.Name, in.Description, in.Price, in.SalePrice)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("create product: %w", err)
	}
	return p, nil
}

func (s *productService) Patch(ctx context.Context, id int64, patch model.ProductPatch) (*model.Product, error) {
	if id <= 0 {
		return nil, apperror.New(apperror.CodeInvalidProductID, "id must be a positive integer")
	}

	var result *model.Product
	err := s.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		p, err := s.repo.GetByIDForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if patch.IsEmpty() {
			result = p
			return nil
		}
		if err := p.ApplyPatch(patch); err != nil {
			return err
		}
		if err := s.repo.Update(ctx, p); err != nil {
			return fmt.Errorf("update product: %w", err)
		}
		result = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
