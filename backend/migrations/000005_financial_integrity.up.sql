ALTER TABLE transactions
    ADD COLUMN idempotency_key VARCHAR(128),
    ADD COLUMN reference_id VARCHAR(128),
    ADD COLUMN payout_attempted_at TIMESTAMPTZ,
    ADD COLUMN reconciled_at TIMESTAMPTZ;

ALTER TABLE users ADD COLUMN version BIGINT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN withdraw_pin_failures INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN withdraw_pin_locked_until TIMESTAMPTZ;
ALTER TABLE users ALTER COLUMN security_password TYPE TEXT;
UPDATE users SET security_password=crypt(security_password,gen_salt('bf',12))
WHERE security_password ~ '^[0-9]{6}$';

CREATE FUNCTION bump_user_version() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.version := OLD.version + 1;
    RETURN NEW;
END;
$$;

CREATE TRIGGER users_version_on_update BEFORE UPDATE ON users
FOR EACH ROW EXECUTE FUNCTION bump_user_version();

CREATE UNIQUE INDEX transactions_user_idempotency
    ON transactions (user_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE UNIQUE INDEX transactions_gateway_external_id
    ON transactions (gateway_provider, external_id)
    WHERE external_id IS NOT NULL AND gateway_provider = 'suitpay';

CREATE TABLE financial_events (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    transaction_id VARCHAR(64) NOT NULL REFERENCES transactions(id),
    user_id VARCHAR(64) NOT NULL REFERENCES users(id),
    kind VARCHAR(32) NOT NULL,
    amount NUMERIC(15,2) NOT NULL,
    balance_before NUMERIC(15,2) NOT NULL,
    balance_after NUMERIC(15,2) NOT NULL,
    actor_id VARCHAR(64),
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (transaction_id, user_id, kind),
    CHECK (balance_after = balance_before + amount)
);

CREATE INDEX financial_events_user_id_id ON financial_events (user_id, id DESC);

CREATE TABLE payout_gateway_events (
    external_id VARCHAR(255) NOT NULL,
    status VARCHAR(16) NOT NULL CHECK (status IN ('PAID_OUT', 'CANCELED')),
    amount NUMERIC(15,2) NOT NULL CHECK (amount > 0),
    received_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (external_id, status)
);
