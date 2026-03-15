package audio

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestFFmpegProbeRunnerIncludesCommandOutputOnDurationFailure(t *testing.T) {
	runner := FFmpegProbeRunner{
		Runner: commandRunnerStub{
			run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				return []byte("/library/albums/song.flac: No such file or directory"), errors.New("exit status 1")
			},
		},
	}

	_, err := runner.Measure(context.Background(), "/library/albums/song.flac")
	if err == nil {
		t.Fatal("expected error")
	}
	got := err.Error()
	if !strings.Contains(got, "ffprobe duration") || !strings.Contains(got, "/library/albums/song.flac: No such file or directory") {
		t.Fatalf("unexpected error %q", got)
	}
}

type commandRunnerStub struct {
	run func(context.Context, string, ...string) ([]byte, error)
}

func (c commandRunnerStub) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return c.run(ctx, name, args...)
}
