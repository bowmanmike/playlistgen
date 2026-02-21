package sqlite

import (
	"context"
	"database/sql"
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

	var syncCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM navidrome_syncs").Scan(&syncCount); err != nil {
		t.Fatalf("count syncs: %v", err)
	}
	if syncCount != 3 {
		t.Fatalf("expected 3 sync records, got %d", syncCount)
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
	expected := [][2]int{{1, 1}, {1, 0}, {1, 1}}
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
	if err := db.QueryRow("SELECT id, status, completed_at FROM navidrome_syncs ORDER BY id DESC LIMIT 1").Scan(&lastSyncID, &status, &completedAt); err != nil {
		t.Fatalf("query last sync meta: %v", err)
	}
	if status != "completed" || completedAt == "" {
		t.Fatalf("sync status not recorded")
	}

	var syncFK int64
	var lastSynced string
	if err := db.QueryRow("SELECT sync_id, last_synced_at FROM navidrome_track_sync_status WHERE navidrome_id=?", "nav1").Scan(&syncFK, &lastSynced); err != nil {
		t.Fatalf("query track sync status: %v", err)
	}
	if syncFK != lastSyncID || lastSynced == "" {
		t.Fatalf("unexpected track sync status")
	}
}
