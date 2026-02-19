package cli

import "github.com/spf13/cobra"

// Execute runs the root CLI command.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "playlistgen",
		Short: "AI-assisted playlist generator",
	}

	return cmd
}
