package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/get-phanuwat-api1/product-service/internal/apperror"
	"github.com/get-phanuwat-api1/product-service/internal/mocks"
	"github.com/get-phanuwat-api1/product-service/internal/model"
	"github.com/get-phanuwat-api1/product-service/internal/service"
	"github.com/get-phanuwat-api1/product-service/pkg/logger"
	"github.com/get-phanuwat-api1/product-service/pkg/optional"
	"github.com/get-phanuwat-api1/product-service/pkg/requestctx"
)

func ptr[T any](v T) *T { return &v }

type deps struct {
	productRepo  *mocks.ProductRepository
	auditLogRepo *mocks.AuditLogRepository
	tx           *mocks.FakeTransactor
	svc          service.ProductService
}

// newDeps builds the service with generated mocks; unmet expectations fail the test automatically.
func newDeps(t *testing.T) deps {
	d := deps{
		productRepo:  mocks.NewProductRepository(t),
		auditLogRepo: mocks.NewAuditLogRepository(t),
		tx:           &mocks.FakeTransactor{},
	}
	d.svc = service.NewProductService(d.productRepo, d.auditLogRepo, d.tx, logger.NewDiscardLogger())
	return d
}

func requestCtx() context.Context {
	return requestctx.WithActor(requestctx.WithRequestID(context.Background(), "req-1"), "user-1")
}

// --- CreateProduct ---

func TestCreateProduct_StoresProductAndAuditLog(t *testing.T) {
	t.Parallel()
	d := newDeps(t)

	d.productRepo.EXPECT().
		CreateProduct(mock.Anything, mock.MatchedBy(func(p *model.Product) bool {
			return p.Name == "Apple" && p.Price == 10 && *p.SalePrice == 8
		})).
		Run(func(_ context.Context, p *model.Product) { p.ID = 42 }).
		Return(nil).Once()

	d.auditLogRepo.EXPECT().
		CreateAuditLog(mock.Anything, mock.MatchedBy(func(l *model.AuditLog) bool {
			return l.Action == model.AuditActionProductCreated &&
				l.EntityType == "product" && l.EntityID == 42 &&
				l.Actor == "user-1" && l.RequestID == "req-1" &&
				l.BeforeData == nil &&
				string(l.AfterData) == `{"id":42,"name":"Apple","description":null,"price":10,"sale_price":8}`
		})).
		Return(nil).Once()

	got, err := d.svc.CreateProduct(requestCtx(), service.CreateProductInput{
		Name: "Apple", Price: 10, SalePrice: ptr(8.0),
	})

	require.NoError(t, err)
	assert.Equal(t, int64(42), got.ID)
	assert.Equal(t, 1, d.tx.Calls)
}

func TestCreateProduct_UsesSystemActorWithoutRequestContext(t *testing.T) {
	t.Parallel()
	d := newDeps(t)
	d.productRepo.EXPECT().CreateProduct(mock.Anything, mock.Anything).Return(nil).Once()
	d.auditLogRepo.EXPECT().
		CreateAuditLog(mock.Anything, mock.MatchedBy(func(l *model.AuditLog) bool {
			return l.Actor == service.SystemActor && l.RequestID == ""
		})).
		Return(nil).Once()

	_, err := d.svc.CreateProduct(context.Background(), service.CreateProductInput{Name: "A", Price: 1})

	require.NoError(t, err)
}

func TestCreateProduct_ValidationErrorDoesNotHitRepo(t *testing.T) {
	t.Parallel()
	d := newDeps(t) // no expectations: any repo call fails the test

	_, err := d.svc.CreateProduct(requestCtx(), service.CreateProductInput{Name: "", Price: 10})

	assert.Equal(t, apperror.CodeInvalidName, apperror.CodeOf(err))
	assert.Equal(t, 0, d.tx.Calls)
}

func TestCreateProduct_RepoErrorSkipsAudit(t *testing.T) {
	t.Parallel()
	d := newDeps(t)
	dbErr := errors.New("db down")
	d.productRepo.EXPECT().CreateProduct(mock.Anything, mock.Anything).Return(dbErr).Once()

	_, err := d.svc.CreateProduct(requestCtx(), service.CreateProductInput{Name: "A", Price: 1})

	assert.ErrorIs(t, err, dbErr)
	assert.Equal(t, apperror.CodeInternal, apperror.CodeOf(err))
}

func TestCreateProduct_AuditErrorFailsTheRequest(t *testing.T) {
	t.Parallel()
	d := newDeps(t)
	auditErr := errors.New("audit insert failed")
	d.productRepo.EXPECT().CreateProduct(mock.Anything, mock.Anything).Return(nil).Once()
	d.auditLogRepo.EXPECT().CreateAuditLog(mock.Anything, mock.Anything).Return(auditErr).Once()

	_, err := d.svc.CreateProduct(requestCtx(), service.CreateProductInput{Name: "A", Price: 1})

	assert.ErrorIs(t, err, auditErr) // the real Transactor rolls back on this error
}

// --- PatchProductByID ---

