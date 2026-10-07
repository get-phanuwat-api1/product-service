package model

import (
	"encoding/json"
	"fmt"
	"time"
)

// AuditAction describes what happened to an entity.
type AuditAction string

const (
	AuditActionProductCreated AuditAction = "PRODUCT_CREATED"
	AuditActionProductUpdated AuditAction = "PRODUCT_UPDATED"
)

// AuditEntityProduct is the entity_type value used for products.
const AuditEntityProduct = "product"

// AuditLog is one immutable record of a business change.
type AuditLog struct {
	ID         int64
	Action     AuditAction
	EntityType string
	EntityID   int64
	Actor      string
	RequestID  string
	BeforeData json.RawMessage // nil when the entity did not exist before (create)
	AfterData  json.RawMessage
	CreatedAt  time.Time
}

// ProductSnapshot is the audited representation of a product.
type ProductSnapshot struct {
	ID          int64    `json:"id"`
	Name        string   `json:"name"`
	Description *string  `json:"description"`
	Price       float64  `json:"price"`
	SalePrice   *float64 `json:"sale_price"`
}

// Snapshot captures the audited fields of the product.
func (p *Product) Snapshot() ProductSnapshot {
	return ProductSnapshot{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
		SalePrice:   p.SalePrice,
	}
}

// Equal reports whether two snapshots hold the same values.
func (s ProductSnapshot) Equal(o ProductSnapshot) bool {
	return s.ID == o.ID && s.Name == o.Name && s.Price == o.Price &&
		equalPtr(s.Description, o.Description) && equalPtr(s.SalePrice, o.SalePrice)
}

func equalPtr[T comparable](a, b *T) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// NewProductAuditLog builds an audit record for a product change.
// before is nil for a create.
func NewProductAuditLog(action AuditAction, before *ProductSnapshot, after ProductSnapshot, actor, requestID string) (*AuditLog, error) {
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return nil, fmt.Errorf("marshal after snapshot: %w", err)
	}
	var beforeJSON json.RawMessage
	if before != nil {
		if beforeJSON, err = json.Marshal(before); err != nil {
			return nil, fmt.Errorf("marshal before snapshot: %w", err)
		}
	}
	return &AuditLog{
		Action:     action,
		EntityType: AuditEntityProduct,
		EntityID:   after.ID,
		Actor:      actor,
		RequestID:  requestID,
		BeforeData: beforeJSON,
		AfterData:  afterJSON,
	}, nil
}
