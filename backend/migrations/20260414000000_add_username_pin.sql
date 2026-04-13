-- Add username and pin_hash columns for OS-style login
-- This replaces email/password with username/PIN for authentication

BEGIN;

-- Add username column (unique, used for login display)
ALTER TABLE users ADD COLUMN username TEXT UNIQUE;

-- Add pin_hash column (replaces password_hash for PIN-based auth)
ALTER TABLE users ADD COLUMN pin_hash TEXT;

-- Create index on username for fast lookups
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

-- Migrate existing email to username where username is null
-- This assumes email prefix can be used as username
UPDATE users SET username = LOWER(SUBSTR(email, 1, INSTR(email, '@') - 1)) WHERE username IS NULL;

COMMIT;
