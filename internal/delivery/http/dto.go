package http

import (
	"time"

	"github.com/phanuwat/product-service/internal/domain"
	"github.com/phanuwat/product-service/internal/pkg/optional"
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

func (r PatchProductRequest) toDomain() domain.ProductPatch {
	return domain.ProductPatch{
		Name:        r.Name,
		Description: r.Description,
		Price:       r.Price,
		SalePrice:   r.SalePrice,
	}
}

// ProductData is the product representation returned in responses.
type ProductData struct {
	ID          int64     `json:"id" example:"1"`
	Name        string    `json:"name" example:"iPhone 17"`
	Description *string   `json:"description" example:"Latest model" extensions:"x-nullable"`
	Price       float64   `json:"price" example:"42900"`
	SalePrice   *float64  `json:"sale_price" example:"39900" extensions:"x-nullable"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toProductData(p *domain.Product) *ProductData {
	return &ProductData{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		SalePrice:   p.SalePrice,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

// CreateProductResponse is the response of POST /product.
type CreateProductResponse struct {
	Successful bool         `json:"successful" example:"true"`
	ErrorCode  string       `json:"error_code" example:""`
	Data       *ProductData `json:"data" extensions:"x-nullable"`
}

// BaseResponse is the response of PATCH /product/{id} and of any error.
type BaseResponse struct {
	Successful bool   `json:"successful" example:"true"`
	ErrorCode  string `json:"error_code" example:""`
}

// ErrorResponse documents the error shape for create (data is always null).
type ErrorResponse struct {
	Successful bool    `json:"successful" example:"false"`
	ErrorCode  string  `json:"error_code" example:"INVALID_PRICE"`
	Data       *string `json:"data,omitempty" extensions:"x-nullable"`
}
