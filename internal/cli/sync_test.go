package cli

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/bowmanmike/playlistgen/internal/app"
	"github.com/bowmanmike/playlistgen/internal/navidrome"
	"github.com/bowmanmike/playlistgen/internal/storage/sqlite"
)

func TestRunSync(t *testing.T) {
	t.Run("requires url", func(t *testing.T) {
		opts := newOptions()
		opts.dbPath = ""
		cmd := &cobra.Command{}
		if err := runSync(context.Background(), cmd, opts); err == nil {
			t.Fatalf("expected error for missing url")
		}
	})

	t.Run("requires credentials", func(t *testing.T) {
		opts := newOptions()
		opts.navidromeURL = "https://navidrome.local"
		cmd := &cobra.Command{}
		if err := runSync(context.Background(), cmd, opts); err == nil {
			t.Fatalf("expected error for missing credentials")
		}
	})

	t.Run("success logs count", func(t *testing.T) {
		out := &bytes.Buffer{}
		cmd := &cobra.Command{}
		cmd.SetOut(out)

		store := &trackStoreStub{}
		dbPath := filepath.Join(t.TempDir(), "tracks", "db.sqlite")
		opts := &options{
			navidromeURL:      "https://navidrome.local",
			navidromeUsername: "user",
			navidromePassword: "pass",
			dbPath:            dbPath,
			newNavidromeClient: func(cfg navidrome.Config) (app.NavidromePort, error) {
				return navidromeClientFunc(func(ctx context.Context) ([]app.Track, error) {
					return []app.Track{{ID: "1"}, {ID: "2"}}, nil
				}), nil
			},
			newStore: func(cfg sqlite.Config) (app.TrackStore, error) {
				if cfg.Path != dbPath {
					t.Fatalf("unexpected db path %s", cfg.Path)
				}
				return store, nil
			},
			newApp: func(deps app.Dependencies) (*app.App, error) {
				return app.New(deps)
			},
		}

		if err := runSync(context.Background(), cmd, opts); err != nil {
			t.Fatalf("runSync error: %v", err)
		}

		got := out.String()
		for _, want := range []string{
			"Navidrome URL: https://navidrome.local",
			"Navidrome user: user",
			"Track store path: " + dbPath,
			"Updated 2 tracks (skipped 0, deleted 0)",
			"Fetched 2 tracks",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("expected output to contain %q, actual: %q", want, got)
			}
		}
		if !store.saved {
			t.Fatalf("expected store to save tracks")
		}
	})

	t.Run("uses environment fallback", func(t *testing.T) {
		t.Setenv("NAVIDROME_URL", "https://navidrome.local")
		t.Setenv("NAVIDROME_USERNAME", "envuser")
		t.Setenv("NAVIDROME_PASSWORD", "envpass")

		out := &bytes.Buffer{}
		cmd := &cobra.Command{}
		cmd.SetOut(out)

		opts := &options{
			newNavidromeClient: func(cfg navidrome.Config) (app.NavidromePort, error) {
				if cfg.Username != "envuser" || cfg.Password != "envpass" {
					t.Fatalf("expected env credentials, got %+v", cfg)
				}
				return navidromeClientFunc(func(ctx context.Context) ([]app.Track, error) {
					return []app.Track{{ID: "1"}}, nil
				}), nil
			},
			newApp: func(deps app.Dependencies) (*app.App, error) {
				return app.New(deps)
			},
		}

		if err := runSync(context.Background(), cmd, opts); err != nil {
			t.Fatalf("runSync error: %v", err)
		}
	})

	t.Run("propagates client errors", func(t *testing.T) {
		cmd := &cobra.Command{}
		opts := &options{
			navidromeURL:      "https://navidrome.local",
			navidromeUsername: "user",
			navidromePassword: "pass",
			dbPath:            "",
			newNavidromeClient: func(cfg navidrome.Config) (app.NavidromePort, error) {
				return nil, errors.New("boom")
			},
		}

		if err := runSync(context.Background(), cmd, opts); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("propagates app errors", func(t *testing.T) {
		cmd := &cobra.Command{}
		opts := &options{
			navidromeURL:      "https://navidrome.local",
			navidromeUsername: "user",
			navidromePassword: "pass",
			dbPath:            "",
			newNavidromeClient: func(cfg navidrome.Config) (app.NavidromePort, error) {
				return navidromeClientFunc(func(ctx context.Context) ([]app.Track, error) {
					return nil, nil
				}), nil
			},
			newApp: func(deps app.Dependencies) (*app.App, error) {
				return nil, errors.New("app error")
			},
		}

		if err := runSync(context.Background(), cmd, opts); err == nil {
			t.Fatalf("expected error")
		}
	})
}

type navidromeClientFunc func(context.Context) ([]app.Track, error)

func (f navidromeClientFunc) ListTracks(ctx context.Context) ([]app.Track, error) {
	return f(ctx)
}

type trackStoreStub struct {
	saved bool
}

func (s *trackStoreStub) SaveTracks(ctx context.Context, tracks []app.Track) (app.SaveStats, error) {
	s.saved = true
	return app.SaveStats{Updated: len(tracks)}, nil
}
