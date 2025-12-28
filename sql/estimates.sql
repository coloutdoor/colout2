-- estimates.sql
-- Create the estimates table for Columbia Outdoor deck/patio calculator submissions

CREATE TABLE IF NOT EXISTS estimates (
    estimate_id    BIGSERIAL PRIMARY KEY,           -- Auto-incrementing ID (better than INTEGER for growth)
    description    TEXT,                            -- Your 'desc' field (renamed for clarity)
    length         DOUBLE PRECISION,                -- REAL → DOUBLE PRECISION (more precision)
    width          DOUBLE PRECISION,
    height         DOUBLE PRECISION,
    material       TEXT,
    rail_material  TEXT,
    rail_infill    TEXT,
    stair_width    DOUBLE PRECISION,
    stair_rail_count DOUBLE PRECISION,
    has_demo       BOOLEAN     DEFAULT FALSE,
    has_fascia     BOOLEAN     DEFAULT FALSE,
    total_cost     DOUBLE PRECISION,
    
    -- Homeowner contact info
    first_name     TEXT,
    last_name      TEXT,
    address        TEXT,
    city           TEXT,
    state          TEXT,                            -- e.g., 'WA', 'OR', 'ID'
    zip            TEXT,
    phone_number   TEXT,
    email          TEXT,
    
    -- Dates (stored as text in original — better as proper types when possible)
    save_date      TIMESTAMPTZ,                     -- When estimate was saved/created
    accept_date    TIMESTAMPTZ,                     -- When contractor/homeowner accepted
    expiration_date TIMESTAMPTZ,                    -- Estimate expiration
    
    -- Timestamps for record tracking
    created_at     TIMESTAMPTZ DEFAULT NOW(),
    updated_at     TIMESTAMPTZ DEFAULT NOW()
);

-- Useful indexes for common queries
CREATE INDEX IF NOT EXISTS idx_estimates_email       ON estimates(email);
CREATE INDEX IF NOT EXISTS idx_estimates_created_at  ON estimates(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_estimates_total_cost  ON estimates(total_cost);
CREATE INDEX IF NOT EXISTS idx_estimates_state       ON estimates(state);

-- This will reset back to 1000
ALTER SEQUENCE estimates_estimate_id_seq RESTART WITH 1000;

-- Optional: Add a simple trigger to auto-update updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER trigger_update_estimates_timestamp
    BEFORE UPDATE ON estimates
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();