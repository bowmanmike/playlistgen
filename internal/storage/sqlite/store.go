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

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := migrations.Run(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &Store{db: db}, nil
}

// SaveTracks inserts or replaces provided tracks and records sync metadata.
func (s *Store) SaveTracks(ctx context.Context, tracks []app.Track) error {
	if len(tracks) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	syncID, err := createSync(ctx, tx)
	if err != nil {
		tx.Rollback()
		return err
	}

	trackStmt, err := tx.PrepareContext(ctx, insertTrackSQL)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("prepare track stmt: %w", err)
	}
	defer trackStmt.Close()

	statusStmt, err := tx.PrepareContext(ctx, upsertTrackSyncStatusSQL)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("prepare status stmt: %w", err)
	}
	defer statusStmt.Close()

	processed, updated := 0, 0

	for _, tr := range tracks {
		processed++
		_, err := trackStmt.ExecContext(ctx,
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
		)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("exec insert track: %w", err)
		}
		updated++

		trackID, err := fetchTrackID(ctx, tx, tr.ID)
		if err != nil {
			tx.Rollback()
			return err
		}

		if _, err := statusStmt.ExecContext(ctx, trackID, tr.ID, time.Now().Format(time.RFC3339Nano), syncID); err != nil {
			tx.Rollback()
			return fmt.Errorf("update track sync status: %w", err)
		}
	}

	if err := completeSync(ctx, tx, syncID, processed, updated, "completed"); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}

func fetchTrackID(ctx context.Context, tx *sql.Tx, navidromeID string) (int64, error) {
	var id int64
	if err := tx.QueryRowContext(ctx, "SELECT id FROM tracks WHERE navidrome_id = ?", navidromeID).Scan(&id); err != nil {
		return 0, fmt.Errorf("fetch track id: %w", err)
	}
	return id, nil
}

func createSync(ctx context.Context, tx *sql.Tx) (int64, error) {
	now := time.Now().Format(time.RFC3339Nano)
	res, err := tx.ExecContext(ctx, createSyncSQL, now)
	if err != nil {
		return 0, fmt.Errorf("create sync: %w", err)
	}
	syncID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("sync last insert id: %w", err)
	}
	return syncID, nil
}

func completeSync(ctx context.Context, tx *sql.Tx, syncID int64, processed, updated int, status string) error {
	now := time.Now().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, completeSyncSQL, now, status, processed, updated, syncID); err != nil {
		return fmt.Errorf("complete sync: %w", err)
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
