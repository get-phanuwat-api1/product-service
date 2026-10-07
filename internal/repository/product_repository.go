// Package repository is the data access layer (PostgreSQL).
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phanuwat/product-service/internal/apperror"
	"github.com/phanuwat/product-service/internal/model"
)

// ProductRepository persists products.
type ProductRepository interface {
	Create(ctx context.Context, p *model.Product) error
	// GetByIDForUpdate loads a product and locks the row when called inside a transaction.
	GetByIDForUpdate(ctx context.Context, id int64) (*model.Product, error)
	Update(ctx context.Context, p *model.Product) error
}

type productRepository struct {
	pool *pgxpool.Pool
}

func NewProductRepository(pool *pgxpool.Pool) ProductRepository {
	return &productRepository{pool: pool}
}

func (r *productRepository) Create(ctx context.Context, p *model.Product) error {
	const q = `
		INSERT INTO products (name, description, price, sale_price)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`
	err := conn(ctx, r.pool).
		QueryRow(ctx, q, p.Name, p.Description, p.Price, p.SalePrice).
		Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert product: %w", err)
	}
	return nil
}

func (r *productRepository) GetByIDForUpdate(ctx context.Context, id int64) (*model.Product, error) {
	q := `SELECT id, name, description, price, sale_price, created_at, updated_at
	      FROM products WHERE id = $1`
	if inTx(ctx) {
		q += " FOR UPDATE"
	}

	var p model.Product
	err := conn(ctx, r.pool).QueryRow(ctx, q, id).
		Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.SalePrice, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.ErrProductNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	return &p, nil
}

func (r *productRepository) Update(ctx context.Context, p *model.Product) error {
	const q = `
		UPDATE products
		SET name = $2, description = $3, price = $4, sale_price = $5, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`
	err := conn(ctx, r.pool).
		QueryRow(ctx, q, p.ID, p.Name, p.Description, p.Price, p.SalePrice).
		Scan(&p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperror.ErrProductNotFound
	}
	if err != nil {
		return fmt.Errorf("update product: %w", err)
	}
	return nil
}
