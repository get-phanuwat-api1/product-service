package domain_test

import (
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phanuwat/product-service/internal/domain"
	"github.com/phanuwat/product-service/internal/pkg/optional"
)

func ptr[T any](v T) *T { return &v }

func TestNewProduct(t *testing.T) {
	tests := []struct {
		name        string
		inName      string
		desc        *string
		price       float64
		salePrice   *float64
		wantErrCode domain.ErrorCode
	}{
		{name: "valid minimal", inName: "Apple", price: 10},
		{name: "valid full", inName: "Apple", desc: ptr("fresh"), price: 10, salePrice: ptr(8.5)},
		{name: "valid zero price", inName: "Free", price: 0},
		{name: "sale price equals price", inName: "A", price: 10, salePrice: ptr(10.0)},
		{name: "empty name", inName: "", price: 10, wantErrCode: domain.ErrCodeInvalidName},
		{name: "blank name", inName: "   ", price: 10, wantErrCode: domain.ErrCodeInvalidName},
		{name: "name too long", inName: strings.Repeat("a", 256), price: 10, wantErrCode: domain.ErrCodeInvalidName},
		{name: "negative price", inName: "A", price: -1, wantErrCode: domain.ErrCodeInvalidPrice},
		{name: "NaN price", inName: "A", price: math.NaN(), wantErrCode: domain.ErrCodeInvalidPrice},
		{name: "negative sale price", inName: "A", price: 10, salePrice: ptr(-1.0), wantErrCode: domain.ErrCodeInvalidSalePrice},
		{name: "sale price above price", inName: "A", price: 10, salePrice: ptr(11.0), wantErrCode: domain.ErrCodeInvalidSalePrice},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := domain.NewProduct(tt.inName, tt.desc, tt.price, tt.salePrice)
			if tt.wantErrCode != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErrCode, domain.CodeOf(err))
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
	p, err := domain.NewProduct("  Apple  ", nil, 1, nil)
	require.NoError(t, err)
	assert.Equal(t, "Apple", p.Name)
}

func baseProduct() *domain.Product {
	return &domain.Product{ID: 1, Name: "Apple", Description: ptr("fresh"), Price: 100, SalePrice: ptr(90.0)}
}

func TestProduct_ApplyPatch(t *testing.T) {
	tests := []struct {
		name        string
		patch       domain.ProductPatch
		want        *domain.Product
		wantErrCode domain.ErrorCode
	}{
		{
			name:  "empty patch keeps everything",
			patch: domain.ProductPatch{},
			want:  baseProduct(),
		},
		{
			name:  "only name is overwritten",
			patch: domain.ProductPatch{Name: optional.Of("Banana")},
			want:  &domain.Product{ID: 1, Name: "Banana", Description: ptr("fresh"), Price: 100, SalePrice: ptr(90.0)},
		},
		{
			name:  "description set to null",
			patch: domain.ProductPatch{Description: optional.Null[string]()},
			want:  &domain.Product{ID: 1, Name: "Apple", Description: nil, Price: 100, SalePrice: ptr(90.0)},
		},
		{
			name:  "sale price set to null",
			patch: domain.ProductPatch{SalePrice: optional.Null[float64]()},
			want:  &domain.Product{ID: 1, Name: "Apple", Description: ptr("fresh"), Price: 100, SalePrice: nil},
		},
		{
			name:  "price and sale price together",
			patch: domain.ProductPatch{Price: optional.Of(50.0), SalePrice: optional.Of(40.0)},
			want:  &domain.Product{ID: 1, Name: "Apple", Description: ptr("fresh"), Price: 50, SalePrice: ptr(40.0)},
		},
		{
			name:        "name null is rejected",
			patch:       domain.ProductPatch{Name: optional.Null[string]()},
			wantErrCode: domain.ErrCodeInvalidName,
		},
		{
			name:        "name blank is rejected",
			patch:       domain.ProductPatch{Name: optional.Of("  ")},
			wantErrCode: domain.ErrCodeInvalidName,
		},
		{
			name:        "price null is rejected",
			patch:       domain.ProductPatch{Price: optional.Null[float64]()},
			wantErrCode: domain.ErrCodeInvalidPrice,
		},
		{
			name:        "price below existing sale price is rejected",
			patch:       domain.ProductPatch{Price: optional.Of(80.0)},
			wantErrCode: domain.ErrCodeInvalidSalePrice,
		},
		{
			name:        "negative sale price is rejected",
			patch:       domain.ProductPatch{SalePrice: optional.Of(-5.0)},
			wantErrCode: domain.ErrCodeInvalidSalePrice,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := baseProduct()
			err := p.ApplyPatch(tt.patch)
			if tt.wantErrCode != "" {
				require.Error(t, err)
				assert.Equal(t, tt.wantErrCode, domain.CodeOf(err))
				assert.Equal(t, baseProduct(), p, "product must be unchanged on error")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, p)
		})
	}
}

func TestProductPatch_IsEmpty(t *testing.T) {
	assert.True(t, domain.ProductPatch{}.IsEmpty())
	assert.False(t, domain.ProductPatch{Description: optional.Null[string]()}.IsEmpty())
}

func TestErrorIs(t *testing.T) {
	err := domain.NewError(domain.ErrCodeProductNotFound, "other message")
	assert.ErrorIs(t, err, domain.ErrProductNotFound)
	assert.NotErrorIs(t, domain.NewError(domain.ErrCodeInvalidName, "x"), domain.ErrProductNotFound)
}
