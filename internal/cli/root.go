package cli

import (
	"io"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/bowmanmike/playlistgen/internal/app"
	"github.com/bowmanmike/playlistgen/internal/logging"
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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.ensureLogger(cmd.ErrOrStderr())
		},
	}

	cmd.PersistentFlags().StringVar(&opts.navidromeURL, "navidrome-url", "", "Navidrome base URL (or NAVIDROME_URL)")
	cmd.PersistentFlags().StringVar(&opts.navidromeUsername, "navidrome-username", "", "Navidrome username (or NAVIDROME_USERNAME/NAVIDROME_USER)")
	cmd.PersistentFlags().StringVar(&opts.navidromePassword, "navidrome-password", "", "Navidrome password (or NAVIDROME_PASSWORD)")
	cmd.PersistentFlags().StringVar(&opts.dbPath, "db-path", getEnv("PLAYLISTGEN_DB_PATH", defaultDBPath), "SQLite database path (or PLAYLISTGEN_DB_PATH)")
	cmd.PersistentFlags().StringVar(&opts.logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	cmd.PersistentFlags().StringVar(&opts.logFormat, "log-format", "json", "Log format (json, text)")

	cmd.AddCommand(newSyncCmd(opts))
	cmd.AddCommand(newAudioProcessCmd(opts))

	return cmd
}

type options struct {
	navidromeURL       string
	navidromeUsername  string
	navidromePassword  string
	dbPath             string
	logLevel           string
	logFormat          string
	logger             *slog.Logger
	forceProcessing    bool
	newNavidromeClient func(navidrome.Config) (app.NavidromePort, error)
	newStore           func(sqlite.Config) (app.TrackStore, error)
	newApp             func(app.Dependencies) (*app.App, error)
}

func newOptions() *options {
	return &options{
		logLevel:  "info",
		logFormat: "json",
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

func (o *options) ensureLogger(dst io.Writer) error {
	if o.logger != nil {
		return nil
	}
	logger, err := logging.New(o.logLevel, o.logFormat, dst)
	if err != nil {
		return err
	}
	o.logger = logger
	slog.SetDefault(logger)
	return nil
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}
