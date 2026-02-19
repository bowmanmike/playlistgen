package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bowmanmike/playlistgen/internal/navidrome"
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
	if opts.navidromeURL == "" {
		return errors.New("navidrome URL must be set via --navidrome-url or NAVIDROME_URL")
	}

	client, err := opts.newNavidromeClient(navidrome.Config{
		BaseURL: opts.navidromeURL,
		APIKey:  opts.navidromeAPIKey,
	})
	if err != nil {
		return fmt.Errorf("init navidrome client: %w", err)
	}

	tracks, err := client.ListTracks(ctx)
	if err != nil {
		return fmt.Errorf("list tracks: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Fetched %d tracks\n", len(tracks))
	return nil
}
