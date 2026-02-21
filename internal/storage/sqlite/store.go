package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/bowmanmike/playlistgen/internal/app"
	"github.com/bowmanmike/playlistgen/internal/db"
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

	queries := db.New(tx)
	startedAt := nowUTC()
	syncID, err := queries.CreateSync(ctx, startedAt)
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("create sync: %w", err)
	}

	processed, updated := 0, 0
	syncedAt := nowUTC()

	for _, tr := range tracks {
		processed++
		if err := queries.UpsertTrack(ctx, convertTrack(tr)); err != nil {
			tx.Rollback()
			return fmt.Errorf("upsert track: %w", err)
		}
		updated++

		trackID, err := queries.SelectTrackID(ctx, tr.ID)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("select track id: %w", err)
		}

		if err := queries.UpsertTrackSyncStatus(ctx, db.UpsertTrackSyncStatusParams{
			TrackID:      trackID,
			NavidromeID:  tr.ID,
			LastSyncedAt: syncedAt,
			SyncID:       syncID,
		}); err != nil {
			tx.Rollback()
			return fmt.Errorf("update track sync status: %w", err)
		}
	}

	completedAt := sql.NullString{String: nowUTC(), Valid: true}
	if err := queries.CompleteSync(ctx, db.CompleteSyncParams{
		CompletedAt:     completedAt,
		Status:          "completed",
		TracksProcessed: int64(processed),
		TracksUpdated:   int64(updated),
		ID:              syncID,
	}); err != nil {
		tx.Rollback()
		return fmt.Errorf("complete sync: %w", err)
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

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func convertTrack(tr app.Track) db.UpsertTrackParams {
	return db.UpsertTrackParams{
		NavidromeID:     tr.ID,
		Title:           tr.Title,
		Artist:          tr.Artist,
		ArtistID:        nullStringValue(tr.ArtistID),
		Album:           tr.Album,
		AlbumID:         nullStringValue(tr.AlbumID),
		AlbumArtist:     nullStringValue(tr.AlbumArtist),
		Genre:           nullStringPtr(tr.Genre),
		Year:            nullIntPtr(tr.Year),
		TrackNumber:     nullIntPtr(tr.TrackNumber),
		DiscNumber:      nullIntPtr(tr.DiscNumber),
		DurationSeconds: int64(tr.Duration / time.Second),
		Bitrate:         nullIntPtr(tr.BitRate),
		FileSize:        nullInt64Ptr(tr.FileSize),
		Path:            tr.Path,
		ContentType:     nullStringPtr(tr.ContentType),
		Suffix:          tr.Suffix,
		CreatedAt:       tr.CreatedAt.Format(time.RFC3339Nano),
	}
}

func nullStringPtr(v *string) sql.NullString {
	if v == nil || strings.TrimSpace(*v) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}

func nullStringValue(v string) sql.NullString {
	if strings.TrimSpace(v) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

func nullIntPtr(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

func nullInt64Ptr(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}
