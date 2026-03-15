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
				ID:    1,
				Track: testAudioTrack("track-1"),
			}},
			nil,
		},
	}
	opts := &options{
		dbPath: filepath.Join(t.TempDir(), "jobs.db"),
		newAudioStore: func(cfg sqlite.Config) (audioJobStore, error) {
			return store, nil
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
}

func TestRunAudioProcessExitsWhenNoJobsCanBeClaimed(t *testing.T) {
	cmd := &cobra.Command{}
	store := &audioJobStoreStub{}
	opts := &options{
		dbPath: filepath.Join(t.TempDir(), "jobs.db"),
		newAudioStore: func(cfg sqlite.Config) (audioJobStore, error) {
			return store, nil
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
	jobs := []sqlite.AudioJob{
		{ID: 1, Track: testAudioTrack("track-1")},
		{ID: 2, Track: testAudioTrack("track-2")},
		{ID: 3, Track: testAudioTrack("track-3")},
	}

	done := make(chan error, 1)
	go func() {
		done <- processAudioBatch(ctx, store, jobs, 1, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("processAudioBatch did not return after cancellation")
	}
}

type audioJobStoreStub struct {
	mu               sync.Mutex
	claimBatches     [][]sqlite.AudioJob
	claimCalls       int
	lastClaimOptions sqlite.ClaimOptions
	completedJobIDs  []int64
	failedJobIDs     []int64
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

func (s *audioJobStoreStub) FailAudioJob(ctx context.Context, jobID int64, err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failedJobIDs = append(s.failedJobIDs, jobID)
	return nil
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
