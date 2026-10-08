package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/get-phanuwat-api1/product-service/internal/model"
	"github.com/get-phanuwat-api1/product-service/internal/repository"
	"github.com/get-phanuwat-api1/product-service/test/testutil"
)

func TestAuditLogRepository_CreateAndList(t *testing.T) {
	testutil.TruncateTables(t, pool)
	repo := repository.NewAuditLogRepository(pool)
	ctx := context.Background()

	created := &model.AuditLog{
		Action: model.AuditActionProductCreated, EntityType: "product", EntityID: 7,
		Actor: "user-1", RequestID: "req-1",
		AfterData: []byte(`{"id":7,"name":"Apple"}`),
	}
	require.NoError(t, repo.CreateAuditLog(ctx, created))
	assert.NotZero(t, created.ID)
	assert.False(t, created.CreatedAt.IsZero())

	updated := &model.AuditLog{
		Action: model.AuditActionProductUpdated, EntityType: "product", EntityID: 7,
		Actor: "user-2", RequestID: "req-2",
		BeforeData: []byte(`{"id":7,"name":"Apple"}`),
		AfterData:  []byte(`{"id":7,"name":"Banana"}`),
	}
	require.NoError(t, repo.CreateAuditLog(ctx, updated))

	// a different entity must not show up
	require.NoError(t, repo.CreateAuditLog(ctx, &model.AuditLog{
		Action: model.AuditActionProductCreated, EntityType: "product", EntityID: 8, Actor: "x",
		AfterData: []byte(`{}`),
	}))

	logs, err := repo.ListAuditLogsByEntity(ctx, "product", 7)
	require.NoError(t, err)
	require.Len(t, logs, 2)

	assert.Equal(t, model.AuditActionProductCreated, logs[0].Action)
	assert.Equal(t, "user-1", logs[0].Actor)
	assert.Equal(t, "req-1", logs[0].RequestID)
	assert.Nil(t, logs[0].BeforeData)
	assert.JSONEq(t, `{"id":7,"name":"Apple"}`, string(logs[0].AfterData))

	assert.Equal(t, model.AuditActionProductUpdated, logs[1].Action)
	assert.JSONEq(t, `{"id":7,"name":"Apple"}`, string(logs[1].BeforeData))
	assert.JSONEq(t, `{"id":7,"name":"Banana"}`, string(logs[1].AfterData))
}

func TestAuditLogRepository_RollsBackWithTransaction(t *testing.T) {
	testutil.TruncateTables(t, pool)
	repo := repository.NewAuditLogRepository(pool)
	tx := repository.NewTransactor(pool)
	ctx := context.Background()

	boom := errors.New("boom")
	err := tx.WithinTransaction(ctx, func(ctx context.Context) error {
		require.NoError(t, repo.CreateAuditLog(ctx, &model.AuditLog{
			Action: model.AuditActionProductCreated, EntityType: "product", EntityID: 1, Actor: "x",
			AfterData: []byte(`{}`),
		}))
		return boom
	})
	assert.ErrorIs(t, err, boom)

	logs, err := repo.ListAuditLogsByEntity(ctx, "product", 1)
	require.NoError(t, err)
	assert.Empty(t, logs)
}
