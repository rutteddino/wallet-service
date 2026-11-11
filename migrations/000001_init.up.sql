CREATE TABLE IF NOT EXISTS wallets (
    id         UUID PRIMARY KEY,
    balance    BIGINT NOT NULL DEFAULT 0 CHECK (balance >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS operations (
    id              UUID PRIMARY KEY,
    wallet_id       UUID NOT NULL REFERENCES wallets (id),
    operation_type  TEXT NOT NULL CHECK (operation_type IN ('DEPOSIT', 'WITHDRAW')),
    amount          BIGINT NOT NULL CHECK (amount > 0),
    idempotency_key TEXT NOT NULL UNIQUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_operations_wallet_id ON operations (wallet_id);
