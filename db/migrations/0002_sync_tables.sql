-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS navidrome_syncs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    tracks_processed INTEGER NOT NULL DEFAULT 0,
    tracks_updated INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'in_progress'
);

CREATE TABLE IF NOT EXISTS navidrome_track_sync_status (
    track_id INTEGER NOT NULL PRIMARY KEY,
    navidrome_id TEXT NOT NULL UNIQUE,
    last_synced_at TEXT NOT NULL,
    sync_id INTEGER NOT NULL,
    FOREIGN KEY(track_id) REFERENCES tracks(id) ON DELETE CASCADE,
    FOREIGN KEY(sync_id) REFERENCES navidrome_syncs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_track_sync_status_navidrome ON navidrome_track_sync_status(navidrome_id);

-- +goose Down
DROP TABLE IF EXISTS navidrome_track_sync_status;
DROP TABLE IF EXISTS navidrome_syncs;
