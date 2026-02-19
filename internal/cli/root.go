package cli

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"github.com/bowmanmike/playlistgen/internal/navidrome"
)

// Execute runs the root CLI command.
func Execute() error {
	opts := newOptions()
	rootCmd := newRootCmd(opts)
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	return rootCmd.Execute()
}

func newRootCmd(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "playlistgen",
		Short: "AI-assisted playlist generator",
	}

	cmd.PersistentFlags().StringVar(&opts.navidromeURL, "navidrome-url", getEnv("NAVIDROME_URL", ""), "Navidrome base URL (or NAVIDROME_URL)")
	cmd.PersistentFlags().StringVar(&opts.navidromeAPIKey, "navidrome-api-key", getEnv("NAVIDROME_API_KEY", ""), "Navidrome API key (or NAVIDROME_API_KEY)")

	cmd.AddCommand(newSyncCmd(opts))

	return cmd
}

type navidromeAPI interface {
	ListTracks(ctx context.Context) ([]navidrome.Track, error)
}

type options struct {
	navidromeURL       string
	navidromeAPIKey    string
	newNavidromeClient func(navidrome.Config) (navidromeAPI, error)
}

func newOptions() *options {
	return &options{
		newNavidromeClient: func(cfg navidrome.Config) (navidromeAPI, error) {
			return navidrome.NewClient(cfg)
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
