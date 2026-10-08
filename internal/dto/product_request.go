// Package dto holds HTTP request/response shapes (Data Transfer Objects).
package dto

import (
	"github.com/get-phanuwat-api1/product-service/internal/model"
	"github.com/get-phanuwat-api1/product-service/pkg/optional"
)

// CreateProductRequest is the body of POST /product.
type CreateProductRequest struct {
	Name        *string  `json:"name" example:"iPhone 17" extensions:"x-nullable=false"`
	Description *string  `json:"description" example:"Latest model" extensions:"x-nullable"`
	SalePrice   *float64 `json:"sale_price" example:"39900" extensions:"x-nullable"`
	Price       *float64 `json:"price" example:"42900" extensions:"x-nullable=false"`
}

// PatchProductRequest is the body of PATCH /product/{id}.
// Omitted keys are left untouched; explicit null clears nullable fields.
type PatchProductRequest struct {
	Name        optional.Field[string]  `json:"name" swaggertype:"string" example:"iPhone 17 Pro"`
	Description optional.Field[string]  `json:"description" swaggertype:"string" example:"Updated description" extensions:"x-nullable"`
	SalePrice   optional.Field[float64] `json:"sale_price" swaggertype:"number" example:"38900" extensions:"x-nullable"`
	Price       optional.Field[float64] `json:"price" swaggertype:"number" example:"41900"`
}

// ToModel converts the request into a model.ProductPatch.
func (r PatchProductRequest) ToModel() model.ProductPatch {
	return model.ProductPatch{
		Name:        r.Name,
		Description: r.Description,
		Price:       r.Price,
		SalePrice:   r.SalePrice,
	}
}
