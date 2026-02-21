package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/bowmanmike/playlistgen/internal/app"
)

// Config drives Store construction.
type Config struct {
	Path string
}

// Store implements app.TrackStore backed by SQLite.
type Store struct {
	db *sql.DB
}

// New creates a Store and ensures schema exists.
func New(cfg Config) (*Store, error) {
	if cfg.Path == "" {
		return nil, errors.New("database path is required")
	}

	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite DB: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	if err := bootstrap(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("bootstrap schema: %w", err)
	}

	return &Store{db: db}, nil
}

func bootstrap(db *sql.DB) error {
	const schema = `CREATE TABLE IF NOT EXISTS tracks (
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
`
	_, err := db.Exec(schema)
	return err
}

// SaveTracks inserts or replaces provided tracks.
func (s *Store) SaveTracks(ctx context.Context, tracks []app.Track) error {
	if len(tracks) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO tracks (navidrome_id, title, artist, artist_id, album, album_id, album_artist, genre, year, track_number, disc_number, duration_seconds, bitrate, file_size, path, content_type, suffix, created_at)
	        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(navidrome_id) DO UPDATE SET
            title=excluded.title,
            artist=excluded.artist,
            artist_id=excluded.artist_id,
            album=excluded.album,
            album_id=excluded.album_id,
            album_artist=excluded.album_artist,
            genre=excluded.genre,
            year=excluded.year,
            track_number=excluded.track_number,
            disc_number=excluded.disc_number,
            duration_seconds=excluded.duration_seconds,
            bitrate=excluded.bitrate,
            file_size=excluded.file_size,
            path=excluded.path,
            content_type=excluded.content_type,
            suffix=excluded.suffix,
            created_at=excluded.created_at`)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("prepare stmt: %w", err)
	}
	defer stmt.Close()

	for _, tr := range tracks {
		if _, err := stmt.ExecContext(ctx,
			tr.ID,
			tr.Title,
			tr.Artist,
			tr.ArtistID,
			tr.Album,
			tr.AlbumID,
			tr.AlbumArtist,
			nullString(tr.Genre),
			nullInt(tr.Year),
			nullInt(tr.TrackNumber),
			nullInt(tr.DiscNumber),
			int(tr.Duration/time.Second),
			nullInt(tr.BitRate),
			nullInt64(tr.FileSize),
			tr.Path,
			nullString(tr.ContentType),
			tr.Suffix,
			tr.CreatedAt.Format(time.RFC3339Nano),
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("exec insert: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

// Close releases database resources.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func nullString(v *string) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func nullInt(v *int) interface{} {
	if v == nil {
		return nil
	}
	return *v
}

func nullInt64(v *int64) interface{} {
	if v == nil {
		return nil
	}
	return *v
}
