package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/bowmanmike/playlistgen/internal/app"
)

func TestNewRequiresPath(t *testing.T) {
	if _, err := New(Config{}); err == nil {
		t.Fatalf("expected error when path missing")
	}
}

func TestSaveTracks(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tracks.db")
	store, err := New(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	genre := "Rock"
	year := 2020
	trackNo := 3
	discNo := 1
	bitrate := 320
	size := int64(123456)
	contentType := "audio/flac"
	baseCreated := time.Unix(1000, 0)
	initialChanged := time.Unix(1500, 0)
	tracks := []app.Track{{
		ID:          "nav1",
		Title:       "Song",
		Artist:      "Artist",
		ArtistID:    "artist1",
		Album:       "Album",
		AlbumID:     "album1",
		AlbumArtist: "AlbumArtist",
		Genre:       &genre,
		Year:        &year,
		TrackNumber: &trackNo,
		DiscNumber:  &discNo,
		Duration:    120 * time.Second,
		BitRate:     &bitrate,
		FileSize:    &size,
		Path:        "/music/song.mp3",
		ContentType: &contentType,
		Suffix:      "flac",
		CreatedAt:   baseCreated,
		UpdatedAt:   initialChanged,
	}}
	if _, err := store.SaveTracks(context.Background(), tracks); err != nil {
		t.Fatalf("save tracks: %v", err)
	}

	// upsert
	duration := 180 * time.Second
	tracks = []app.Track{{
		ID:          "nav1",
		Title:       "New",
		Artist:      "Artist",
		ArtistID:    "artist1",
		Album:       "Album",
		AlbumID:     "album1",
		AlbumArtist: "AlbumArtist",
		Genre:       &genre,
		Year:        &year,
		TrackNumber: &trackNo,
		DiscNumber:  &discNo,
		Duration:    duration,
		BitRate:     &bitrate,
		FileSize:    &size,
		Path:        "/music/song.mp3",
		ContentType: &contentType,
		Suffix:      "flac",
		CreatedAt:   baseCreated,
		UpdatedAt:   initialChanged,
	}}
	if _, err := store.SaveTracks(context.Background(), tracks); err != nil {
		t.Fatalf("save tracks second: %v", err)
	}

	// change upstream metadata
	changedAgain := time.Now().Add(2 * time.Hour)
	tracks = []app.Track{{
		ID:          "nav1",
		Title:       "Newest",
		Artist:      "Artist",
		ArtistID:    "artist1",
		Album:       "Album",
		AlbumID:     "album1",
		AlbumArtist: "AlbumArtist",
		Genre:       &genre,
		Year:        &year,
		TrackNumber: &trackNo,
		DiscNumber:  &discNo,
		Duration:    duration,
		BitRate:     &bitrate,
		FileSize:    &size,
		Path:        "/music/song.mp3",
		ContentType: &contentType,
		Suffix:      "flac",
		CreatedAt:   baseCreated,
		UpdatedAt:   changedAgain,
	}}
	if _, err := store.SaveTracks(context.Background(), tracks); err != nil {
		t.Fatalf("save tracks third: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer db.Close()

	var title string
	var durationSec int
	var artist string
	var bitrateOut sql.NullInt64
	var created string
	if err := db.QueryRow("SELECT title, duration_seconds, artist, bitrate, created_at FROM tracks WHERE navidrome_id=?", "nav1").Scan(&title, &durationSec, &artist, &bitrateOut, &created); err != nil {
		t.Fatalf("query track: %v", err)
	}

	if title != "Newest" || durationSec != 180 || artist != "Artist" || !bitrateOut.Valid || bitrateOut.Int64 != int64(bitrate) {
		t.Fatalf("unexpected record")
	}
	if created == "" {
		t.Fatalf("expected created timestamp")
	}

	nav1Latest := tracks[0]
	nav1Latest.UpdatedAt = time.Time{}
	secondTrack := app.Track{
		ID:          "nav2",
		Title:       "Second",
		Artist:      "Another Artist",
		ArtistID:    "artist2",
		Album:       "Second Album",
		AlbumID:     "album2",
		AlbumArtist: "Another Artist",
		Genre:       &genre,
		Year:        &year,
		TrackNumber: &trackNo,
		DiscNumber:  &discNo,
		Duration:    200 * time.Second,
		BitRate:     &bitrate,
		FileSize:    &size,
		Path:        "/music/second.mp3",
		ContentType: &contentType,
		Suffix:      "mp3",
		CreatedAt:   baseCreated.Add(time.Hour),
	}

	addStats, err := store.SaveTracks(context.Background(), []app.Track{nav1Latest, secondTrack})
	if err != nil {
		t.Fatalf("save tracks add second: %v", err)
	}
	if addStats.Updated != 1 || addStats.Deleted != 0 {
		t.Fatalf("unexpected stats adding track: %+v", addStats)
	}

	secondTrack.UpdatedAt = time.Time{}
	finalStats, err := store.SaveTracks(context.Background(), []app.Track{secondTrack})
	if err != nil {
		t.Fatalf("delete missing tracks: %v", err)
	}
	if finalStats.Deleted != 1 || finalStats.Updated != 0 {
		t.Fatalf("unexpected stats deleting track: %+v", finalStats)
	}

	var syncCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM navidrome_syncs").Scan(&syncCount); err != nil {
		t.Fatalf("count syncs: %v", err)
	}
	if syncCount != 5 {
		t.Fatalf("expected 5 sync records, got %d", syncCount)
	}

	rows, err := db.Query("SELECT tracks_processed, tracks_updated FROM navidrome_syncs ORDER BY id")
	if err != nil {
		t.Fatalf("query sync stats: %v", err)
	}
	defer rows.Close()
	var stats [][2]int
	for rows.Next() {
		var processed, updated int
		if err := rows.Scan(&processed, &updated); err != nil {
			t.Fatalf("scan stats: %v", err)
		}
		stats = append(stats, [2]int{processed, updated})
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("stats rows err: %v", err)
	}
	expected := [][2]int{{1, 1}, {1, 0}, {1, 1}, {2, 1}, {1, 0}}
	if len(stats) != len(expected) {
		t.Fatalf("unexpected stats len %d", len(stats))
	}
	for i := range expected {
		if stats[i] != expected[i] {
			t.Fatalf("unexpected stats[%d]=%v want %v", i, stats[i], expected[i])
		}
	}

	var lastSyncID int64
	var status, completedAt string
	var tracksDeleted int
	if err := db.QueryRow("SELECT id, status, completed_at, tracks_deleted FROM navidrome_syncs ORDER BY id DESC LIMIT 1").Scan(&lastSyncID, &status, &completedAt, &tracksDeleted); err != nil {
		t.Fatalf("query last sync meta: %v", err)
	}
	if status != "completed" || completedAt == "" {
		t.Fatalf("sync status not recorded")
	}
	if tracksDeleted != 1 {
		t.Fatalf("expected one deleted track recorded, got %d", tracksDeleted)
	}

	var trackCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM tracks").Scan(&trackCount); err != nil {
		t.Fatalf("count tracks: %v", err)
	}
	if trackCount != 1 {
		t.Fatalf("expected only one track remaining, got %d", trackCount)
	}
	var remainingNavID string
	if err := db.QueryRow("SELECT navidrome_id FROM tracks").Scan(&remainingNavID); err != nil {
		t.Fatalf("query remaining track: %v", err)
	}
	if remainingNavID != "nav2" {
		t.Fatalf("expected nav2 to remain, got %s", remainingNavID)
	}
	var jobCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM track_audio_analysis WHERE track_id IN (SELECT id FROM tracks WHERE navidrome_id='nav1')").Scan(&jobCount); err != nil {
		t.Fatalf("count deleted audio jobs: %v", err)
	}
	if jobCount != 0 {
		t.Fatalf("expected audio job rows removed for nav1, got %d", jobCount)
	}
	if err := db.QueryRow("SELECT sync_id, last_synced_at FROM navidrome_track_sync_status WHERE navidrome_id=?", "nav1").Scan(new(sql.NullInt64), new(sql.NullString)); err != sql.ErrNoRows {
		t.Fatalf("expected nav1 sync status removed, got %v", err)
	}

	var pendingAudio int
	if err := db.QueryRow("SELECT COUNT(*) FROM track_audio_analysis WHERE track_id=(SELECT id FROM tracks WHERE navidrome_id=?)", "nav2").Scan(&pendingAudio); err != nil {
		t.Fatalf("count audio jobs: %v", err)
	}
	if pendingAudio == 0 {
		t.Fatalf("expected at least one audio job row for nav2")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM track_audio_analysis WHERE track_id=(SELECT id FROM tracks WHERE navidrome_id=?) AND status='pending'`, "nav2").Scan(&pendingAudio); err != nil {
		t.Fatalf("count pending audio jobs: %v", err)
	}
	if pendingAudio == 0 {
		t.Fatalf("expected pending audio jobs for nav2")
	}

	var pendingEmbedding int
	if err := db.QueryRow(`SELECT COUNT(*) FROM track_embedding_jobs WHERE track_id=(SELECT id FROM tracks WHERE navidrome_id=?)`, "nav2").Scan(&pendingEmbedding); err != nil {
		t.Fatalf("count embedding jobs: %v", err)
	}
	if pendingEmbedding == 0 {
		t.Fatalf("expected embedding job row for nav2")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM track_embedding_jobs WHERE track_id=(SELECT id FROM tracks WHERE navidrome_id=?) AND status='pending'`, "nav2").Scan(&pendingEmbedding); err != nil {
		t.Fatalf("count pending embedding jobs: %v", err)
	}
	if pendingEmbedding == 0 {
		t.Fatalf("expected pending embedding jobs for nav2")
	}
}

func TestAudioJobLifecycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "jobs.db")
	store, err := New(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	track := app.Track{
		ID:        "jobtrack",
		Title:     "Job Track",
		Artist:    "Artist",
		Album:     "Album",
		CreatedAt: time.Unix(2000, 0),
		Duration:  90 * time.Second,
		Path:      "/music/jobtrack.flac",
		Suffix:    "flac",
	}
	if _, err := store.SaveTracks(context.Background(), []app.Track{track}); err != nil {
		t.Fatalf("save tracks: %v", err)
	}

	jobs, err := store.ListPendingAudioJobs(context.Background(), 10)
	if err != nil {
		t.Fatalf("list pending jobs: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatalf("expected at least one pending job")
	}
	job := jobs[0]
	if job.Track.ID != track.ID || job.Track.Path != track.Path {
		t.Fatalf("job track mismatch: %+v", job.Track)
	}

	if err := store.CompleteAudioJob(context.Background(), job.ID); err != nil {
		t.Fatalf("complete job: %v", err)
	}

	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer raw.Close()

	var status string
	if err := raw.QueryRow("SELECT status FROM track_audio_analysis WHERE id=?", job.ID).Scan(&status); err != nil {
		t.Fatalf("query job status: %v", err)
	}
	if status != "completed" {
		t.Fatalf("expected job completed, got %s", status)
	}

	jobErr := errors.New("simulated failure")
	if err := store.FailAudioJob(context.Background(), job.ID, jobErr); err != nil {
		t.Fatalf("fail job: %v", err)
	}
	if err := raw.QueryRow("SELECT status, error FROM track_audio_analysis WHERE id=?", job.ID).Scan(&status, new(sql.NullString)); err != nil {
		t.Fatalf("query failed job: %v", err)
	}
	if status != "failed" {
		t.Fatalf("expected job failed, got %s", status)
	}
}

func TestSaveTracksDoesNotCreateDuplicatePendingJobs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "dedupe.db")
	store, err := New(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	track := app.Track{
		ID:        "dupetrack",
		Title:     "Dupe Track",
		Artist:    "Artist",
		Album:     "Album",
		CreatedAt: time.Unix(3000, 0),
		UpdatedAt: time.Unix(3500, 0),
		Duration:  100 * time.Second,
		Path:      "/music/dupe.flac",
		Suffix:    "flac",
	}
	if _, err := store.SaveTracks(context.Background(), []app.Track{track}); err != nil {
		t.Fatalf("save tracks first: %v", err)
	}

	track.UpdatedAt = track.UpdatedAt.Add(time.Hour)
	if _, err := store.SaveTracks(context.Background(), []app.Track{track}); err != nil {
		t.Fatalf("save tracks second: %v", err)
	}

	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer raw.Close()

	var activeAudio int
	if err := raw.QueryRow(`
		SELECT COUNT(*)
		FROM track_audio_analysis
		WHERE track_id = (SELECT id FROM tracks WHERE navidrome_id = ?)
		  AND status IN ('pending', 'processing')
	`, track.ID).Scan(&activeAudio); err != nil {
		t.Fatalf("count active audio jobs: %v", err)
	}
	if activeAudio != 1 {
		t.Fatalf("expected 1 active audio job, got %d", activeAudio)
	}

	var activeEmbedding int
	if err := raw.QueryRow(`
		SELECT COUNT(*)
		FROM track_embedding_jobs
		WHERE track_id = (SELECT id FROM tracks WHERE navidrome_id = ?)
		  AND status IN ('pending', 'processing')
	`, track.ID).Scan(&activeEmbedding); err != nil {
		t.Fatalf("count active embedding jobs: %v", err)
	}
	if activeEmbedding != 1 {
		t.Fatalf("expected 1 active embedding job, got %d", activeEmbedding)
	}
}

