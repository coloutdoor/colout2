-- photos table: replaces static/photos.yaml as the source of truth for
-- project photos (admin review queue, public /projects gallery, and
-- sitemap.xml image entries).
--
-- No migration framework — apply manually to both DATABASE_URL (dev) and
-- DATABASE_URL_PROD, per CLAUDE.md.

CREATE TABLE photos (
    id           SERIAL PRIMARY KEY,
    uri          TEXT NOT NULL UNIQUE,
    md5          TEXT,
    source_path  TEXT,
    city         TEXT NOT NULL DEFAULT '',
    category     TEXT NOT NULL DEFAULT '',
    description  TEXT NOT NULL DEFAULT '',
    tags         TEXT[] NOT NULL DEFAULT '{}',
    featured     BOOLEAN NOT NULL DEFAULT false,
    reviewed     BOOLEAN NOT NULL DEFAULT false,
    project      TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_photos_project  ON photos (project);
CREATE INDEX idx_photos_md5      ON photos (md5);
CREATE INDEX idx_photos_reviewed ON photos (reviewed);
