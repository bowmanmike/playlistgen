package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
)

type FFmpegProbeRunner struct {
	Runner CommandRunner
}

func (r FFmpegProbeRunner) Measure(ctx context.Context, path string) (MeasuredAudio, error) {
	runner := r.Runner
	if runner == nil {
		runner = ExecRunner{}
	}

	durationOut, err := runner.Run(ctx, "ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_entries", "format=duration",
		path,
	)
	if err != nil {
		return MeasuredAudio{}, fmt.Errorf("ffprobe duration: %w", err)
	}
	var durationPayload struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(durationOut, &durationPayload); err != nil {
		return MeasuredAudio{}, fmt.Errorf("decode duration payload: %w", err)
	}
	durationSeconds, err := strconv.ParseFloat(durationPayload.Format.Duration, 64)
	if err != nil {
		return MeasuredAudio{}, fmt.Errorf("parse duration: %w", err)
	}

	loudnessOut, err := runner.Run(ctx, "ffmpeg",
		"-i", path,
		"-af", "loudnorm=I=-16:TP=-1.5:LRA=11:print_format=json",
		"-f", "null",
		"-",
	)
	if err != nil {
		return MeasuredAudio{}, fmt.Errorf("ffmpeg loudness scan: %w", err)
	}

	measuredLoudness, measuredPeak, err := parseLoudnormOutput(loudnessOut)
	if err != nil {
		return MeasuredAudio{}, err
	}

	return MeasuredAudio{
		FileDurationSeconds: durationSeconds,
		IntegratedLUFS:      measuredLoudness,
		TruePeak:            measuredPeak,
	}, nil
}

var loudnormJSON = regexp.MustCompile(`\{[\s\S]*"input_i"[\s\S]*\}`)

func parseLoudnormOutput(out []byte) (*float64, *float64, error) {
	match := loudnormJSON.Find(out)
	if len(match) == 0 {
		return nil, nil, fmt.Errorf("ffmpeg loudnorm output missing json")
	}
	var payload struct {
		InputI  string `json:"input_i"`
		InputTP string `json:"input_tp"`
	}
	if err := json.Unmarshal(match, &payload); err != nil {
		return nil, nil, fmt.Errorf("decode loudnorm payload: %w", err)
	}
	lufs, err := strconv.ParseFloat(payload.InputI, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("parse integrated loudness: %w", err)
	}
	peak, err := strconv.ParseFloat(payload.InputTP, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("parse true peak: %w", err)
	}
	return &lufs, &peak, nil
}
