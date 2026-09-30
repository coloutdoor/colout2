-- projects table: replaces static/project_descriptions.yaml as the source of
-- truth for project names + blurbs shown on /projects, and is the target of
-- photos.project_id (see below).
--
-- No migration framework — apply manually to both DATABASE_URL (dev) and
-- DATABASE_URL_PROD, per CLAUDE.md.

CREATE TABLE projects (
    id           SERIAL PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    description  TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Replaces the free-text photos.project column with a real foreign key, so a
-- typo in the admin UI can no longer silently create an orphaned "project."
-- The old photos.project text column is left in place (unused by the app
-- going forward) as a safety net until both DBs are confirmed migrated.
ALTER TABLE photos ADD COLUMN project_id INTEGER REFERENCES projects(id);
CREATE INDEX idx_photos_project_id ON photos (project_id);
