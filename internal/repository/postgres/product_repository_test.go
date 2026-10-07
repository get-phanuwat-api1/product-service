package postgres_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phanuwat/product-service/internal/domain"
	"github.com/phanuwat/product-service/internal/repository/postgres"
	"github.com/phanuwat/product-service/internal/testutil"
)

var pool *pgxpool.Pool

func TestMain(m *testing.M) {
	flag.Parse()
	if testing.Short() {
		fmt.Println("skipping repository integration tests in -short mode")
		os.Exit(0)
	}
	ctx := context.Background()
	p, cleanup, err := testutil.StartPostgres(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot start postgres (is Docker running?): %v\n", err)
		os.Exit(1)
	}
	pool = p
	code := m.Run()
	cleanup()
	os.Exit(code)
}

func ptr[T any](v T) *T { return &v }

func TestProductRepository_CreateAndGet(t *testing.T) {
	testutil.TruncateProducts(t, pool)
	repo := postgres.NewProductRepository(pool)
	ctx := context.Background()

	p := &domain.Product{Name: "Apple", Description: ptr("fresh"), Price: 10.5, SalePrice: ptr(9.25)}
	require.NoError(t, repo.Create(ctx, p))
	assert.NotZero(t, p.ID)
	assert.False(t, p.CreatedAt.IsZero())

	got, err := repo.GetByIDForUpdate(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "Apple", got.Name)
	assert.Equal(t, "fresh", *got.Description)
	assert.Equal(t, 10.5, got.Price)
	assert.Equal(t, 9.25, *got.SalePrice)
}

func TestProductRepository_CreateWithNulls(t *testing.T) {
	testutil.TruncateProducts(t, pool)
	repo := postgres.NewProductRepository(pool)
	ctx := context.Background()

	p := &domain.Product{Name: "Plain", Price: 1}
	require.NoError(t, repo.Create(ctx, p))

	got, err := repo.GetByIDForUpdate(ctx, p.ID)
	require.NoError(t, err)
	assert.Nil(t, got.Description)
	assert.Nil(t, got.SalePrice)
}

func TestProductRepository_GetNotFound(t *testing.T) {
	testutil.TruncateProducts(t, pool)
	repo := postgres.NewProductRepository(pool)

	_, err := repo.GetByIDForUpdate(context.Background(), 12345)
	assert.ErrorIs(t, err, domain.ErrProductNotFound)
}

func TestProductRepository_Update(t *testing.T) {
	testutil.TruncateProducts(t, pool)
	repo := postgres.NewProductRepository(pool)
	ctx := context.Background()

	p := &domain.Product{Name: "Apple", Description: ptr("fresh"), Price: 10, SalePrice: ptr(9.0)}
	require.NoError(t, repo.Create(ctx, p))
	createdUpdatedAt := p.UpdatedAt

	p.Name = "Banana"
	p.Description = nil
	p.SalePrice = nil
	require.NoError(t, repo.Update(ctx, p))

	got, err := repo.GetByIDForUpdate(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "Banana", got.Name)
	assert.Nil(t, got.Description)
	assert.Nil(t, got.SalePrice)
	assert.Equal(t, 10.0, got.Price)
	assert.False(t, got.UpdatedAt.Before(createdUpdatedAt))
}

func TestProductRepository_UpdateNotFound(t *testing.T) {
	testutil.TruncateProducts(t, pool)
	repo := postgres.NewProductRepository(pool)

	err := repo.Update(context.Background(), &domain.Product{ID: 999, Name: "X", Price: 1})
	assert.ErrorIs(t, err, domain.ErrProductNotFound)
}

func TestTransactor_CommitAndRollback(t *testing.T) {
	testutil.TruncateProducts(t, pool)
	repo := postgres.NewProductRepository(pool)
	tx := postgres.NewTransactor(pool)
	ctx := context.Background()

	p := &domain.Product{Name: "Apple", Price: 10}
	require.NoError(t, repo.Create(ctx, p))

	// rollback: change is discarded
	boom := errors.New("boom")
	err := tx.WithinTransaction(ctx, func(ctx context.Context) error {
		got, err := repo.GetByIDForUpdate(ctx, p.ID)
		require.NoError(t, err)
		got.Name = "Rolled back"
		require.NoError(t, repo.Update(ctx, got))
		return boom
	})
	assert.ErrorIs(t, err, boom)
	got, err := repo.GetByIDForUpdate(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "Apple", got.Name)

	// commit: change is persisted
	err = tx.WithinTransaction(ctx, func(ctx context.Context) error {
		got, err := repo.GetByIDForUpdate(ctx, p.ID)
		if err != nil {
			return err
		}
		got.Name = "Committed"
		return repo.Update(ctx, got)
	})
	require.NoError(t, err)
	got, err = repo.GetByIDForUpdate(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "Committed", got.Name)
}
