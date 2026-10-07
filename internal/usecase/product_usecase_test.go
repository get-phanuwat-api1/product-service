package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/phanuwat/product-service/internal/domain"
	"github.com/phanuwat/product-service/internal/pkg/optional"
	"github.com/phanuwat/product-service/internal/usecase"
)

// --- test doubles ---

type mockRepo struct{ mock.Mock }

func (m *mockRepo) Create(ctx context.Context, p *domain.Product) error {
	return m.Called(ctx, p).Error(0)
}

func (m *mockRepo) GetByIDForUpdate(ctx context.Context, id int64) (*domain.Product, error) {
	args := m.Called(ctx, id)
	p, _ := args.Get(0).(*domain.Product)
	return p, args.Error(1)
}

func (m *mockRepo) Update(ctx context.Context, p *domain.Product) error {
	return m.Called(ctx, p).Error(0)
}

// fakeTx runs fn directly and records whether it was used.
type fakeTx struct{ calls int }

func (f *fakeTx) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	f.calls++
	return fn(ctx)
}

func ptr[T any](v T) *T { return &v }

// --- Create ---

func TestCreate_Success(t *testing.T) {
	repo := new(mockRepo)
	uc := usecase.NewProductUsecase(repo, &fakeTx{})

	repo.On("Create", mock.Anything, mock.MatchedBy(func(p *domain.Product) bool {
		return p.Name == "Apple" && p.Price == 10 && *p.SalePrice == 8
	})).Run(func(args mock.Arguments) {
		args.Get(1).(*domain.Product).ID = 42
	}).Return(nil).Once()

	got, err := uc.Create(context.Background(), usecase.CreateProductInput{
		Name: "Apple", Price: 10, SalePrice: ptr(8.0),
	})

	require.NoError(t, err)
	assert.Equal(t, int64(42), got.ID)
	repo.AssertExpectations(t)
}

func TestCreate_ValidationErrorDoesNotHitRepo(t *testing.T) {
	repo := new(mockRepo)
	uc := usecase.NewProductUsecase(repo, &fakeTx{})

	_, err := uc.Create(context.Background(), usecase.CreateProductInput{Name: "", Price: 10})

	assert.Equal(t, domain.ErrCodeInvalidName, domain.CodeOf(err))
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreate_RepoError(t *testing.T) {
	repo := new(mockRepo)
	uc := usecase.NewProductUsecase(repo, &fakeTx{})
	dbErr := errors.New("db down")
	repo.On("Create", mock.Anything, mock.Anything).Return(dbErr).Once()

	_, err := uc.Create(context.Background(), usecase.CreateProductInput{Name: "A", Price: 1})

	assert.ErrorIs(t, err, dbErr)
	assert.Equal(t, domain.ErrCodeInternal, domain.CodeOf(err))
}

// --- Patch ---

func existing() *domain.Product {
	return &domain.Product{ID: 1, Name: "Apple", Description: ptr("fresh"), Price: 100, SalePrice: ptr(90.0)}
}

func TestPatch_OverwritesOnlyProvidedFields(t *testing.T) {
	repo := new(mockRepo)
	tx := &fakeTx{}
	uc := usecase.NewProductUsecase(repo, tx)

	repo.On("GetByIDForUpdate", mock.Anything, int64(1)).Return(existing(), nil).Once()
	repo.On("Update", mock.Anything, &domain.Product{
		ID: 1, Name: "Banana", Description: nil, Price: 100, SalePrice: ptr(90.0),
	}).Return(nil).Once()

	got, err := uc.Patch(context.Background(), 1, domain.ProductPatch{
		Name:        optional.Of("Banana"),
		Description: optional.Null[string](),
	})

	require.NoError(t, err)
	assert.Equal(t, "Banana", got.Name)
	assert.Nil(t, got.Description)
	assert.Equal(t, 1, tx.calls)
	repo.AssertExpectations(t)
}

func TestPatch_EmptyPatchSkipsUpdate(t *testing.T) {
	repo := new(mockRepo)
	uc := usecase.NewProductUsecase(repo, &fakeTx{})
	repo.On("GetByIDForUpdate", mock.Anything, int64(1)).Return(existing(), nil).Once()

	got, err := uc.Patch(context.Background(), 1, domain.ProductPatch{})

	require.NoError(t, err)
	assert.Equal(t, existing(), got)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestPatch_NotFound(t *testing.T) {
	repo := new(mockRepo)
	uc := usecase.NewProductUsecase(repo, &fakeTx{})
	repo.On("GetByIDForUpdate", mock.Anything, int64(99)).Return(nil, domain.ErrProductNotFound).Once()

	_, err := uc.Patch(context.Background(), 99, domain.ProductPatch{Name: optional.Of("X")})

	assert.ErrorIs(t, err, domain.ErrProductNotFound)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestPatch_InvalidID(t *testing.T) {
	repo := new(mockRepo)
	tx := &fakeTx{}
	uc := usecase.NewProductUsecase(repo, tx)

	_, err := uc.Patch(context.Background(), 0, domain.ProductPatch{})

	assert.Equal(t, domain.ErrCodeInvalidProductID, domain.CodeOf(err))
	assert.Equal(t, 0, tx.calls)
}

func TestPatch_ValidationErrorDoesNotUpdate(t *testing.T) {
	repo := new(mockRepo)
	uc := usecase.NewProductUsecase(repo, &fakeTx{})
	repo.On("GetByIDForUpdate", mock.Anything, int64(1)).Return(existing(), nil).Once()

	_, err := uc.Patch(context.Background(), 1, domain.ProductPatch{SalePrice: optional.Of(1000.0)})

	assert.Equal(t, domain.ErrCodeInvalidSalePrice, domain.CodeOf(err))
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestPatch_UpdateError(t *testing.T) {
	repo := new(mockRepo)
	uc := usecase.NewProductUsecase(repo, &fakeTx{})
	dbErr := errors.New("db down")
	repo.On("GetByIDForUpdate", mock.Anything, int64(1)).Return(existing(), nil).Once()
	repo.On("Update", mock.Anything, mock.Anything).Return(dbErr).Once()

	_, err := uc.Patch(context.Background(), 1, domain.ProductPatch{Price: optional.Of(200.0)})

	assert.ErrorIs(t, err, dbErr)
}
