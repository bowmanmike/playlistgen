package cli

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/bowmanmike/playlistgen/internal/app"
	"github.com/bowmanmike/playlistgen/internal/audio"
	"github.com/bowmanmike/playlistgen/internal/storage/sqlite"
)

func TestRunAudioProcessClaimsJobsFromStore(t *testing.T) {
	out := &bytes.Buffer{}
	cmd := &cobra.Command{}
	cmd.SetOut(out)
	cmd.SetErr(out)

	store := &audioJobStoreStub{
		claimBatches: [][]sqlite.AudioJob{
			{{
				ID:      1,
				TrackID: 101,
				Track:   testAudioTrack("track-1"),
			}},
			nil,
		},
	}
	analyzer := &audioAnalyzerStub{result: audio.AnalysisResult{
		AnalyzedAt: time.Unix(100, 0).UTC(),
		Measured: audio.MeasuredAudio{
			FileDurationSeconds: 120,
		},
		Effective: audio.EffectiveAudio{
			GainSource: "none",
			PeakSource: "none",
		},
	}}
	opts := &options{
		dbPath: filepath.Join(t.TempDir(), "jobs.db"),
		newAudioStore: func(cfg sqlite.Config) (audioJobStore, error) {
			return store, nil
		},
		newAudioAnalyzer: func(root string) audioAnalyzer {
			analyzer.root = root
			return analyzer
		},
		logFormat: "text",
	}

	if err := runAudioProcess(context.Background(), cmd, opts, audioProcessConfig{
		batchSize:   1,
		workerCount: 1,
		processAll:  true,
	}); err != nil {
		t.Fatalf("runAudioProcess: %v", err)
	}

	if store.claimCalls != 2 {
		t.Fatalf("expected 2 claim calls, got %d", store.claimCalls)
	}
	if len(store.completedJobIDs) != 1 || store.completedJobIDs[0] != 1 {
		t.Fatalf("unexpected completed jobs %+v", store.completedJobIDs)
	}
	if store.lastClaimOptions.Limit != 1 {
		t.Fatalf("unexpected claim limit %d", store.lastClaimOptions.Limit)
	}
	if store.lastClaimOptions.ClaimedBy == "" {
		t.Fatalf("expected claimed_by to be set")
	}
	if len(store.featureRecords) != 1 || store.featureRecords[0].TrackID != 101 {
		t.Fatalf("unexpected feature records %+v", store.featureRecords)
	}
	if len(store.runSummaries) != 1 || store.runSummaries[0].Status != "completed" {
		t.Fatalf("unexpected run summaries %+v", store.runSummaries)
	}
	if analyzer.root != defaultLibraryRoot {
		t.Fatalf("unexpected analyzer root %q", analyzer.root)
	}
}

func TestRunAudioProcessExitsWhenNoJobsCanBeClaimed(t *testing.T) {
	cmd := &cobra.Command{}
	store := &audioJobStoreStub{}
	opts := &options{
		dbPath: filepath.Join(t.TempDir(), "jobs.db"),
		newAudioStore: func(cfg sqlite.Config) (audioJobStore, error) {
			return store, nil
		},
		newAudioAnalyzer: func(root string) audioAnalyzer {
			return &audioAnalyzerStub{}
		},
		logFormat: "text",
	}

	if err := runAudioProcess(context.Background(), cmd, opts, audioProcessConfig{
		batchSize:   5,
		workerCount: 1,
		processAll:  true,
	}); err != nil {
		t.Fatalf("runAudioProcess: %v", err)
	}

	if store.claimCalls != 1 {
		t.Fatalf("expected 1 claim call, got %d", store.claimCalls)
	}
}

func TestProcessAudioBatchReturnsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := &audioJobStoreStub{}
	analyzer := &audioAnalyzerStub{
		err: context.Canceled,
	}
	jobs := []sqlite.AudioJob{
		{ID: 1, TrackID: 101, Track: testAudioTrack("track-1")},
		{ID: 2, TrackID: 102, Track: testAudioTrack("track-2")},
		{ID: 3, TrackID: 103, Track: testAudioTrack("track-3")},
	}

	done := make(chan error, 1)
	go func() {
		_, err := processAudioBatch(ctx, store, analyzer, jobs, 1, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
		done <- err
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("expected nil or context canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("processAudioBatch did not return after cancellation")
	}
}

func TestRunAudioProcessUsesConfiguredLibraryRoot(t *testing.T) {
	cmd := &cobra.Command{}
	store := &audioJobStoreStub{
		claimBatches: [][]sqlite.AudioJob{{{
			ID:      1,
			TrackID: 101,
			Track:   testAudioTrack("track-1"),
		}}},
	}
	analyzer := &audioAnalyzerStub{result: audio.AnalysisResult{
		AnalyzedAt: time.Now().UTC(),
		Measured: audio.MeasuredAudio{
			FileDurationSeconds: 60,
		},
		Effective: audio.EffectiveAudio{
			GainSource: "none",
			PeakSource: "none",
		},
	}}
	opts := &options{
		dbPath:      filepath.Join(t.TempDir(), "jobs.db"),
		libraryRoot: "/mounted/music",
		newAudioStore: func(cfg sqlite.Config) (audioJobStore, error) {
			return store, nil
		},
		newAudioAnalyzer: func(root string) audioAnalyzer {
			analyzer.root = root
			return analyzer
		},
		logFormat: "text",
	}
	if err := runAudioProcess(context.Background(), cmd, opts, audioProcessConfig{batchSize: 1, workerCount: 1}); err != nil {
		t.Fatalf("runAudioProcess: %v", err)
	}
	if analyzer.root != "/mounted/music" {
		t.Fatalf("unexpected analyzer root %q", analyzer.root)
	}
}

func TestRunAudioProcessMarksJobFailedOnAnalyzerError(t *testing.T) {
	cmd := &cobra.Command{}
	store := &audioJobStoreStub{
		claimBatches: [][]sqlite.AudioJob{{{
			ID:      1,
			TrackID: 101,
			Track:   testAudioTrack("track-1"),
		}}},
	}
	opts := &options{
		dbPath: filepath.Join(t.TempDir(), "jobs.db"),
		newAudioStore: func(cfg sqlite.Config) (audioJobStore, error) {
			return store, nil
		},
		newAudioAnalyzer: func(root string) audioAnalyzer {
			return &audioAnalyzerStub{err: errors.New("analyzer boom"), root: root}
		},
		logFormat: "text",
	}
	if err := runAudioProcess(context.Background(), cmd, opts, audioProcessConfig{batchSize: 1, workerCount: 1}); err == nil {
		t.Fatal("expected error")
	}
	if len(store.failedJobIDs) != 1 || store.failedJobIDs[0] != 1 {
		t.Fatalf("unexpected failed jobs %+v", store.failedJobIDs)
	}
	if len(store.runSummaries) != 1 || store.runSummaries[0].Status != "failed" {
		t.Fatalf("unexpected run summaries %+v", store.runSummaries)
	}
}

type audioJobStoreStub struct {
	mu               sync.Mutex
	claimBatches     [][]sqlite.AudioJob
	claimCalls       int
	lastClaimOptions sqlite.ClaimOptions
	completedJobIDs  []int64
	failedJobIDs     []int64
	featureRecords   []sqlite.AudioFeatureRecord
	runIDs           []int64
	runSummaries     []sqlite.AudioProcessingRunSummary
}

func (s *audioJobStoreStub) ClaimPendingAudioJobs(ctx context.Context, opts sqlite.ClaimOptions) ([]sqlite.AudioJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimCalls++
	s.lastClaimOptions = opts
	if len(s.claimBatches) == 0 {
		return nil, nil
	}
	batch := s.claimBatches[0]
	s.claimBatches = s.claimBatches[1:]
	return batch, nil
}

func (s *audioJobStoreStub) CompleteAudioJob(ctx context.Context, jobID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completedJobIDs = append(s.completedJobIDs, jobID)
	return nil
}

func (s *audioJobStoreStub) StartAudioProcessingRun(ctx context.Context, startedAt time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	runID := int64(len(s.runIDs) + 1)
	s.runIDs = append(s.runIDs, runID)
	return runID, nil
}

func (s *audioJobStoreStub) CompleteAudioProcessingRun(ctx context.Context, runID int64, summary sqlite.AudioProcessingRunSummary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runSummaries = append(s.runSummaries, summary)
	return nil
}

func (s *audioJobStoreStub) UpsertTrackAudioFeatures(ctx context.Context, record sqlite.AudioFeatureRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.featureRecords = append(s.featureRecords, record)
	return nil
}

func (s *audioJobStoreStub) FailAudioJob(ctx context.Context, jobID int64, err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failedJobIDs = append(s.failedJobIDs, jobID)
	return nil
}

type audioAnalyzerStub struct {
	root   string
	result audio.AnalysisResult
	err    error
}

func (a *audioAnalyzerStub) Analyze(ctx context.Context, navPath string) (audio.AnalysisResult, error) {
	if a.err != nil {
		return audio.AnalysisResult{}, a.err
	}
	return a.result, nil
}

func (s *audioJobStoreStub) Close() error {
	return nil
}

func testAudioTrack(id string) app.Track {
	return app.Track{
		ID:       id,
		Title:    id,
		Artist:   "Artist",
		Album:    "Album",
		Path:     "/music/" + id + ".flac",
		Suffix:   "flac",
		Duration: time.Second,
	}
}
