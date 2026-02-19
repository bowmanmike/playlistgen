package app

import (
	"context"
	"errors"
	"testing"
)

func TestNewAppRequiresNavidrome(t *testing.T) {
	_, err := New(Dependencies{})
	if err == nil {
		t.Fatalf("expected error when navidrome port missing")
	}
}

func TestSyncTracks(t *testing.T) {
	tracks := []Track{{ID: "1"}, {ID: "2"}}

	t.Run("fetch error", func(t *testing.T) {
		app, err := New(Dependencies{
			Navidrome: navidromeStub(func(ctx context.Context) ([]Track, error) {
				return nil, errors.New("boom")
			}),
		})
		if err != nil {
			t.Fatalf("new app: %v", err)
		}

		if _, err := app.SyncTracks(context.Background()); err == nil {
			t.Fatalf("expected fetch error")
		}
	})

	t.Run("persists when store configured", func(t *testing.T) {
		store := &storeStub{}
		app, err := New(Dependencies{
			Navidrome: navidromeStub(func(ctx context.Context) ([]Track, error) {
				return tracks, nil
			}),
			Store: store,
		})
		if err != nil {
			t.Fatalf("new app: %v", err)
		}

		n, err := app.SyncTracks(context.Background())
		if err != nil {
			t.Fatalf("sync tracks: %v", err)
		}
		if n != len(tracks) {
			t.Fatalf("expected %d got %d", len(tracks), n)
		}
		if !store.saved {
			t.Fatalf("expected store to be called")
		}
	})

	t.Run("store error propagated", func(t *testing.T) {
		app, err := New(Dependencies{
			Navidrome: navidromeStub(func(ctx context.Context) ([]Track, error) {
				return tracks, nil
			}),
			Store: storeStubErr{err: errors.New("save failed")},
		})
		if err != nil {
			t.Fatalf("new app: %v", err)
		}
		if _, err := app.SyncTracks(context.Background()); err == nil {
			t.Fatalf("expected store error")
		}
	})
}

type navidromeStub func(context.Context) ([]Track, error)

func (n navidromeStub) ListTracks(ctx context.Context) ([]Track, error) {
	return n(ctx)
}

type storeStub struct {
	saved bool
}

func (s *storeStub) SaveTracks(ctx context.Context, tracks []Track) error {
	s.saved = true
	return nil
}

type storeStubErr struct {
	err error
}

func (e storeStubErr) SaveTracks(ctx context.Context, tracks []Track) error {
	return e.err
}
