package cli

import "testing"

func TestNewRootCmd(t *testing.T) {
	opts := newOptions()
	cmd := newRootCmd(opts)
	if cmd.Use != "playlistgen" {
		t.Fatalf("unexpected use %s", cmd.Use)
	}
	if !cmd.HasSubCommands() {
		t.Fatalf("expected subcommands to be registered")
	}
}