func existing() *model.Product {
	return &model.Product{ID: 1, Name: "Apple", Description: ptr("fresh"), Price: 100, SalePrice: ptr(90.0)}
}

func TestPatchProductByID_OverwritesOnlyProvidedFieldsAndAudits(t *testing.T) {
	t.Parallel()
	d := newDeps(t)

	d.productRepo.EXPECT().GetProductByIDForUpdate(mock.Anything, int64(1)).Return(existing(), nil).Once()
	d.productRepo.EXPECT().
		UpdateProduct(mock.Anything, &model.Product{ID: 1, Name: "Banana", Description: nil, Price: 100, SalePrice: ptr(90.0)}).
		Return(nil).Once()
	d.auditLogRepo.EXPECT().
		CreateAuditLog(mock.Anything, mock.MatchedBy(func(l *model.AuditLog) bool {
			return l.Action == model.AuditActionProductUpdated && l.EntityID == 1 &&
				string(l.BeforeData) == `{"id":1,"name":"Apple","description":"fresh","price":100,"sale_price":90}` &&
				string(l.AfterData) == `{"id":1,"name":"Banana","description":null,"price":100,"sale_price":90}`
		})).
		Return(nil).Once()

	got, err := d.svc.PatchProductByID(requestCtx(), 1, model.ProductPatch{
		Name:        optional.Of("Banana"),
		Description: optional.Null[string](),
	})

	require.NoError(t, err)
	assert.Equal(t, "Banana", got.Name)
	assert.Nil(t, got.Description)
	assert.Equal(t, 1, d.tx.Calls)
}

func TestPatchProductByID_NoOpPatchSkipsUpdateAndAudit(t *testing.T) {
	t.Parallel()
	for name, patch := range map[string]model.ProductPatch{
		"empty body":     {},
		"same values":    {Name: optional.Of("Apple"), Price: optional.Of(100.0)},
		"same nullables": {Description: optional.Of("fresh"), SalePrice: optional.Of(90.0)},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			d := newDeps(t) // no UpdateProduct / CreateAuditLog expectations
			d.productRepo.EXPECT().GetProductByIDForUpdate(mock.Anything, int64(1)).Return(existing(), nil).Once()

			got, err := d.svc.PatchProductByID(requestCtx(), 1, patch)

			require.NoError(t, err)
			assert.Equal(t, existing(), got)
		})
	}
}

func TestPatchProductByID_NotFound(t *testing.T) {
	t.Parallel()
	d := newDeps(t)
	d.productRepo.EXPECT().GetProductByIDForUpdate(mock.Anything, int64(99)).Return(nil, apperror.ErrProductNotFound).Once()

	_, err := d.svc.PatchProductByID(requestCtx(), 99, model.ProductPatch{Name: optional.Of("X")})

	assert.ErrorIs(t, err, apperror.ErrProductNotFound)
}

func TestPatchProductByID_InvalidID(t *testing.T) {
	t.Parallel()
	d := newDeps(t)

	_, err := d.svc.PatchProductByID(requestCtx(), 0, model.ProductPatch{})

	assert.Equal(t, apperror.CodeInvalidProductID, apperror.CodeOf(err))
	assert.Equal(t, 0, d.tx.Calls)
}

func TestPatchProductByID_ValidationErrorDoesNotUpdate(t *testing.T) {
	t.Parallel()
	d := newDeps(t)
	d.productRepo.EXPECT().GetProductByIDForUpdate(mock.Anything, int64(1)).Return(existing(), nil).Once()

	_, err := d.svc.PatchProductByID(requestCtx(), 1, model.ProductPatch{SalePrice: optional.Of(1000.0)})

	assert.Equal(t, apperror.CodeInvalidSalePrice, apperror.CodeOf(err))
}

func TestPatchProductByID_UpdateErrorSkipsAudit(t *testing.T) {
	t.Parallel()
	d := newDeps(t)
	dbErr := errors.New("db down")
	d.productRepo.EXPECT().GetProductByIDForUpdate(mock.Anything, int64(1)).Return(existing(), nil).Once()
	d.productRepo.EXPECT().UpdateProduct(mock.Anything, mock.Anything).Return(dbErr).Once()

	_, err := d.svc.PatchProductByID(requestCtx(), 1, model.ProductPatch{Price: optional.Of(200.0)})

	assert.ErrorIs(t, err, dbErr)
}

func TestPatchProductByID_AuditErrorFailsTheRequest(t *testing.T) {
	t.Parallel()
	d := newDeps(t)
	auditErr := errors.New("audit insert failed")
	d.productRepo.EXPECT().GetProductByIDForUpdate(mock.Anything, int64(1)).Return(existing(), nil).Once()
	d.productRepo.EXPECT().UpdateProduct(mock.Anything, mock.Anything).Return(nil).Once()
	d.auditLogRepo.EXPECT().CreateAuditLog(mock.Anything, mock.Anything).Return(auditErr).Once()

	_, err := d.svc.PatchProductByID(requestCtx(), 1, model.ProductPatch{Price: optional.Of(200.0)})

	assert.ErrorIs(t, err, auditErr)
}
