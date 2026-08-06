-- Add notes column to estimate_custom_items for the longer description field
ALTER TABLE estimate_custom_items ADD COLUMN IF NOT EXISTS notes TEXT NOT NULL DEFAULT '';
