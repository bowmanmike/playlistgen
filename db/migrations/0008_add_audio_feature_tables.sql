-- +goose Up
CREATE TABLE IF NOT EXISTS track_audio_features (
    track_id INTEGER NOT NULL PRIMARY KEY,
    analyzed_at TEXT NOT NULL,
    file_duration_seconds REAL NOT NULL,
    measured_integrated_lufs REAL,
    measured_true_peak REAL,
    replaygain_track_gain_db REAL,
    replaygain_track_peak REAL,
    replaygain_album_gain_db REAL,
    replaygain_album_peak REAL,
    effective_gain_db REAL,
    effective_peak REAL,
    effective_gain_source TEXT NOT NULL,
    effective_peak_source TEXT NOT NULL,
    FOREIGN KEY(track_id) REFERENCES tracks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS audio_processing_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    status TEXT NOT NULL DEFAULT 'in_progress',
    jobs_claimed INTEGER NOT NULL DEFAULT 0,
    jobs_completed INTEGER NOT NULL DEFAULT 0,
    jobs_failed INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_audio_processing_runs_started_at
ON audio_processing_runs(started_at);

-- +goose Down
DROP INDEX IF EXISTS idx_audio_processing_runs_started_at;
DROP TABLE IF EXISTS audio_processing_runs;
DROP TABLE IF EXISTS track_audio_features;
