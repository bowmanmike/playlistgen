package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/bowmanmike/playlistgen/internal/app"
	"github.com/bowmanmike/playlistgen/internal/migrations"
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

	if err := migrations.Run(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &Store{db: db}, nil
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
