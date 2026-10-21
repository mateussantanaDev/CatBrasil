CREATE TABLE IF NOT EXISTS user_identities (
    user_id TEXT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    payload_ciphertext BYTEA NOT NULL,
    cpf_digest CHAR(64) NOT NULL UNIQUE,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'verified', 'rejected')),
    review_ref TEXT,
    reviewed_by TEXT,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT reviewed_identity_has_evidence CHECK (
        status = 'pending' OR (review_ref IS NOT NULL AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)
    )
);

CREATE TABLE IF NOT EXISTS identity_events (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('submitted', 'verified', 'rejected', 'resubmitted')),
    cpf_digest CHAR(64) NOT NULL,
    review_ref TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS identity_events_user_created_idx ON identity_events(user_id, created_at DESC);

CREATE OR REPLACE FUNCTION prevent_identity_event_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'identity_events is append-only';
END;
$$ LANGUAGE plpgsql;
DROP TRIGGER IF EXISTS identity_events_append_only ON identity_events;
CREATE TRIGGER identity_events_append_only BEFORE UPDATE OR DELETE ON identity_events
FOR EACH ROW EXECUTE FUNCTION prevent_identity_event_mutation();
