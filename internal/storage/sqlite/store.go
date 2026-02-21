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
	Path                string
	ForceProcessingJobs bool
}

// Store implements app.TrackStore backed by SQLite.
type Store struct {
	db                  *sql.DB
	forceProcessingJobs bool
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

	return &Store{db: db, forceProcessingJobs: cfg.ForceProcessingJobs}, nil
}

// AudioJob represents a pending audio analysis task and its associated track.
type AudioJob struct {
	ID    int64
	Track app.Track
}

// SaveTracks inserts or replaces provided tracks and records sync metadata.
func (s *Store) SaveTracks(ctx context.Context, tracks []app.Track) (app.SaveStats, error) {
	stats := app.SaveStats{Fetched: len(tracks)}
	if len(tracks) == 0 {
		return stats, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return app.SaveStats{}, fmt.Errorf("begin tx: %w", err)
	}

	queries := db.New(tx)
	startedAt := nowUTC()
	syncID, err := queries.CreateSync(ctx, startedAt)
	if err != nil {
		tx.Rollback()
		return app.SaveStats{}, fmt.Errorf("create sync: %w", err)
	}

	statusRows, err := queries.ListTrackSyncStatus(ctx)
	if err != nil {
		tx.Rollback()
		return app.SaveStats{}, fmt.Errorf("list track sync statuses: %w", err)
	}

	statusMap := make(map[string]trackSyncStatus, len(statusRows))
	existingNavIDs := make(map[string]struct{}, len(statusRows))
	for _, row := range statusRows {
		statusMap[row.NavidromeID] = trackSyncStatus{
			trackID:      row.TrackID,
			lastSyncedAt: parseTimestamp(row.LastSyncedAt),
		}
		existingNavIDs[row.NavidromeID] = struct{}{}
	}

	processed, updated, deleted := 0, 0, 0
	remoteNavIDs := make(map[string]struct{}, len(tracks))

	for _, tr := range tracks {
		processed++
		remoteNavIDs[tr.ID] = struct{}{}
		status := statusMap[tr.ID]
		remoteChanged := trackChangedAt(tr)

		if status.trackID != 0 && !remoteChanged.IsZero() && !remoteChanged.After(status.lastSyncedAt) {
			if err := queries.UpsertTrackSyncStatus(ctx, db.UpsertTrackSyncStatusParams{
				TrackID:      status.trackID,
				NavidromeID:  tr.ID,
				LastSyncedAt: formatTimestamp(status.lastSyncedAt),
				SyncID:       syncID,
			}); err != nil {
				tx.Rollback()
				return app.SaveStats{}, fmt.Errorf("touch track sync status: %w", err)
			}
			if s.forceProcessingJobs && status.trackID != 0 {
				if err := enqueueProcessingJobs(ctx, queries, status.trackID); err != nil {
					tx.Rollback()
					return app.SaveStats{}, err
				}
			}
			continue
		}

		if err := queries.UpsertTrack(ctx, convertTrack(tr)); err != nil {
			tx.Rollback()
			return app.SaveStats{}, fmt.Errorf("upsert track: %w", err)
		}
		updated++

		trackID := status.trackID
		if trackID == 0 {
			var err error
			trackID, err = queries.SelectTrackID(ctx, tr.ID)
			if err != nil {
				tx.Rollback()
				return app.SaveStats{}, fmt.Errorf("select track id: %w", err)
			}
		}

		syncedAt := time.Now().UTC()
		if err := queries.UpsertTrackSyncStatus(ctx, db.UpsertTrackSyncStatusParams{
			TrackID:      trackID,
			NavidromeID:  tr.ID,
			LastSyncedAt: formatTimestamp(syncedAt),
			SyncID:       syncID,
		}); err != nil {
			tx.Rollback()
			return app.SaveStats{}, fmt.Errorf("update track sync status: %w", err)
		}
		statusMap[tr.ID] = trackSyncStatus{
			trackID:      trackID,
			lastSyncedAt: syncedAt,
		}

		if err := enqueueProcessingJobs(ctx, queries, trackID); err != nil {
			tx.Rollback()
			return app.SaveStats{}, err
		}
	}

	var toDelete []string
	for navID := range existingNavIDs {
		if _, ok := remoteNavIDs[navID]; !ok {
			toDelete = append(toDelete, navID)
		}
	}

	if len(toDelete) > 0 {
		if err := queries.DeleteTracksByNavidromeIDs(ctx, toDelete); err != nil {
			tx.Rollback()
			return app.SaveStats{}, fmt.Errorf("delete missing tracks: %w", err)
		}
		deleted = len(toDelete)
	}

	completedAt := sql.NullString{String: nowUTC(), Valid: true}
	if err := queries.CompleteSync(ctx, db.CompleteSyncParams{
		CompletedAt:     completedAt,
		Status:          "completed",
		TracksProcessed: int64(processed),
		TracksUpdated:   int64(updated),
		TracksDeleted:   int64(deleted),
		ID:              syncID,
	}); err != nil {
		tx.Rollback()
		return app.SaveStats{}, fmt.Errorf("complete sync: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return app.SaveStats{}, fmt.Errorf("commit tx: %w", err)
	}

	stats.Updated = updated
	stats.Skipped = processed - updated
	stats.Deleted = deleted
	return stats, nil
}

// Close releases database resources.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

type trackSyncStatus struct {
	trackID      int64
	lastSyncedAt time.Time
}

func trackChangedAt(tr app.Track) time.Time {
	if !tr.UpdatedAt.IsZero() {
		return tr.UpdatedAt
	}
	return tr.CreatedAt
}

func parseTimestamp(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	if ts, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return ts
	}
	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		return ts
	}
	return time.Time{}
}

