package handler_test

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/get-phanuwat-api1/product-service/internal/apperror"
	"github.com/get-phanuwat-api1/product-service/internal/handler"
	"github.com/get-phanuwat-api1/product-service/internal/mocks"
	"github.com/get-phanuwat-api1/product-service/internal/model"
	"github.com/get-phanuwat-api1/product-service/internal/service"
	"github.com/get-phanuwat-api1/product-service/pkg/logger"
	"github.com/get-phanuwat-api1/product-service/pkg/optional"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode) // global: set once, before parallel tests start
	os.Exit(m.Run())
}

func setup(svc *mocks.ProductService) *gin.Engine {
	h := handler.NewProductHandler(svc, logger.NewDiscardLogger())
	r := gin.New()
	r.POST("/product", h.CreateProduct)
	r.PATCH("/product/:id", h.PatchProductByID)
	return r
}

func send(r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, bytes.NewBufferString(body)))
	return w
}

func ptr[T any](v T) *T { return &v }

func TestCreateProduct_MapsRequestToService(t *testing.T) {
	t.Parallel()
	svc := mocks.NewProductService(t)
	svc.EXPECT().
		CreateProduct(mock.Anything, service.CreateProductInput{Name: "Apple", Description: nil, Price: 10, SalePrice: ptr(8.0)}).
		Return(&model.Product{ID: 7, Name: "Apple", Price: 10, SalePrice: ptr(8.0)}, nil).Once()

	w := send(setup(svc), http.MethodPost, "/product", `{"name":"Apple","description":null,"sale_price":8,"price":10}`)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.JSONEq(t, `{"successful":true,"error_code":"","data":{
		"id":7,"name":"Apple","description":null,"price":10,"sale_price":8,
		"created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z"}}`, w.Body.String())
}

func TestCreateProduct_Errors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		body       string
		svcErr     error
		wantStatus int
		wantCode   string
	}{
		{"malformed json", `{`, nil, http.StatusBadRequest, "INVALID_REQUEST_BODY"},
		{"two json objects", `{"name":"A","price":1}{}`, nil, http.StatusBadRequest, "INVALID_REQUEST_BODY"},
		{"missing name", `{"price":1}`, nil, http.StatusBadRequest, "INVALID_NAME"},
		{"missing price", `{"name":"A"}`, nil, http.StatusBadRequest, "INVALID_PRICE"},
		{"service validation", `{"name":"A","price":-1}`, apperror.New(apperror.CodeInvalidPrice, "x"), http.StatusBadRequest, "INVALID_PRICE"},
		{"unexpected error", `{"name":"A","price":1}`, errors.New("db down"), http.StatusInternalServerError, "INTERNAL_SERVER_ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := mocks.NewProductService(t)
			if tt.svcErr != nil {
				svc.EXPECT().CreateProduct(mock.Anything, mock.Anything).Return(nil, tt.svcErr).Once()
			}

			w := send(setup(svc), http.MethodPost, "/product", tt.body)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.JSONEq(t, `{"successful":false,"error_code":"`+tt.wantCode+`","data":null}`, w.Body.String())
		})
	}
}

func TestPatchProductByID_MapsUndefinedAndNull(t *testing.T) {
	t.Parallel()
	svc := mocks.NewProductService(t)
	svc.EXPECT().
		PatchProductByID(mock.Anything, int64(5), model.ProductPatch{
			Name:        optional.Of("Banana"),
			Description: optional.Null[string](),
		}).
		Return(&model.Product{}, nil).Once()

	w := send(setup(svc), http.MethodPatch, "/product/5", `{"name":"Banana","description":null}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"successful":true,"error_code":""}`, w.Body.String())
}

func TestPatchProductByID_Errors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		path       string
		body       string
		svcErr     error
		wantStatus int
		wantCode   string
	}{
		{"invalid id", "/product/abc", `{}`, nil, http.StatusBadRequest, "INVALID_PRODUCT_ID"},
		{"negative id", "/product/-1", `{}`, nil, http.StatusBadRequest, "INVALID_PRODUCT_ID"},
		{"malformed json", "/product/1", `{"name"`, nil, http.StatusBadRequest, "INVALID_REQUEST_BODY"},
		{"not found", "/product/1", `{}`, apperror.ErrProductNotFound, http.StatusNotFound, "PRODUCT_NOT_FOUND"},
		{"unexpected error", "/product/1", `{}`, errors.New("db down"), http.StatusInternalServerError, "INTERNAL_SERVER_ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc := mocks.NewProductService(t)
			if tt.svcErr != nil {
				svc.EXPECT().PatchProductByID(mock.Anything, mock.Anything, mock.Anything).Return(nil, tt.svcErr).Once()
			}

			w := send(setup(svc), http.MethodPatch, tt.path, tt.body)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.JSONEq(t, `{"successful":false,"error_code":"`+tt.wantCode+`"}`, w.Body.String())
		})
	}
}
