CREATE TABLE IF NOT EXISTS products (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(255)     NOT NULL,
    description TEXT             NULL,
    price       DOUBLE PRECISION NOT NULL CHECK (price >= 0),
    sale_price  DOUBLE PRECISION NULL CHECK (sale_price >= 0),
    created_at  TIMESTAMPTZ      NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);
