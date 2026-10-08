package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/get-phanuwat-api1/product-service/internal/model"
)

// AuditLogRepository persists audit records. Calls made with a transactional ctx
// join that transaction, so the audit row commits or rolls back with the change.
type AuditLogRepository interface {
	// CreateAuditLog inserts the record and fills its ID and CreatedAt.
	CreateAuditLog(ctx context.Context, log *model.AuditLog) error
	// ListAuditLogsByEntity returns the history of one entity, oldest first (id is insertion order).
	ListAuditLogsByEntity(ctx context.Context, entityType string, entityID int64) ([]model.AuditLog, error)
}

type auditLogRepository struct {
	pool *pgxpool.Pool
}

func NewAuditLogRepository(pool *pgxpool.Pool) AuditLogRepository {
	return &auditLogRepository{pool: pool}
}

func (r *auditLogRepository) CreateAuditLog(ctx context.Context, log *model.AuditLog) error {
	const q = `
		INSERT INTO audit_logs (action, entity_type, entity_id, actor, request_id, before_data, after_data)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb)
		RETURNING id, created_at`
	err := conn(ctx, r.pool).
		QueryRow(ctx, q, string(log.Action), log.EntityType, log.EntityID, log.Actor, log.RequestID,
			nullableJSON(log.BeforeData), nullableJSON(log.AfterData)).
		Scan(&log.ID, &log.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

func (r *auditLogRepository) ListAuditLogsByEntity(ctx context.Context, entityType string, entityID int64) ([]model.AuditLog, error) {
	const q = `
		SELECT id, action, entity_type, entity_id, actor, request_id, before_data, after_data, created_at
		FROM audit_logs
		WHERE entity_type = $1 AND entity_id = $2
		ORDER BY id`
	rows, err := r.pool.Query(ctx, q, entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	var logs []model.AuditLog
	for rows.Next() {
		var (
			l             model.AuditLog
			action        string
			before, after []byte
		)
		if err := rows.Scan(&l.ID, &action, &l.EntityType, &l.EntityID, &l.Actor, &l.RequestID,
			&before, &after, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		l.Action = model.AuditAction(action)
		l.BeforeData, l.AfterData = before, after
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate audit logs: %w", err)
	}
	return logs, nil
}

// nullableJSON maps empty JSON to SQL NULL and passes raw JSON as text for ::jsonb.
func nullableJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}
