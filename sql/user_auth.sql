-- user_auth.sql
-- Authentication table for Columbia Outdoor users (homeowners and contractors)

CREATE TABLE user_auth (
    id              BIGSERIAL PRIMARY KEY,
    email           TEXT UNIQUE NOT NULL,
    password_hash   TEXT NOT NULL,                    -- Store bcrypt/argon2/scrypt hash
    role            TEXT NOT NULL CHECK (role IN ('homeowner', 'contractor', 'admin')),
    
    -- Optional profile fields (can be moved to a separate users table later)
    first_name      TEXT,
    last_name       TEXT,
    phone           TEXT,
    
    -- Account status
    is_active       BOOLEAN DEFAULT TRUE,
    email_verified  BOOLEAN DEFAULT FALSE,
    
    -- Timestamps
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    last_login_at   TIMESTAMPTZ
);

-- Indexes for performance
CREATE INDEX idx_user_auth_email ON user_auth(email);
CREATE INDEX idx_user_auth_role   ON user_auth(role);

-- Auto-update updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER trigger_update_user_auth_timestamp
    BEFORE UPDATE ON user_auth
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();