-- Discard invalid legacy values. Earlier migration converted valid six-digit PINs
-- to bcrypt; any other non-hash value cannot be used safely for withdrawals.
UPDATE users
SET security_password = ''
WHERE security_password IS NULL
   OR (security_password <> '' AND security_password !~ '^\$2[aby]\$[0-9]{2}\$[./A-Za-z0-9]{53}$');

ALTER TABLE users
    ALTER COLUMN security_password SET DEFAULT '',
    ALTER COLUMN security_password SET NOT NULL;

ALTER TABLE users ADD CONSTRAINT users_security_password_bcrypt_only
    CHECK (security_password = '' OR security_password ~ '^\$2[aby]\$[0-9]{2}\$[./A-Za-z0-9]{53}$');
