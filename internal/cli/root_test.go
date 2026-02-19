package cli

import "testing"

func TestNewRootCmd(t *testing.T) {
	cmd := newRootCmd()
	if cmd.Use != "playlistgen" {
		t.Fatalf("unexpected use %s", cmd.Use)
	}
}
