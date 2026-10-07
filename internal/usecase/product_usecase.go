package usecase

import (
	"context"
	"fmt"

	"github.com/phanuwat/product-service/internal/domain"
)

// CreateProductInput is the input for creating a product.
type CreateProductInput struct {
	Name        string
	Description *string
	Price       float64
	SalePrice   *float64
}

// ProductUsecase orchestrates product application flows.
type ProductUsecase interface {
	Create(ctx context.Context, in CreateProductInput) (*domain.Product, error)
	Patch(ctx context.Context, id int64, patch domain.ProductPatch) (*domain.Product, error)
}

type productUsecase struct {
	repo domain.ProductRepository
	tx   domain.Transactor
}

// NewProductUsecase wires the usecase with its dependencies.
func NewProductUsecase(repo domain.ProductRepository, tx domain.Transactor) ProductUsecase {
	return &productUsecase{repo: repo, tx: tx}
}

func (u *productUsecase) Create(ctx context.Context, in CreateProductInput) (*domain.Product, error) {
	p, err := domain.NewProduct(in.Name, in.Description, in.Price, in.SalePrice)
	if err != nil {
		return nil, err
	}
	if err := u.repo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("create product: %w", err)
	}
	return p, nil
}

func (u *productUsecase) Patch(ctx context.Context, id int64, patch domain.ProductPatch) (*domain.Product, error) {
	if id <= 0 {
		return nil, domain.NewError(domain.ErrCodeInvalidProductID, "id must be a positive integer")
	}

	var result *domain.Product
	err := u.tx.WithinTransaction(ctx, func(ctx context.Context) error {
		p, err := u.repo.GetByIDForUpdate(ctx, id)
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
		if err := u.repo.Update(ctx, p); err != nil {
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
