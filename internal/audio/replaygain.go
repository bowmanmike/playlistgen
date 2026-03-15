package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type CommandRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

type FFProbeReplayGainReader struct {
	Runner CommandRunner
}

func (r FFProbeReplayGainReader) Read(ctx context.Context, path string) (RawReplayGain, error) {
	runner := r.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	out, err := runner.Run(ctx, "ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_entries", "format_tags",
		path,
	)
	if err != nil {
		return RawReplayGain{}, fmt.Errorf("ffprobe replaygain tags: %w", err)
	}
	var payload struct {
		Format struct {
			Tags map[string]string `json:"tags"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return RawReplayGain{}, fmt.Errorf("decode replaygain tags: %w", err)
	}
	return RawReplayGain{
		TrackGainDB: parseReplayGainValue(payload.Format.Tags["REPLAYGAIN_TRACK_GAIN"]),
		TrackPeak:   parseReplayGainValue(payload.Format.Tags["REPLAYGAIN_TRACK_PEAK"]),
		AlbumGainDB: parseReplayGainValue(payload.Format.Tags["REPLAYGAIN_ALBUM_GAIN"]),
		AlbumPeak:   parseReplayGainValue(payload.Format.Tags["REPLAYGAIN_ALBUM_PEAK"]),
	}, nil
}

func parseReplayGainValue(raw string) *float64 {
	raw = strings.TrimSpace(strings.TrimSuffix(strings.ToLower(raw), " db"))
	if raw == "" {
		return nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil
	}
	return &value
}
