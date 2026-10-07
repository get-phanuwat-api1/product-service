package domain

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/phanuwat/product-service/internal/pkg/optional"
)

const MaxNameLength = 255

// Product is the core entity.
type Product struct {
	ID          int64
	Name        string
	Description *string
	Price       float64
	SalePrice   *float64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewProduct builds a validated Product.
func NewProduct(name string, description *string, price float64, salePrice *float64) (*Product, error) {
	p := &Product{
		Name:        strings.TrimSpace(name),
		Description: description,
		Price:       price,
		SalePrice:   salePrice,
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}

// Validate enforces product business rules.
func (p *Product) Validate() error {
	if p.Name == "" {
		return NewError(ErrCodeInvalidName, "name is required")
	}
	if len([]rune(p.Name)) > MaxNameLength {
		return NewError(ErrCodeInvalidName, "name must not exceed 255 characters")
	}
	if !isFinite(p.Price) || p.Price < 0 {
		return NewError(ErrCodeInvalidPrice, "price must be a non-negative number")
	}
	if p.SalePrice != nil {
		if !isFinite(*p.SalePrice) || *p.SalePrice < 0 {
			return NewError(ErrCodeInvalidSalePrice, "sale_price must be a non-negative number")
		}
		if *p.SalePrice > p.Price {
			return NewError(ErrCodeInvalidSalePrice, "sale_price must not be greater than price")
		}
	}
	return nil
}

// ProductPatch describes a partial update. Undefined fields are left untouched.
type ProductPatch struct {
	Name        optional.Field[string]
	Description optional.Field[string]
	Price       optional.Field[float64]
	SalePrice   optional.Field[float64]
}

// IsEmpty reports whether no field was provided.
func (pp ProductPatch) IsEmpty() bool {
	return !pp.Name.Set && !pp.Description.Set && !pp.Price.Set && !pp.SalePrice.Set
}

// ApplyPatch overwrites only the fields present in the patch, then validates the result.
// The receiver is left unchanged when an error is returned.
func (p *Product) ApplyPatch(patch ProductPatch) error {
	next := *p

	if patch.Name.Set {
		if patch.Name.Null {
			return NewError(ErrCodeInvalidName, "name must not be null")
		}
		next.Name = strings.TrimSpace(patch.Name.Value)
	}
	if patch.Description.Set {
		next.Description = patch.Description.Ptr()
	}
	if patch.Price.Set {
		if patch.Price.Null {
			return NewError(ErrCodeInvalidPrice, "price must not be null")
		}
		next.Price = patch.Price.Value
	}
	if patch.SalePrice.Set {
		next.SalePrice = patch.SalePrice.Ptr()
	}

	if err := next.Validate(); err != nil {
		return err
	}
	*p = next
	return nil
}

func isFinite(f float64) bool { return !math.IsNaN(f) && !math.IsInf(f, 0) }

// ProductRepository is the persistence port implemented by the infrastructure layer.
type ProductRepository interface {
	Create(ctx context.Context, p *Product) error
	// GetByIDForUpdate loads a product and locks the row when called inside a transaction.
	GetByIDForUpdate(ctx context.Context, id int64) (*Product, error)
	Update(ctx context.Context, p *Product) error
}

// Transactor runs fn inside a single database transaction.
type Transactor interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
