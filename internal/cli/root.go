package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/bowmanmike/playlistgen/internal/app"
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

type options struct {
	navidromeURL       string
	navidromeAPIKey    string
	newNavidromeClient func(navidrome.Config) (app.NavidromePort, error)
	newApp             func(app.Dependencies) (*app.App, error)
}

func newOptions() *options {
	return &options{
		newNavidromeClient: func(cfg navidrome.Config) (app.NavidromePort, error) {
			return navidrome.NewClient(cfg)
		},
		newApp: func(deps app.Dependencies) (*app.App, error) {
			return app.New(deps)
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
