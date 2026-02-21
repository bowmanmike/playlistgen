-- +goose Up
CREATE TABLE IF NOT EXISTS tracks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    navidrome_id TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    artist TEXT NOT NULL,
    artist_id TEXT,
    album TEXT NOT NULL,
    album_id TEXT,
    album_artist TEXT,
    genre TEXT,
    year INTEGER,
    track_number INTEGER,
    disc_number INTEGER,
    duration_seconds INTEGER NOT NULL DEFAULT 0,
    bitrate INTEGER,
    file_size INTEGER,
    path TEXT NOT NULL,
    content_type TEXT,
    suffix TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tracks_artist ON tracks(artist);
CREATE INDEX IF NOT EXISTS idx_tracks_album ON tracks(album);
CREATE INDEX IF NOT EXISTS idx_tracks_title ON tracks(title);
CREATE INDEX IF NOT EXISTS idx_tracks_navidrome_id ON tracks(navidrome_id);
CREATE INDEX IF NOT EXISTS idx_tracks_genre ON tracks(genre);
CREATE INDEX IF NOT EXISTS idx_tracks_year ON tracks(year);

-- +goose Down
DROP TABLE IF EXISTS tracks;
