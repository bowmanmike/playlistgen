-- +goose Up
ALTER TABLE navidrome_syncs ADD COLUMN tracks_deleted INTEGER NOT NULL DEFAULT 0;

-- +goose Down
-- +goose StatementBegin
PRAGMA foreign_keys = OFF;

CREATE TABLE navidrome_syncs_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    tracks_processed INTEGER NOT NULL DEFAULT 0,
    tracks_updated INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'in_progress'
);

INSERT INTO navidrome_syncs_new (id, started_at, completed_at, tracks_processed, tracks_updated, status)
SELECT id, started_at, completed_at, tracks_processed, tracks_updated, status
FROM navidrome_syncs;

DROP TABLE navidrome_syncs;

ALTER TABLE navidrome_syncs_new RENAME TO navidrome_syncs;

PRAGMA foreign_keys = ON;
-- +goose StatementEnd
