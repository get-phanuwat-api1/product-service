package dto

import (
	"time"

	"github.com/phanuwat/product-service/internal/model"
)

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

// NewProductData maps a model.Product to its response representation.
func NewProductData(p *model.Product) *ProductData {
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

// CreateProductResponse is the response of POST /product. Data is null on error.
type CreateProductResponse struct {
	Successful bool         `json:"successful" example:"true"`
	ErrorCode  string       `json:"error_code" example:""`
	Data       *ProductData `json:"data" extensions:"x-nullable"`
}

// PatchProductResponse is the response of PATCH /product/{id}.
type PatchProductResponse struct {
	Successful bool   `json:"successful" example:"true"`
	ErrorCode  string `json:"error_code" example:""`
}
