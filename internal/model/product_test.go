package model_test

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/get-phanuwat-api1/product-service/internal/apperror"
	"github.com/get-phanuwat-api1/product-service/internal/model"
	"github.com/get-phanuwat-api1/product-service/pkg/optional"
)

func ptr[T any](v T) *T { return &v }

func TestNewProduct(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		inName      string
		desc        *string
		price       float64
		salePrice   *float64
		wantErrCode apperror.Code
	}{
		{name: "valid minimal", inName: "Apple", price: 10},
		{name: "valid full", inName: "Apple", desc: ptr("fresh"), price: 10, salePrice: ptr(8.5)},
		{name: "valid zero price", inName: "Free", price: 0},
		{name: "sale price equals price", inName: "A", price: 10, salePrice: ptr(10.0)},
		{name: "empty name", inName: "", price: 10, wantErrCode: apperror.CodeInvalidName},
		{name: "blank name", inName: "   ", price: 10, wantErrCode: apperror.CodeInvalidName},
		{name: "name too long", inName: strings.Repeat("a", 256), price: 10, wantErrCode: apperror.CodeInvalidName},
		{name: "negative price", inName: "A", price: -1, wantErrCode: apperror.CodeInvalidPrice},
		{name: "NaN price", inName: "A", price: math.NaN(), wantErrCode: apperror.CodeInvalidPrice},
		{name: "negative sale price", inName: "A", price: 10, salePrice: ptr(-1.0), wantErrCode: apperror.CodeInvalidSalePrice},
		{name: "sale price above price", inName: "A", price: 10, salePrice: ptr(11.0), wantErrCode: apperror.CodeInvalidSalePrice},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := model.NewProduct(tt.inName, tt.desc, tt.price, tt.salePrice)
			if tt.wantErrCode != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErrCode, apperror.CodeOf(err))
				assert.Nil(t, p)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, strings.TrimSpace(tt.inName), p.Name)
			assert.Equal(t, tt.desc, p.Description)
			assert.Equal(t, tt.price, p.Price)
			assert.Equal(t, tt.salePrice, p.SalePrice)
		})
	}
}

func TestNewProduct_TrimsName(t *testing.T) {
	t.Parallel()
	p, err := model.NewProduct("  Apple  ", nil, 1, nil)
	require.NoError(t, err)
	assert.Equal(t, "Apple", p.Name)
}

func baseProduct() *model.Product {
	return &model.Product{ID: 1, Name: "Apple", Description: ptr("fresh"), Price: 100, SalePrice: ptr(90.0)}
}

func TestProduct_ApplyPatch(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		patch       model.ProductPatch
		want        *model.Product
		wantErrCode apperror.Code
	}{
		{
			name:  "empty patch keeps everything",
			patch: model.ProductPatch{},
			want:  baseProduct(),
		},
		{
			name:  "only name is overwritten",
			patch: model.ProductPatch{Name: optional.Of("Banana")},
			want:  &model.Product{ID: 1, Name: "Banana", Description: ptr("fresh"), Price: 100, SalePrice: ptr(90.0)},
		},
		{
			name:  "description set to null",
			patch: model.ProductPatch{Description: optional.Null[string]()},
			want:  &model.Product{ID: 1, Name: "Apple", Description: nil, Price: 100, SalePrice: ptr(90.0)},
		},
		{
			name:  "sale price set to null",
			patch: model.ProductPatch{SalePrice: optional.Null[float64]()},
			want:  &model.Product{ID: 1, Name: "Apple", Description: ptr("fresh"), Price: 100, SalePrice: nil},
		},
		{
			name:  "price and sale price together",
			patch: model.ProductPatch{Price: optional.Of(50.0), SalePrice: optional.Of(40.0)},
			want:  &model.Product{ID: 1, Name: "Apple", Description: ptr("fresh"), Price: 50, SalePrice: ptr(40.0)},
		},
		{
			name:        "name null is rejected",
			patch:       model.ProductPatch{Name: optional.Null[string]()},
			wantErrCode: apperror.CodeInvalidName,
		},
		{
			name:        "name blank is rejected",
			patch:       model.ProductPatch{Name: optional.Of("  ")},
			wantErrCode: apperror.CodeInvalidName,
		},
		{
			name:        "price null is rejected",
			patch:       model.ProductPatch{Price: optional.Null[float64]()},
			wantErrCode: apperror.CodeInvalidPrice,
		},
		{
			name:        "price below existing sale price is rejected",
			patch:       model.ProductPatch{Price: optional.Of(80.0)},
			wantErrCode: apperror.CodeInvalidSalePrice,
		},
		{
			name:        "negative sale price is rejected",
			patch:       model.ProductPatch{SalePrice: optional.Of(-5.0)},
			wantErrCode: apperror.CodeInvalidSalePrice,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p := baseProduct()
			err := p.ApplyPatch(tt.patch)
			if tt.wantErrCode != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErrCode, apperror.CodeOf(err))
				assert.Equal(t, baseProduct(), p, "product must be unchanged on error")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, p)
		})
	}
}

func TestProductPatch_IsEmpty(t *testing.T) {
	t.Parallel()
	assert.True(t, model.ProductPatch{}.IsEmpty())
	assert.False(t, model.ProductPatch{Description: optional.Null[string]()}.IsEmpty())
}
