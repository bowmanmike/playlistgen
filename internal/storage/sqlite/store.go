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
        id TEXT PRIMARY KEY,
        title TEXT,
        artist TEXT,
        album TEXT,
        duration_seconds INTEGER,
        path TEXT
    );`
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

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO tracks (id, title, artist, album, duration_seconds, path)
        VALUES (?, ?, ?, ?, ?, ?)
        ON CONFLICT(id) DO UPDATE SET
            title=excluded.title,
            artist=excluded.artist,
            album=excluded.album,
            duration_seconds=excluded.duration_seconds,
            path=excluded.path`)
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
			tr.Album,
			int(tr.Duration/time.Second),
			tr.Path,
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
