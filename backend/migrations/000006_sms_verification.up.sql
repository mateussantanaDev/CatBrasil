-- Códigos antigos em texto puro são invalidados antes de passar ao armazenamento HMAC.
DELETE FROM sms_verifications;
ALTER TABLE sms_verifications ALTER COLUMN code TYPE TEXT;
ALTER TABLE sms_verifications ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_sms_verifications_recent ON sms_verifications (phone, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_verified_phone_unique ON users (phone) WHERE phone LIKE '+55%';
