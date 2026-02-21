package app

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Track represents normalized track metadata used internally.
type Track struct {
	ID          string
	Title       string
	Artist      string
	ArtistID    string
	Album       string
	AlbumID     string
	AlbumArtist string
	Genre       *string
	Year        *int
	TrackNumber *int
	DiscNumber  *int
	Duration    time.Duration
	BitRate     *int
	FileSize    *int64
	Path        string
	ContentType *string
	Suffix      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NavidromePort fetches tracks from Navidrome.
type NavidromePort interface {
	ListTracks(ctx context.Context) ([]Track, error)
}

// TrackStore persists tracks to storage.
type TrackStore interface {
	SaveTracks(ctx context.Context, tracks []Track) (SaveStats, error)
}

// SaveStats reports sync outcomes.
type SaveStats struct {
	Fetched int
	Updated int
	Skipped int
	Deleted int
}

// Dependencies groups external adapters required by the App.
type Dependencies struct {
	Navidrome NavidromePort
	Store     TrackStore
}

// App orchestrates use-cases.
type App struct {
	navidrome NavidromePort
	store     TrackStore
}

// New creates an App with the provided dependencies.
func New(deps Dependencies) (*App, error) {
	if deps.Navidrome == nil {
		return nil, errors.New("navidrome adapter is required")
	}

	return &App{
		navidrome: deps.Navidrome,
		store:     deps.Store,
	}, nil
}

// SyncTracks pulls tracks from Navidrome and optionally persists them.
func (a *App) SyncTracks(ctx context.Context) (SaveStats, error) {
	tracks, err := a.navidrome.ListTracks(ctx)
	if err != nil {
		return SaveStats{}, fmt.Errorf("fetch tracks: %w", err)
	}

	stats := SaveStats{Fetched: len(tracks)}
	if a.store != nil {
		storeStats, err := a.store.SaveTracks(ctx, tracks)
		if err != nil {
			return SaveStats{}, fmt.Errorf("save tracks: %w", err)
		}
		stats.Updated = storeStats.Updated
		stats.Skipped = storeStats.Skipped
		stats.Deleted = storeStats.Deleted
	}

	return stats, nil
}
