-- +goose Up
-- +goose StatementBegin
PRAGMA foreign_keys = OFF;

CREATE TABLE navidrome_syncs_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    tracks_processed INTEGER NOT NULL DEFAULT 0,
    tracks_updated INTEGER NOT NULL DEFAULT 0,
    tracks_deleted INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'in_progress',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO navidrome_syncs_new (
    id,
    started_at,
    completed_at,
    tracks_processed,
    tracks_updated,
    tracks_deleted,
    status,
    created_at
) SELECT
    id,
    started_at,
    completed_at,
    tracks_processed,
    tracks_updated,
    tracks_deleted,
    status,
    COALESCE(started_at, datetime('now'))
FROM navidrome_syncs;

DROP TABLE navidrome_syncs;
ALTER TABLE navidrome_syncs_new RENAME TO navidrome_syncs;

CREATE TABLE navidrome_track_sync_status_new (
    track_id INTEGER NOT NULL PRIMARY KEY,
    navidrome_id TEXT NOT NULL UNIQUE,
    last_synced_at TEXT NOT NULL,
    sync_id INTEGER NOT NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY(track_id) REFERENCES tracks(id) ON DELETE CASCADE,
    FOREIGN KEY(sync_id) REFERENCES navidrome_syncs(id) ON DELETE CASCADE
);

INSERT INTO navidrome_track_sync_status_new (track_id, navidrome_id, last_synced_at, sync_id, created_at)
SELECT track_id, navidrome_id, last_synced_at, sync_id, last_synced_at
FROM navidrome_track_sync_status;

DROP TABLE navidrome_track_sync_status;
ALTER TABLE navidrome_track_sync_status_new RENAME TO navidrome_track_sync_status;

PRAGMA foreign_keys = ON;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
PRAGMA foreign_keys = OFF;

CREATE TABLE navidrome_syncs_old (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    tracks_processed INTEGER NOT NULL DEFAULT 0,
    tracks_updated INTEGER NOT NULL DEFAULT 0,
    tracks_deleted INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'in_progress'
);

INSERT INTO navidrome_syncs_old (
    id,
    started_at,
    completed_at,
    tracks_processed,
    tracks_updated,
    tracks_deleted,
    status
) SELECT
    id,
    started_at,
    completed_at,
    tracks_processed,
    tracks_updated,
    tracks_deleted,
    status
FROM navidrome_syncs;

DROP TABLE navidrome_syncs;
ALTER TABLE navidrome_syncs_old RENAME TO navidrome_syncs;

CREATE TABLE navidrome_track_sync_status_old (
    track_id INTEGER NOT NULL PRIMARY KEY,
    navidrome_id TEXT NOT NULL UNIQUE,
    last_synced_at TEXT NOT NULL,
    sync_id INTEGER NOT NULL,
    FOREIGN KEY(track_id) REFERENCES tracks(id) ON DELETE CASCADE,
    FOREIGN KEY(sync_id) REFERENCES navidrome_syncs(id) ON DELETE CASCADE
);

INSERT INTO navidrome_track_sync_status_old (track_id, navidrome_id, last_synced_at, sync_id)
SELECT track_id, navidrome_id, last_synced_at, sync_id
FROM navidrome_track_sync_status;

DROP TABLE navidrome_track_sync_status;
ALTER TABLE navidrome_track_sync_status_old RENAME TO navidrome_track_sync_status;

PRAGMA foreign_keys = ON;
-- +goose StatementEnd
