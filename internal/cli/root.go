package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/bowmanmike/playlistgen/internal/app"
	"github.com/bowmanmike/playlistgen/internal/navidrome"
	"github.com/bowmanmike/playlistgen/internal/storage/sqlite"
)

const defaultDBPath = "data/db.sqlite"

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

	cmd.PersistentFlags().StringVar(&opts.navidromeURL, "navidrome-url", "", "Navidrome base URL (or NAVIDROME_URL)")
	cmd.PersistentFlags().StringVar(&opts.navidromeUsername, "navidrome-username", "", "Navidrome username (or NAVIDROME_USERNAME/NAVIDROME_USER)")
	cmd.PersistentFlags().StringVar(&opts.navidromePassword, "navidrome-password", "", "Navidrome password (or NAVIDROME_PASSWORD)")
	cmd.PersistentFlags().StringVar(&opts.dbPath, "db-path", getEnv("PLAYLISTGEN_DB_PATH", defaultDBPath), "SQLite database path (or PLAYLISTGEN_DB_PATH)")

	cmd.AddCommand(newSyncCmd(opts))

	return cmd
}

type options struct {
	navidromeURL       string
	navidromeUsername  string
	navidromePassword  string
	dbPath             string
	newNavidromeClient func(navidrome.Config) (app.NavidromePort, error)
	newStore           func(sqlite.Config) (app.TrackStore, error)
	newApp             func(app.Dependencies) (*app.App, error)
}

func newOptions() *options {
	return &options{
		newNavidromeClient: func(cfg navidrome.Config) (app.NavidromePort, error) {
			return navidrome.NewClient(cfg)
		},
		newStore: func(cfg sqlite.Config) (app.TrackStore, error) {
			return sqlite.New(cfg)
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

func (o *options) populateFromEnv() {
	if o.navidromeURL == "" {
		o.navidromeURL = os.Getenv("NAVIDROME_URL")
	}
	if o.navidromeUsername == "" {
		o.navidromeUsername = firstEnv("NAVIDROME_USERNAME", "NAVIDROME_USER")
	}
	if o.navidromePassword == "" {
		o.navidromePassword = os.Getenv("NAVIDROME_PASSWORD")
	}
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}
