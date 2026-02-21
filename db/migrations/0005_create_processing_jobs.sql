-- +goose Up
CREATE TABLE IF NOT EXISTS track_audio_analysis (
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

CREATE TABLE IF NOT EXISTS track_embedding_jobs (
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

CREATE INDEX IF NOT EXISTS idx_audio_analysis_status ON track_audio_analysis(status);
CREATE INDEX IF NOT EXISTS idx_audio_analysis_track ON track_audio_analysis(track_id);
CREATE INDEX IF NOT EXISTS idx_embedding_jobs_status ON track_embedding_jobs(status);
CREATE INDEX IF NOT EXISTS idx_embedding_jobs_track ON track_embedding_jobs(track_id);

-- +goose Down
DROP TABLE IF EXISTS track_embedding_jobs;
DROP TABLE IF EXISTS track_audio_analysis;