func formatTimestamp(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.Format(time.RFC3339Nano)
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

func convertDBTrack(row db.Track) app.Track {
	return app.Track{
		ID:          row.NavidromeID,
		Title:       row.Title,
		Artist:      row.Artist,
		ArtistID:    stringValue(row.ArtistID),
		Album:       row.Album,
		AlbumID:     stringValue(row.AlbumID),
		AlbumArtist: stringValue(row.AlbumArtist),
		Genre:       stringPtrFromSQL(row.Genre),
		Year:        intPtrFromSQL(row.Year),
		TrackNumber: intPtrFromSQL(row.TrackNumber),
		DiscNumber:  intPtrFromSQL(row.DiscNumber),
		Duration:    time.Duration(row.DurationSeconds) * time.Second,
		BitRate:     intPtrFromSQL(row.Bitrate),
		FileSize:    int64PtrFromSQL(row.FileSize),
		Path:        row.Path,
		ContentType: stringPtrFromSQL(row.ContentType),
		Suffix:      row.Suffix,
		CreatedAt:   parseTimestamp(row.CreatedAt),
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

func stringValue(ns sql.NullString) string {
	if !ns.Valid {
		return ""
	}
	return ns.String
}

func stringPtrFromSQL(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	val := ns.String
	return &val
}

func intPtrFromSQL(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}
	v := int(ni.Int64)
	return &v
}

func int64PtrFromSQL(ni sql.NullInt64) *int64 {
	if !ni.Valid {
		return nil
	}
	v := ni.Int64
	return &v
}

func enqueueProcessingJobs(ctx context.Context, queries *db.Queries, trackID int64) error {
	status := "pending"
	resetValue := sql.NullString{}
	if err := queries.InsertTrackAudioJob(ctx, db.InsertTrackAudioJobParams{
		TrackID:       trackID,
		Status:        status,
		ProcessedAt:   resetValue,
		Error:         resetValue,
		Attempts:      0,
		LastAttemptAt: resetValue,
	}); err != nil {
		return fmt.Errorf("enqueue audio job: %w", err)
	}

	if err := queries.InsertTrackEmbeddingJob(ctx, db.InsertTrackEmbeddingJobParams{
		TrackID:       trackID,
		Status:        status,
		ProcessedAt:   resetValue,
		Error:         resetValue,
		Attempts:      0,
		LastAttemptAt: resetValue,
	}); err != nil {
		return fmt.Errorf("enqueue embedding job: %w", err)
	}
	return nil
}

// ListPendingAudioJobs returns pending audio jobs up to the provided limit.
func (s *Store) ListPendingAudioJobs(ctx context.Context, limit int) ([]AudioJob, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.New(s.db).ListPendingAudioJobs(ctx, int64(limit))
	if err != nil {
		return nil, fmt.Errorf("list audio jobs: %w", err)
	}
	jobs := make([]AudioJob, 0, len(rows))
	for _, row := range rows {
		jobs = append(jobs, AudioJob{
			ID:    row.JobID,
			Track: convertDBTrack(row.Track),
		})
	}
	return jobs, nil
}

// CompleteAudioJob marks an audio job as processed successfully.
func (s *Store) CompleteAudioJob(ctx context.Context, jobID int64) error {
	return s.setAudioJobStatus(ctx, jobID, "completed", nil)
}

// FailAudioJob marks an audio job as failed with the supplied error.
func (s *Store) FailAudioJob(ctx context.Context, jobID int64, jobErr error) error {
	if jobErr == nil {
		return s.setAudioJobStatus(ctx, jobID, "failed", nil)
	}
	msg := jobErr.Error()
	return s.setAudioJobStatus(ctx, jobID, "failed", &msg)
}

func (s *Store) setAudioJobStatus(ctx context.Context, jobID int64, status string, failureMessage *string) error {
	processed := sql.NullString{}
	if status == "completed" {
		processed = sql.NullString{String: nowUTC(), Valid: true}
	}
	errField := sql.NullString{}
	if failureMessage != nil && strings.TrimSpace(*failureMessage) != "" {
		errField = sql.NullString{String: *failureMessage, Valid: true}
	}

	params := db.UpdateAudioJobStatusParams{
		Status:        status,
		ProcessedAt:   processed,
		Error:         errField,
		LastAttemptAt: sql.NullString{String: nowUTC(), Valid: true},
		ID:            jobID,
	}
	if err := db.New(s.db).UpdateAudioJobStatus(ctx, params); err != nil {
		return fmt.Errorf("update audio job status: %w", err)
	}
	return nil
}
