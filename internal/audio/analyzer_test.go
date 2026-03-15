package audio

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestResolveLibraryPathJoinsConfiguredRoot(t *testing.T) {
	got, err := ResolveLibraryPath("/library", "/albums/song.flac")
	if err != nil {
		t.Fatalf("resolve path: %v", err)
	}
	if got != "/library/albums/song.flac" {
		t.Fatalf("unexpected resolved path %q", got)
	}
}

func TestEffectiveValuesPreferReplayGainOverMeasuredValues(t *testing.T) {
	albumGain := -6.5
	albumPeak := 0.81
	trackGain := -7.1
	trackPeak := 0.78
	lufs := -11.2
	peak := 0.95

	got := EffectiveValues(RawReplayGain{
		TrackGainDB: &trackGain,
		TrackPeak:   &trackPeak,
		AlbumGainDB: &albumGain,
		AlbumPeak:   &albumPeak,
	}, MeasuredAudio{
		IntegratedLUFS: &lufs,
		TruePeak:       &peak,
	})

	if got.GainDB == nil || *got.GainDB != albumGain || got.GainSource != "replaygain_album" {
		t.Fatalf("unexpected effective gain %+v", got)
	}
	if got.Peak == nil || *got.Peak != albumPeak || got.PeakSource != "replaygain_album" {
		t.Fatalf("unexpected effective peak %+v", got)
	}
}

func TestEffectiveValuesFallBackToMeasuredValues(t *testing.T) {
	lufs := -10.4
	peak := 0.97
	got := EffectiveValues(RawReplayGain{}, MeasuredAudio{
		IntegratedLUFS: &lufs,
		TruePeak:       &peak,
	})
	if got.GainDB == nil || *got.GainDB != lufs || got.GainSource != "measured_integrated_lufs" {
		t.Fatalf("unexpected effective gain %+v", got)
	}
	if got.Peak == nil || *got.Peak != peak || got.PeakSource != "measured_true_peak" {
		t.Fatalf("unexpected effective peak %+v", got)
	}
}

func TestAnalyzerUsesMeasuredAndTagDataToBuildRecord(t *testing.T) {
	lufs := -11.4
	peak := 0.91
	albumGain := -6.0
	albumPeak := 0.82
	now := time.Unix(100, 0).UTC()
	analyzer := Analyzer{
		Root: "/library",
		Probe: probeStub{measured: MeasuredAudio{
			FileDurationSeconds: 123.4,
			IntegratedLUFS:      &lufs,
			TruePeak:            &peak,
		}},
		Tags: replayGainStub{raw: RawReplayGain{
			AlbumGainDB: &albumGain,
			AlbumPeak:   &albumPeak,
		}},
		Now: func() time.Time { return now },
	}

	got, err := analyzer.Analyze(context.Background(), "/albums/song.flac")
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if got.FilePath != "/library/albums/song.flac" {
		t.Fatalf("unexpected file path %q", got.FilePath)
	}
	if got.AnalyzedAt != now {
		t.Fatalf("unexpected analyzed time %v", got.AnalyzedAt)
	}
	if got.Effective.GainSource != "replaygain_album" || got.Effective.PeakSource != "replaygain_album" {
		t.Fatalf("unexpected effective values %+v", got.Effective)
	}
}

func TestAnalyzerHandlesMissingReplayGainTags(t *testing.T) {
	lufs := -9.3
	peak := 0.98
	analyzer := Analyzer{
		Root: "/library",
		Probe: probeStub{measured: MeasuredAudio{
			FileDurationSeconds: 98.7,
			IntegratedLUFS:      &lufs,
			TruePeak:            &peak,
		}},
		Tags: replayGainStub{err: errors.New("no tags")},
	}

	got, err := analyzer.Analyze(context.Background(), "/albums/song.flac")
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if got.Effective.GainSource != "measured_integrated_lufs" || got.Effective.PeakSource != "measured_true_peak" {
		t.Fatalf("unexpected fallback values %+v", got.Effective)
	}
}

func TestAnalyzerIncludesResolvedPathAndProbeOutputInError(t *testing.T) {
	analyzer := Analyzer{
		Root: "/library",
		Probe: probeStub{
			err: errors.New("ffprobe duration: /library/albums/song.flac: No such file or directory"),
		},
		Tags: replayGainStub{},
	}

	_, err := analyzer.Analyze(context.Background(), "/albums/song.flac")
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got == "" ||
		!containsAll(got,
			"analyze /library/albums/song.flac",
			"ffprobe duration",
			"No such file or directory",
		) {
		t.Fatalf("unexpected error %q", got)
	}
}

type probeStub struct {
	measured MeasuredAudio
	err      error
}

func (p probeStub) Measure(context.Context, string) (MeasuredAudio, error) {
	return p.measured, p.err
}

type replayGainStub struct {
	raw RawReplayGain
	err error
}

func (r replayGainStub) Read(context.Context, string) (RawReplayGain, error) {
	return r.raw, r.err
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
