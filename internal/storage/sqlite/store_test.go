package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

import (
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
		CreatedAt:   time.Unix(1000, 0),
	}}
	if err := store.SaveTracks(context.Background(), tracks); err != nil {
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
		CreatedAt:   time.Unix(1000, 0),
	}}
	if err := store.SaveTracks(context.Background(), tracks); err != nil {
		t.Fatalf("save tracks second: %v", err)
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

	if title != "New" || durationSec != 180 || artist != "Artist" || !bitrateOut.Valid || bitrateOut.Int64 != int64(bitrate) {
		t.Fatalf("unexpected record")
	}
	if created == "" {
		t.Fatalf("expected created timestamp")
	}
}
