// Package mocks contains testify mocks for the layer interfaces.
package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/phanuwat/product-service/internal/model"
	"github.com/phanuwat/product-service/internal/repository"
	"github.com/phanuwat/product-service/internal/service"
)

// ProductRepository mocks repository.ProductRepository.
type ProductRepository struct{ mock.Mock }

var _ repository.ProductRepository = (*ProductRepository)(nil)

func (m *ProductRepository) Create(ctx context.Context, p *model.Product) error {
	return m.Called(ctx, p).Error(0)
}

func (m *ProductRepository) GetByIDForUpdate(ctx context.Context, id int64) (*model.Product, error) {
	args := m.Called(ctx, id)
	p, _ := args.Get(0).(*model.Product)
	return p, args.Error(1)
}

func (m *ProductRepository) Update(ctx context.Context, p *model.Product) error {
	return m.Called(ctx, p).Error(0)
}

// Transactor is a fake repository.Transactor that runs fn directly and counts calls.
type Transactor struct{ Calls int }

var _ repository.Transactor = (*Transactor)(nil)

func (t *Transactor) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	t.Calls++
	return fn(ctx)
}

// ProductService mocks service.ProductService.
type ProductService struct{ mock.Mock }

var _ service.ProductService = (*ProductService)(nil)

func (m *ProductService) Create(ctx context.Context, in service.CreateProductInput) (*model.Product, error) {
	args := m.Called(ctx, in)
	p, _ := args.Get(0).(*model.Product)
	return p, args.Error(1)
}

func (m *ProductService) Patch(ctx context.Context, id int64, patch model.ProductPatch) (*model.Product, error) {
	args := m.Called(ctx, id, patch)
	p, _ := args.Get(0).(*model.Product)
	return p, args.Error(1)
}
