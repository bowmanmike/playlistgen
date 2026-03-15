-- +goose Up
-- +goose StatementBegin
ALTER TABLE track_audio_analysis ADD COLUMN claimed_at TEXT;
ALTER TABLE track_audio_analysis ADD COLUMN claimed_by TEXT;

ALTER TABLE track_embedding_jobs ADD COLUMN claimed_at TEXT;
ALTER TABLE track_embedding_jobs ADD COLUMN claimed_by TEXT;

CREATE UNIQUE INDEX idx_audio_one_active_job_per_track
ON track_audio_analysis(track_id)
WHERE status IN ('pending', 'processing');

CREATE UNIQUE INDEX idx_embedding_one_active_job_per_track
ON track_embedding_jobs(track_id)
WHERE status IN ('pending', 'processing');

CREATE INDEX idx_audio_claim_lookup
ON track_audio_analysis(status, claimed_at, created_at, id);

CREATE INDEX idx_embedding_claim_lookup
ON track_embedding_jobs(status, claimed_at, created_at, id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_embedding_claim_lookup;
DROP INDEX IF EXISTS idx_audio_claim_lookup;
DROP INDEX IF EXISTS idx_embedding_one_active_job_per_track;
DROP INDEX IF EXISTS idx_audio_one_active_job_per_track;

PRAGMA foreign_keys = OFF;

CREATE TABLE track_audio_analysis_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    track_id INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    processed_at TEXT,
    error TEXT,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_attempt_at TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY(track_id) REFERENCES tracks(id) ON DELETE CASCADE
);

INSERT INTO track_audio_analysis_old (
    id,
    track_id,
    status,
    processed_at,
    error,
    attempts,
    last_attempt_at,
    created_at
) SELECT
    id,
    track_id,
    status,
    processed_at,
    error,
    attempts,
    last_attempt_at,
    created_at
FROM track_audio_analysis;

DROP TABLE track_audio_analysis;
ALTER TABLE track_audio_analysis_old RENAME TO track_audio_analysis;

CREATE TABLE track_embedding_jobs_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    track_id INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    processed_at TEXT,
    error TEXT,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_attempt_at TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY(track_id) REFERENCES tracks(id) ON DELETE CASCADE
);

INSERT INTO track_embedding_jobs_old (
    id,
    track_id,
    status,
    processed_at,
    error,
    attempts,
    last_attempt_at,
    created_at
) SELECT
    id,
    track_id,
    status,
    processed_at,
    error,
    attempts,
    last_attempt_at,
    created_at
FROM track_embedding_jobs;

DROP TABLE track_embedding_jobs;
ALTER TABLE track_embedding_jobs_old RENAME TO track_embedding_jobs;

CREATE INDEX IF NOT EXISTS idx_audio_analysis_status ON track_audio_analysis(status);
CREATE INDEX IF NOT EXISTS idx_audio_analysis_track ON track_audio_analysis(track_id);
CREATE INDEX IF NOT EXISTS idx_embedding_jobs_status ON track_embedding_jobs(status);
CREATE INDEX IF NOT EXISTS idx_embedding_jobs_track ON track_embedding_jobs(track_id);

PRAGMA foreign_keys = ON;
-- +goose StatementEnd
