package main

import (
	"log/slog"
	"os"

	"github.com/bowmanmike/playlistgen/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		slog.Error("command failed", "error", err)
		os.Exit(1)
	}
}
