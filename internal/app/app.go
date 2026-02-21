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
}

// NavidromePort fetches tracks from Navidrome.
type NavidromePort interface {
	ListTracks(ctx context.Context) ([]Track, error)
}

// TrackStore persists tracks to storage.
type TrackStore interface {
	SaveTracks(ctx context.Context, tracks []Track) error
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
func (a *App) SyncTracks(ctx context.Context) (int, error) {
	tracks, err := a.navidrome.ListTracks(ctx)
	if err != nil {
		return 0, fmt.Errorf("fetch tracks: %w", err)
	}

	if a.store != nil {
		if err := a.store.SaveTracks(ctx, tracks); err != nil {
			return 0, fmt.Errorf("save tracks: %w", err)
		}
	}

	return len(tracks), nil
}
