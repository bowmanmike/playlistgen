package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/bowmanmike/playlistgen/internal/app"
	"github.com/bowmanmike/playlistgen/internal/navidrome"
	"github.com/bowmanmike/playlistgen/internal/storage/sqlite"
)

func newSyncCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync metadata from Navidrome",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSync(cmd.Context(), cmd, opts)
		},
	}

	return cmd
}

func runSync(ctx context.Context, cmd *cobra.Command, opts *options) error {
	opts.populateFromEnv()

	if opts.navidromeURL == "" {
		return errors.New("navidrome URL must be set via --navidrome-url or NAVIDROME_URL")
	}
	if opts.navidromeUsername == "" || opts.navidromePassword == "" {
		return errors.New("navidrome username and password must be set via flags or environment")
	}

	client, err := opts.newNavidromeClient(navidrome.Config{
		BaseURL:  opts.navidromeURL,
		Username: opts.navidromeUsername,
		Password: opts.navidromePassword,
	})
	if err != nil {
		return fmt.Errorf("init navidrome client: %w", err)
	}

	var store app.TrackStore
	if opts.dbPath != "" {
		if err := ensureDir(opts.dbPath); err != nil {
			return err
		}

		s, err := opts.newStore(sqlite.Config{Path: opts.dbPath})
		if err != nil {
			return fmt.Errorf("init store: %w", err)
		}
		store = s
		if closer, ok := s.(interface{ Close() error }); ok {
			defer closer.Close()
		}
	}

	appInstance, err := opts.newApp(app.Dependencies{
		Navidrome: client,
		Store:     store,
	})
	if err != nil {
		return fmt.Errorf("init app: %w", err)
	}

	count, err := appInstance.SyncTracks(ctx)
	if err != nil {
		return fmt.Errorf("sync tracks: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Fetched %d tracks\n", count)
	return nil
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create db directory: %w", err)
	}
	return nil
}
