package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/bowmanmike/playlistgen/internal/navidrome"
)

func TestRunSync(t *testing.T) {
	t.Run("requires url", func(t *testing.T) {
		opts := newOptions()
		cmd := &cobra.Command{}
		if err := runSync(context.Background(), cmd, opts); err == nil {
			t.Fatalf("expected error for missing url")
		}
	})

	t.Run("success logs count", func(t *testing.T) {
		out := &bytes.Buffer{}
		cmd := &cobra.Command{}
		cmd.SetOut(out)

		opts := &options{
			navidromeURL:    "https://navidrome.local",
			navidromeAPIKey: "key",
			newNavidromeClient: func(cfg navidrome.Config) (navidromeAPI, error) {
				return navidromeClientFunc(func(ctx context.Context) ([]navidrome.Track, error) {
					return []navidrome.Track{{ID: "1"}, {ID: "2"}}, nil
				}), nil
			},
		}

		if err := runSync(context.Background(), cmd, opts); err != nil {
			t.Fatalf("runSync error: %v", err)
		}

		if got := out.String(); got != "Fetched 2 tracks\n" {
			t.Fatalf("unexpected output %q", got)
		}
	})

	t.Run("propagates client errors", func(t *testing.T) {
		cmd := &cobra.Command{}
		opts := &options{
			navidromeURL: "https://navidrome.local",
			newNavidromeClient: func(cfg navidrome.Config) (navidromeAPI, error) {
				return nil, errors.New("boom")
			},
		}

		if err := runSync(context.Background(), cmd, opts); err == nil {
			t.Fatalf("expected error")
		}
	})
}

type navidromeClientFunc func(context.Context) ([]navidrome.Track, error)

func (f navidromeClientFunc) ListTracks(ctx context.Context) ([]navidrome.Track, error) {
	return f(ctx)
}
