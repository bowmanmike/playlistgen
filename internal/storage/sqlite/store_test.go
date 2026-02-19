package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

import "github.com/bowmanmike/playlistgen/internal/app"

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

	tracks := []app.Track{{ID: "1", Title: "Song", Duration: 120 * time.Second}}
	if err := store.SaveTracks(context.Background(), tracks); err != nil {
		t.Fatalf("save tracks: %v", err)
	}

	// upsert
	tracks = []app.Track{{ID: "1", Title: "New", Duration: 180 * time.Second}}
	if err := store.SaveTracks(context.Background(), tracks); err != nil {
		t.Fatalf("save tracks second: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	defer db.Close()

	var title string
	var duration int
	if err := db.QueryRow("SELECT title, duration_seconds FROM tracks WHERE id=?", "1").Scan(&title, &duration); err != nil {
		t.Fatalf("query track: %v", err)
	}

	if title != "New" || duration != 180 {
		t.Fatalf("unexpected record title=%s duration=%d", title, duration)
	}
}
