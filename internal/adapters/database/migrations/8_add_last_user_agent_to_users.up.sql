ALTER TABLE users ADD COLUMN last_user_agent TEXT;
UPDATE users SET last_user_agent = '';
ALTER TABLE users ALTER COLUMN last_user_agent SET NOT NULL;
