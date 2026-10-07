package handler_test

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/phanuwat/product-service/internal/apperror"
	"github.com/phanuwat/product-service/internal/handler"
	"github.com/phanuwat/product-service/internal/mocks"
	"github.com/phanuwat/product-service/internal/model"
	"github.com/phanuwat/product-service/internal/service"
	"github.com/phanuwat/product-service/pkg/optional"
)

func setup(svc *mocks.ProductService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := handler.NewProductHandler(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
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
	svc := new(mocks.ProductService)
	svc.On("CreateProduct", mock.Anything, service.CreateProductInput{
		Name: "Apple", Description: nil, Price: 10, SalePrice: ptr(8.0),
	}).Return(&model.Product{ID: 7, Name: "Apple", Price: 10, SalePrice: ptr(8.0)}, nil).Once()

	w := send(setup(svc), http.MethodPost, "/product", `{"name":"Apple","description":null,"sale_price":8,"price":10}`)

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.JSONEq(t, `{"successful":true,"error_code":"","data":{
		"id":7,"name":"Apple","description":null,"price":10,"sale_price":8,
		"created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z"}}`, w.Body.String())
	svc.AssertExpectations(t)
}

func TestCreateProduct_Errors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		svcErr     error
		wantStatus int
		wantCode   string
	}{
		{"malformed json", `{`, nil, http.StatusBadRequest, "INVALID_REQUEST_BODY"},
		{"missing name", `{"price":1}`, nil, http.StatusBadRequest, "INVALID_NAME"},
		{"missing price", `{"name":"A"}`, nil, http.StatusBadRequest, "INVALID_PRICE"},
		{"service validation", `{"name":"A","price":-1}`, apperror.New(apperror.CodeInvalidPrice, "x"), http.StatusBadRequest, "INVALID_PRICE"},
		{"unexpected error", `{"name":"A","price":1}`, errors.New("db down"), http.StatusInternalServerError, "INTERNAL_SERVER_ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := new(mocks.ProductService)
			if tt.svcErr != nil {
				svc.On("CreateProduct", mock.Anything, mock.Anything).Return(nil, tt.svcErr).Once()
			}

			w := send(setup(svc), http.MethodPost, "/product", tt.body)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.JSONEq(t, `{"successful":false,"error_code":"`+tt.wantCode+`","data":null}`, w.Body.String())
			svc.AssertExpectations(t)
		})
	}
}

func TestPatchProductByID_MapsUndefinedAndNull(t *testing.T) {
	svc := new(mocks.ProductService)
	svc.On("PatchProductByID", mock.Anything, int64(5), model.ProductPatch{
		Name:        optional.Of("Banana"),
		Description: optional.Null[string](),
	}).Return(&model.Product{}, nil).Once()

	w := send(setup(svc), http.MethodPatch, "/product/5", `{"name":"Banana","description":null}`)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"successful":true,"error_code":""}`, w.Body.String())
	svc.AssertExpectations(t)
}

func TestPatchProductByID_Errors(t *testing.T) {
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
			svc := new(mocks.ProductService)
			if tt.svcErr != nil {
				svc.On("PatchProductByID", mock.Anything, mock.Anything, mock.Anything).Return(nil, tt.svcErr).Once()
			}

			w := send(setup(svc), http.MethodPatch, tt.path, tt.body)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.JSONEq(t, `{"successful":false,"error_code":"`+tt.wantCode+`"}`, w.Body.String())
			svc.AssertExpectations(t)
		})
	}
}
