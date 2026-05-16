-- contractor_profile.sql
-- Contractor branding, credentials, and approval status for Columbia Outdoor platform

-- Add project_manager role to user_auth
ALTER TABLE user_auth
DROP CONSTRAINT IF EXISTS user_auth_role_check;

ALTER TABLE user_auth
ADD CONSTRAINT user_auth_role_check
    CHECK (role IN ('homeowner', 'contractor', 'admin', 'project_manager'));

-- Contractor profile table
CREATE TABLE IF NOT EXISTS contractor_profile (
    id                  BIGSERIAL PRIMARY KEY,
    user_id             BIGINT NOT NULL UNIQUE REFERENCES user_auth(id),

    -- Company branding
    company_name        TEXT NOT NULL,
    logo_url            TEXT,                       -- URL to logo image (GCS or external)
    phone               TEXT,
    website             TEXT,
    terms_text          TEXT,                       -- Custom T&C; falls back to Columbia Outdoor default if NULL

    -- Specialty (expandable list: decks, patio_covers, pergolas, landscapes, ...)
    specialties         TEXT[] DEFAULT ARRAY['decks'],

    -- Service area
    service_states      TEXT[],                     -- e.g. ARRAY['WA', 'OR']
    service_city        TEXT,                       -- Primary city e.g. 'Woodland'
    service_radius_miles INT DEFAULT 50,            -- Coverage radius in miles

    -- Licensing & credentials (verified manually by admin before approval)
    license_number      TEXT,
    license_state       TEXT,
    bond_number         TEXT,
    insurance_carrier   TEXT,
    insurance_policy    TEXT,

    -- Approval workflow
    approval_status     TEXT NOT NULL DEFAULT 'pending'
                            CHECK (approval_status IN ('pending', 'approved', 'rejected')),
    approval_notes      TEXT,                       -- Admin notes on approval or rejection
    approved_at         TIMESTAMPTZ,
    approved_by         BIGINT REFERENCES user_auth(id),

    -- Timestamps
    created_at          TIMESTAMPTZ DEFAULT NOW(),
    updated_at          TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_contractor_profile_user_id        ON contractor_profile(user_id);
CREATE INDEX IF NOT EXISTS idx_contractor_profile_approval_status ON contractor_profile(approval_status);
CREATE INDEX IF NOT EXISTS idx_contractor_profile_specialties     ON contractor_profile USING GIN(specialties);
CREATE INDEX IF NOT EXISTS idx_contractor_profile_service_states  ON contractor_profile USING GIN(service_states);
CREATE INDEX IF NOT EXISTS idx_contractor_profile_service_city    ON contractor_profile(service_city);

-- Auto-update updated_at
CREATE TRIGGER trigger_update_contractor_profile_timestamp
    BEFORE UPDATE ON contractor_profile
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
