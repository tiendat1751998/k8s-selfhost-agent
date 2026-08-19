DROP INDEX IF EXISTS idx_refresh_tokens_hash;
DROP INDEX IF EXISTS idx_refresh_tokens_user;
DROP TABLE IF EXISTS refresh_tokens;

ALTER TABLE users DROP COLUMN IF EXISTS mfa_verified_at;
ALTER TABLE users DROP COLUMN IF EXISTS mfa_recovery_codes;
ALTER TABLE users DROP COLUMN IF EXISTS mfa_enabled;
ALTER TABLE users DROP COLUMN IF EXISTS mfa_secret;
