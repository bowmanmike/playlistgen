package main

import "testing"

func TestAppMessage(t *testing.T) {
	if msg := appMessage(); msg != "main" {
		t.Fatalf("expected %q got %q", "main", msg)
	}
}
