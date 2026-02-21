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

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Navidrome URL: %s\n", opts.navidromeURL)
	fmt.Fprintf(out, "Navidrome user: %s\n", opts.navidromeUsername)

	client, err := opts.newNavidromeClient(navidrome.Config{
		BaseURL:  opts.navidromeURL,
		Username: opts.navidromeUsername,
		Password: opts.navidromePassword,
	})
	if err != nil {
		return fmt.Errorf("init navidrome client: %w", err)
	}

	var (
		store             app.TrackStore
		persistenceOn     bool
		resolvedStorePath string
	)
	if opts.dbPath != "" {
		absPath, err := filepath.Abs(opts.dbPath)
		if err != nil {
			return fmt.Errorf("resolve db path: %w", err)
		}
		resolvedStorePath = absPath

		if err := ensureDir(resolvedStorePath); err != nil {
			return err
		}

		s, err := opts.newStore(sqlite.Config{Path: resolvedStorePath})
		if err != nil {
			return fmt.Errorf("init store: %w", err)
		}
		store = s
		persistenceOn = true
		if closer, ok := s.(interface{ Close() error }); ok {
			defer closer.Close()
		}
	}
	if persistenceOn {
		fmt.Fprintf(out, "Track store path: %s\n", resolvedStorePath)
	} else {
		fmt.Fprintln(out, "Track store disabled (no db-path provided)")
	}

	appInstance, err := opts.newApp(app.Dependencies{
		Navidrome: client,
		Store:     store,
	})
	if err != nil {
		return fmt.Errorf("init app: %w", err)
	}

	stats, err := appInstance.SyncTracks(ctx)
	if err != nil {
		return fmt.Errorf("sync tracks: %w", err)
	}

	if persistenceOn {
		fmt.Fprintf(out, "Updated %d tracks (skipped %d, deleted %d)\n", stats.Updated, stats.Skipped, stats.Deleted)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Fetched %d tracks\n", stats.Fetched)
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
