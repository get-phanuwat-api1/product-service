-- Append-only audit trail of business changes. No FK on entity_id on purpose:
-- audit rows must survive even if the audited entity is deleted.
CREATE TABLE IF NOT EXISTS audit_logs (
    id          BIGSERIAL PRIMARY KEY,
    action      VARCHAR(50)  NOT NULL,
    entity_type VARCHAR(50)  NOT NULL,
    entity_id   BIGINT       NOT NULL,
    actor       VARCHAR(128) NOT NULL,
    request_id  VARCHAR(128) NOT NULL DEFAULT '',
    before_data JSONB        NULL,
    after_data  JSONB        NULL,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_entity ON audit_logs (entity_type, entity_id, created_at);
