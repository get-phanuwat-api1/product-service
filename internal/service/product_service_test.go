package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/phanuwat/product-service/internal/apperror"
	"github.com/phanuwat/product-service/internal/mocks"
	"github.com/phanuwat/product-service/internal/model"
	"github.com/phanuwat/product-service/internal/service"
	"github.com/phanuwat/product-service/pkg/optional"
)

func ptr[T any](v T) *T { return &v }

// --- Create ---

func TestCreate_Success(t *testing.T) {
	repo := new(mocks.ProductRepository)
	svc := service.NewProductService(repo, &mocks.Transactor{})

	repo.On("Create", mock.Anything, mock.MatchedBy(func(p *model.Product) bool {
		return p.Name == "Apple" && p.Price == 10 && *p.SalePrice == 8
	})).Run(func(args mock.Arguments) {
		args.Get(1).(*model.Product).ID = 42
	}).Return(nil).Once()

	got, err := svc.Create(context.Background(), service.CreateProductInput{
		Name: "Apple", Price: 10, SalePrice: ptr(8.0),
	})

	require.NoError(t, err)
	assert.Equal(t, int64(42), got.ID)
	repo.AssertExpectations(t)
}

func TestCreate_ValidationErrorDoesNotHitRepo(t *testing.T) {
	repo := new(mocks.ProductRepository)
	svc := service.NewProductService(repo, &mocks.Transactor{})

	_, err := svc.Create(context.Background(), service.CreateProductInput{Name: "", Price: 10})

	assert.Equal(t, apperror.CodeInvalidName, apperror.CodeOf(err))
	repo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

func TestCreate_RepoError(t *testing.T) {
	repo := new(mocks.ProductRepository)
	svc := service.NewProductService(repo, &mocks.Transactor{})
	dbErr := errors.New("db down")
	repo.On("Create", mock.Anything, mock.Anything).Return(dbErr).Once()

	_, err := svc.Create(context.Background(), service.CreateProductInput{Name: "A", Price: 1})

	assert.ErrorIs(t, err, dbErr)
	assert.Equal(t, apperror.CodeInternal, apperror.CodeOf(err))
}

// --- Patch ---

func existing() *model.Product {
	return &model.Product{ID: 1, Name: "Apple", Description: ptr("fresh"), Price: 100, SalePrice: ptr(90.0)}
}

func TestPatch_OverwritesOnlyProvidedFields(t *testing.T) {
	repo := new(mocks.ProductRepository)
	tx := &mocks.Transactor{}
	svc := service.NewProductService(repo, tx)

	repo.On("GetByIDForUpdate", mock.Anything, int64(1)).Return(existing(), nil).Once()
	repo.On("Update", mock.Anything, &model.Product{
		ID: 1, Name: "Banana", Description: nil, Price: 100, SalePrice: ptr(90.0),
	}).Return(nil).Once()

	got, err := svc.Patch(context.Background(), 1, model.ProductPatch{
		Name:        optional.Of("Banana"),
		Description: optional.Null[string](),
	})

	require.NoError(t, err)
	assert.Equal(t, "Banana", got.Name)
	assert.Nil(t, got.Description)
	assert.Equal(t, 1, tx.Calls)
	repo.AssertExpectations(t)
}

func TestPatch_EmptyPatchSkipsUpdate(t *testing.T) {
	repo := new(mocks.ProductRepository)
	svc := service.NewProductService(repo, &mocks.Transactor{})
	repo.On("GetByIDForUpdate", mock.Anything, int64(1)).Return(existing(), nil).Once()

	got, err := svc.Patch(context.Background(), 1, model.ProductPatch{})

	require.NoError(t, err)
	assert.Equal(t, existing(), got)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestPatch_NotFound(t *testing.T) {
	repo := new(mocks.ProductRepository)
	svc := service.NewProductService(repo, &mocks.Transactor{})
	repo.On("GetByIDForUpdate", mock.Anything, int64(99)).Return(nil, apperror.ErrProductNotFound).Once()

	_, err := svc.Patch(context.Background(), 99, model.ProductPatch{Name: optional.Of("X")})

	assert.ErrorIs(t, err, apperror.ErrProductNotFound)
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestPatch_InvalidID(t *testing.T) {
	repo := new(mocks.ProductRepository)
	tx := &mocks.Transactor{}
	svc := service.NewProductService(repo, tx)

	_, err := svc.Patch(context.Background(), 0, model.ProductPatch{})

	assert.Equal(t, apperror.CodeInvalidProductID, apperror.CodeOf(err))
	assert.Equal(t, 0, tx.Calls)
}

func TestPatch_ValidationErrorDoesNotUpdate(t *testing.T) {
	repo := new(mocks.ProductRepository)
	svc := service.NewProductService(repo, &mocks.Transactor{})
	repo.On("GetByIDForUpdate", mock.Anything, int64(1)).Return(existing(), nil).Once()

	_, err := svc.Patch(context.Background(), 1, model.ProductPatch{SalePrice: optional.Of(1000.0)})

	assert.Equal(t, apperror.CodeInvalidSalePrice, apperror.CodeOf(err))
	repo.AssertNotCalled(t, "Update", mock.Anything, mock.Anything)
}

func TestPatch_UpdateError(t *testing.T) {
	repo := new(mocks.ProductRepository)
	svc := service.NewProductService(repo, &mocks.Transactor{})
	dbErr := errors.New("db down")
	repo.On("GetByIDForUpdate", mock.Anything, int64(1)).Return(existing(), nil).Once()
	repo.On("Update", mock.Anything, mock.Anything).Return(dbErr).Once()

	_, err := svc.Patch(context.Background(), 1, model.ProductPatch{Price: optional.Of(200.0)})

	assert.ErrorIs(t, err, dbErr)
}