func TestClaimPendingAudioJobsReclaimsStaleProcessingJobs(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stale-claim.db")
	store, err := New(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	track := app.Track{
		ID:        "stale-claim-track",
		Title:     "Stale Claim Track",
		Artist:    "Artist",
		Album:     "Album",
		CreatedAt: time.Unix(4000, 0),
		UpdatedAt: time.Unix(4500, 0),
		Duration:  90 * time.Second,
		Path:      "/music/stale.flac",
		Suffix:    "flac",
	}
	if _, err := store.SaveTracks(context.Background(), []app.Track{track}); err != nil {
		t.Fatalf("save tracks: %v", err)
	}

	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer raw.Close()

	staleClaimedAt := time.Now().Add(-2 * time.Hour).Format(time.RFC3339Nano)
	if _, err := raw.Exec(`
		UPDATE track_audio_analysis
		SET status = 'processing', claimed_at = ?, claimed_by = ?
		WHERE track_id = (SELECT id FROM tracks WHERE navidrome_id = ?)
	`, staleClaimedAt, "runner-old", track.ID); err != nil {
		t.Fatalf("seed stale claim: %v", err)
	}

	jobs, err := store.ClaimPendingAudioJobs(context.Background(), ClaimOptions{
		Limit:      1,
		ClaimedBy:  "runner-new",
		StaleAfter: time.Minute,
		Now:        time.Now(),
	})
	if err != nil {
		t.Fatalf("claim pending jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 claimed job, got %d", len(jobs))
	}
	if jobs[0].Track.ID != track.ID {
		t.Fatalf("expected claimed track %s, got %s", track.ID, jobs[0].Track.ID)
	}
}

func TestStoreClaimPendingAudioJobsSetsClaimFields(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "claim-fields.db")
	store, err := New(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	track := app.Track{
		ID:        "claim-track",
		Title:     "Claim Track",
		Artist:    "Artist",
		Album:     "Album",
		CreatedAt: time.Unix(5000, 0),
		Duration:  70 * time.Second,
		Path:      "/music/claim.flac",
		Suffix:    "flac",
	}
	if _, err := store.SaveTracks(context.Background(), []app.Track{track}); err != nil {
		t.Fatalf("save tracks: %v", err)
	}

	now := time.Now().UTC()
	jobs, err := store.ClaimPendingAudioJobs(context.Background(), ClaimOptions{
		Limit:      1,
		ClaimedBy:  "runner-a",
		StaleAfter: time.Minute,
		Now:        now,
	})
	if err != nil {
		t.Fatalf("claim pending jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 claimed job, got %d", len(jobs))
	}

	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer raw.Close()

	var status string
	var claimedAt sql.NullString
	var claimedBy sql.NullString
	if err := raw.QueryRow("SELECT status, claimed_at, claimed_by FROM track_audio_analysis WHERE id=?", jobs[0].ID).Scan(&status, &claimedAt, &claimedBy); err != nil {
		t.Fatalf("query claimed job: %v", err)
	}
	if status != "processing" {
		t.Fatalf("expected processing status, got %s", status)
	}
	if !claimedAt.Valid || claimedAt.String != now.Format(time.RFC3339Nano) {
		t.Fatalf("unexpected claimed_at %v", claimedAt)
	}
	if !claimedBy.Valid || claimedBy.String != "runner-a" {
		t.Fatalf("unexpected claimed_by %v", claimedBy)
	}
}

func TestCompleteAudioJobClearsClaimFields(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "complete-clears-claim.db")
	store, err := New(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	jobID := seedClaimedAudioJob(t, store, dbPath, "complete-claim")
	if err := store.CompleteAudioJob(context.Background(), jobID); err != nil {
		t.Fatalf("complete job: %v", err)
	}

	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer raw.Close()

	var status string
	var processedAt sql.NullString
	var claimedAt sql.NullString
	var claimedBy sql.NullString
	if err := raw.QueryRow("SELECT status, processed_at, claimed_at, claimed_by FROM track_audio_analysis WHERE id=?", jobID).Scan(&status, &processedAt, &claimedAt, &claimedBy); err != nil {
		t.Fatalf("query completed job: %v", err)
	}
	if status != "completed" {
		t.Fatalf("expected completed status, got %s", status)
	}
	if !processedAt.Valid {
		t.Fatalf("expected processed timestamp")
	}
	if claimedAt.Valid || claimedBy.Valid {
		t.Fatalf("expected claim fields cleared, got claimed_at=%v claimed_by=%v", claimedAt, claimedBy)
	}
}

func TestFailAudioJobRetainsAttemptsAndClearsOwnership(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "fail-clears-claim.db")
	store, err := New(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	jobID := seedClaimedAudioJob(t, store, dbPath, "fail-claim")
	if err := store.FailAudioJob(context.Background(), jobID, errors.New("boom")); err != nil {
		t.Fatalf("fail job: %v", err)
	}

	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer raw.Close()

	var status string
	var attempts int
	var jobErr sql.NullString
	var claimedAt sql.NullString
	var claimedBy sql.NullString
	if err := raw.QueryRow("SELECT status, attempts, error, claimed_at, claimed_by FROM track_audio_analysis WHERE id=?", jobID).Scan(&status, &attempts, &jobErr, &claimedAt, &claimedBy); err != nil {
		t.Fatalf("query failed job: %v", err)
	}
	if status != "failed" {
		t.Fatalf("expected failed status, got %s", status)
	}
	if attempts != 1 {
		t.Fatalf("expected attempts=1, got %d", attempts)
	}
	if !jobErr.Valid || jobErr.String != "boom" {
		t.Fatalf("unexpected error field %v", jobErr)
	}
	if claimedAt.Valid || claimedBy.Valid {
		t.Fatalf("expected claim fields cleared, got claimed_at=%v claimed_by=%v", claimedAt, claimedBy)
	}
}

func TestClaimPendingAudioJobsDoesNotReturnSameJobTwice(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "concurrent-claims.db")
	storeA, err := New(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("new store A: %v", err)
	}
	t.Cleanup(func() { storeA.Close() })

	storeB, err := New(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("new store B: %v", err)
	}
	t.Cleanup(func() { storeB.Close() })

	tracks := make([]app.Track, 0, 4)
	for i := 0; i < 4; i++ {
		tracks = append(tracks, app.Track{
			ID:        "concurrent-track-" + string(rune('a'+i)),
			Title:     "Concurrent Track",
			Artist:    "Artist",
			Album:     "Album",
			CreatedAt: time.Unix(int64(6000+i), 0),
			Duration:  80 * time.Second,
			Path:      "/music/concurrent.flac",
			Suffix:    "flac",
		})
	}
	if _, err := storeA.SaveTracks(context.Background(), tracks); err != nil {
		t.Fatalf("save tracks: %v", err)
	}

	type claimResult struct {
		jobs []AudioJob
		err  error
	}

	now := time.Now().UTC()
	resultCh := make(chan claimResult, 2)
	go func() {
		jobs, err := storeA.ClaimPendingAudioJobs(context.Background(), ClaimOptions{Limit: 2, ClaimedBy: "runner-a", StaleAfter: time.Minute, Now: now})
		resultCh <- claimResult{jobs: jobs, err: err}
	}()
	go func() {
		jobs, err := storeB.ClaimPendingAudioJobs(context.Background(), ClaimOptions{Limit: 2, ClaimedBy: "runner-b", StaleAfter: time.Minute, Now: now})
		resultCh <- claimResult{jobs: jobs, err: err}
	}()

	first := <-resultCh
	second := <-resultCh
	if first.err != nil {
		t.Fatalf("first claim: %v", first.err)
	}
	if second.err != nil {
		t.Fatalf("second claim: %v", second.err)
	}

	seen := make(map[int64]string)
	for _, batch := range []claimResult{first, second} {
		for _, job := range batch.jobs {
			if owner, ok := seen[job.ID]; ok {
				t.Fatalf("job %d claimed twice by %s and another runner", job.ID, owner)
			}
			seen[job.ID] = job.Track.ID
		}
	}
	if len(seen) != 4 {
		t.Fatalf("expected 4 unique claimed jobs, got %d", len(seen))
	}
}

