-- Custom line items for an estimate (contractor-added, can be positive or negative)
CREATE TABLE IF NOT EXISTS estimate_custom_items (
    id           SERIAL PRIMARY KEY,
    estimate_id  INTEGER NOT NULL REFERENCES estimates(estimate_id) ON DELETE CASCADE,
    description  TEXT    NOT NULL DEFAULT '',
    cost         NUMERIC(10,2) NOT NULL DEFAULT 0,
    sort_order   INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_estimate_custom_items_estimate_id
    ON estimate_custom_items (estimate_id);
