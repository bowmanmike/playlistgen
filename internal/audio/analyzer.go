package audio

import (
	"context"
	"fmt"
	"time"
)

type MeasuredAudio struct {
	FileDurationSeconds float64
	IntegratedLUFS      *float64
	TruePeak            *float64
}

type RawReplayGain struct {
	TrackGainDB *float64
	TrackPeak   *float64
	AlbumGainDB *float64
	AlbumPeak   *float64
}

type EffectiveAudio struct {
	GainDB     *float64
	Peak       *float64
	GainSource string
	PeakSource string
}

type AnalysisResult struct {
	AnalyzedAt time.Time
	FilePath   string
	Measured   MeasuredAudio
	ReplayGain RawReplayGain
	Effective  EffectiveAudio
}

type ProbeRunner interface {
	Measure(context.Context, string) (MeasuredAudio, error)
}

type ReplayGainReader interface {
	Read(context.Context, string) (RawReplayGain, error)
}

type Analyzer struct {
	Root  string
	Probe ProbeRunner
	Tags  ReplayGainReader
	Now   func() time.Time
}

func (a Analyzer) Analyze(ctx context.Context, navPath string) (AnalysisResult, error) {
	filePath, err := ResolveLibraryPath(a.Root, navPath)
	if err != nil {
		return AnalysisResult{}, err
	}
	if a.Probe == nil {
		return AnalysisResult{}, fmt.Errorf("probe runner is required")
	}
	if a.Tags == nil {
		return AnalysisResult{}, fmt.Errorf("replaygain reader is required")
	}

	measured, err := a.Probe.Measure(ctx, filePath)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("analyze %s: %w", filePath, err)
	}
	rawTags, err := a.Tags.Read(ctx, filePath)
	if err != nil {
		return AnalysisResult{
			AnalyzedAt: timestamp(a.Now),
			FilePath:   filePath,
			Measured:   measured,
			ReplayGain: RawReplayGain{},
			Effective:  EffectiveValues(RawReplayGain{}, measured),
		}, nil
	}

	return AnalysisResult{
		AnalyzedAt: timestamp(a.Now),
		FilePath:   filePath,
		Measured:   measured,
		ReplayGain: rawTags,
		Effective:  EffectiveValues(rawTags, measured),
	}, nil
}

func EffectiveValues(raw RawReplayGain, measured MeasuredAudio) EffectiveAudio {
	gain, gainSource := firstValue([]valueSource{
		{raw.AlbumGainDB, "replaygain_album"},
		{raw.TrackGainDB, "replaygain_track"},
		{measured.IntegratedLUFS, "measured_integrated_lufs"},
	})
	peak, peakSource := firstValue([]valueSource{
		{raw.AlbumPeak, "replaygain_album"},
		{raw.TrackPeak, "replaygain_track"},
		{measured.TruePeak, "measured_true_peak"},
	})
	return EffectiveAudio{
		GainDB:     gain,
		Peak:       peak,
		GainSource: gainSource,
		PeakSource: peakSource,
	}
}

type valueSource struct {
	value  *float64
	source string
}

func firstValue(values []valueSource) (*float64, string) {
	for _, item := range values {
		if item.value != nil {
			v := *item.value
			return &v, item.source
		}
	}
	return nil, "none"
}

func timestamp(nowFn func() time.Time) time.Time {
	if nowFn == nil {
		return time.Now().UTC()
	}
	return nowFn().UTC()
}