func seedClaimedAudioJob(t *testing.T, store *Store, dbPath, navidromeID string) int64 {
	t.Helper()

	track := app.Track{
		ID:        navidromeID,
		Title:     "Seed Claimed Track",
		Artist:    "Artist",
		Album:     "Album",
		CreatedAt: time.Unix(7000, 0),
		Duration:  75 * time.Second,
		Path:      "/music/seed.flac",
		Suffix:    "flac",
	}
	if _, err := store.SaveTracks(context.Background(), []app.Track{track}); err != nil {
		t.Fatalf("save seed track: %v", err)
	}

	jobs, err := store.ClaimPendingAudioJobs(context.Background(), ClaimOptions{
		Limit:      1,
		ClaimedBy:  "seed-runner",
		StaleAfter: time.Minute,
		Now:        time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("claim seed job: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 seed job, got %d", len(jobs))
	}
	return jobs[0].ID
}

func TestUpsertTrackAudioFeaturesPersistsMeasuredAndReplayGainValues(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audio-features.db")
	store, err := New(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	track := app.Track{
		ID:        "feature-track",
		Title:     "Feature Track",
		Artist:    "Artist",
		Album:     "Album",
		CreatedAt: time.Unix(8000, 0),
		Duration:  123 * time.Second,
		Path:      "/music/feature.flac",
		Suffix:    "flac",
	}
	if _, err := store.SaveTracks(context.Background(), []app.Track{track}); err != nil {
		t.Fatalf("save track: %v", err)
	}

	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer raw.Close()

	var trackID int64
	if err := raw.QueryRow("SELECT id FROM tracks WHERE navidrome_id = ?", track.ID).Scan(&trackID); err != nil {
		t.Fatalf("select track id: %v", err)
	}

	measuredLoudness := -11.2
	measuredPeak := 0.92
	replayGainTrack := -7.15
	replayGainTrackPeak := 0.88
	replayGainAlbum := -6.01
	replayGainAlbumPeak := 0.9
	effectiveGain := replayGainAlbum
	effectivePeak := replayGainAlbumPeak
	if err := store.UpsertTrackAudioFeatures(context.Background(), AudioFeatureRecord{
		TrackID:                trackID,
		AnalyzedAt:             time.Now().UTC(),
		FileDurationSeconds:    123.45,
		MeasuredIntegratedLUFS: &measuredLoudness,
		MeasuredTruePeak:       &measuredPeak,
		ReplayGainTrackGainDB:  &replayGainTrack,
		ReplayGainTrackPeak:    &replayGainTrackPeak,
		ReplayGainAlbumGainDB:  &replayGainAlbum,
		ReplayGainAlbumPeak:    &replayGainAlbumPeak,
		EffectiveGainDB:        &effectiveGain,
		EffectivePeak:          &effectivePeak,
		EffectiveGainSource:    "replaygain_album",
		EffectivePeakSource:    "replaygain_album",
	}); err != nil {
		t.Fatalf("upsert audio features: %v", err)
	}

	var count int
	if err := raw.QueryRow("SELECT COUNT(*) FROM track_audio_features WHERE track_id = ?", trackID).Scan(&count); err != nil {
		t.Fatalf("count feature rows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 feature row, got %d", count)
	}
}

func TestAudioProcessingRunLifecycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audio-runs.db")
	store, err := New(Config{Path: dbPath})
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	startedAt := time.Now().UTC()
	runID, err := store.StartAudioProcessingRun(context.Background(), startedAt)
	if err != nil {
		t.Fatalf("start run: %v", err)
	}

	completedAt := startedAt.Add(2 * time.Minute)
	if err := store.CompleteAudioProcessingRun(context.Background(), runID, AudioProcessingRunSummary{
		CompletedAt:   completedAt,
		Status:        "completed",
		JobsClaimed:   5,
		JobsCompleted: 4,
		JobsFailed:    1,
	}); err != nil {
		t.Fatalf("complete run: %v", err)
	}

	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer raw.Close()

	var status string
	var jobsClaimed int
	var jobsCompleted int
	var jobsFailed int
	if err := raw.QueryRow(`
		SELECT status, jobs_claimed, jobs_completed, jobs_failed
		FROM audio_processing_runs
		WHERE id = ?
	`, runID).Scan(&status, &jobsClaimed, &jobsCompleted, &jobsFailed); err != nil {
		t.Fatalf("query run row: %v", err)
	}
	if status != "completed" || jobsClaimed != 5 || jobsCompleted != 4 || jobsFailed != 1 {
		t.Fatalf("unexpected run row status=%s claimed=%d completed=%d failed=%d", status, jobsClaimed, jobsCompleted, jobsFailed)
	}
}
