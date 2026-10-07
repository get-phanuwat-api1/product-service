package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/phanuwat/product-service/internal/model"
)

func TestProductSnapshot_Equal(t *testing.T) {
	t.Parallel()
	a := baseProduct().Snapshot()

	assert.True(t, a.Equal(baseProduct().Snapshot()), "same values behind different pointers")

	b := baseProduct()
	b.Description = nil
	assert.False(t, a.Equal(b.Snapshot()))

	c := baseProduct()
	c.SalePrice = ptr(1.0)
	assert.False(t, a.Equal(c.Snapshot()))

	d := baseProduct()
	d.Name = "Other"
	assert.False(t, a.Equal(d.Snapshot()))
}

func TestNewProductAuditLog_Create(t *testing.T) {
	t.Parallel()
	after := baseProduct().Snapshot()

	log, err := model.NewProductAuditLog(model.AuditActionProductCreated, nil, after, "user-1", "req-1")

	require.NoError(t, err)
	assert.Equal(t, model.AuditActionProductCreated, log.Action)
	assert.Equal(t, "product", log.EntityType)
	assert.Equal(t, int64(1), log.EntityID)
	assert.Equal(t, "user-1", log.Actor)
	assert.Equal(t, "req-1", log.RequestID)
	assert.Nil(t, log.BeforeData)
	assert.JSONEq(t, `{"id":1,"name":"Apple","description":"fresh","price":100,"sale_price":90}`, string(log.AfterData))
}

func TestNewProductAuditLog_Update(t *testing.T) {
	t.Parallel()
	before := baseProduct().Snapshot()
	p := baseProduct()
	p.Description = nil

	log, err := model.NewProductAuditLog(model.AuditActionProductUpdated, &before, p.Snapshot(), "user-1", "req-1")

	require.NoError(t, err)
	assert.JSONEq(t, `{"id":1,"name":"Apple","description":"fresh","price":100,"sale_price":90}`, string(log.BeforeData))
	assert.JSONEq(t, `{"id":1,"name":"Apple","description":null,"price":100,"sale_price":90}`, string(log.AfterData))
}
