package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/phanuwat/product-service/internal/domain"
)

// ProductRepository is the PostgreSQL implementation of domain.ProductRepository.
type ProductRepository struct {
	pool *pgxpool.Pool
}

var _ domain.ProductRepository = (*ProductRepository)(nil)

func NewProductRepository(pool *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{pool: pool}
}

func (r *ProductRepository) Create(ctx context.Context, p *domain.Product) error {
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

func (r *ProductRepository) GetByIDForUpdate(ctx context.Context, id int64) (*domain.Product, error) {
	q := `SELECT id, name, description, price, sale_price, created_at, updated_at
	      FROM products WHERE id = $1`
	if _, inTx := ctx.Value(txKey{}).(pgx.Tx); inTx {
		q += " FOR UPDATE"
	}

	var p domain.Product
	err := conn(ctx, r.pool).QueryRow(ctx, q, id).
		Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.SalePrice, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrProductNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get product: %w", err)
	}
	return &p, nil
}

func (r *ProductRepository) Update(ctx context.Context, p *domain.Product) error {
	const q = `
		UPDATE products
		SET name = $2, description = $3, price = $4, sale_price = $5, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at`
	err := conn(ctx, r.pool).
		QueryRow(ctx, q, p.ID, p.Name, p.Description, p.Price, p.SalePrice).
		Scan(&p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrProductNotFound
	}
	if err != nil {
		return fmt.Errorf("update product: %w", err)
	}
	return nil
}
