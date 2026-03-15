package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/bowmanmike/playlistgen/internal/audio"
	"github.com/bowmanmike/playlistgen/internal/storage/sqlite"
)

const audioClaimStaleAfter = 5 * time.Minute

type audioProcessConfig struct {
	batchSize   int
	workerCount int
	processAll  bool
}

type audioJobStore interface {
	ClaimPendingAudioJobs(context.Context, sqlite.ClaimOptions) ([]sqlite.AudioJob, error)
	StartAudioProcessingRun(context.Context, time.Time) (int64, error)
	CompleteAudioProcessingRun(context.Context, int64, sqlite.AudioProcessingRunSummary) error
	UpsertTrackAudioFeatures(context.Context, sqlite.AudioFeatureRecord) error
	CompleteAudioJob(context.Context, int64) error
	FailAudioJob(context.Context, int64, error) error
	Close() error
}

type audioAnalyzer interface {
	Analyze(context.Context, string) (audio.AnalysisResult, error)
}

func newAudioProcessCmd(opts *options) *cobra.Command {
	cfg := audioProcessConfig{
		batchSize:   50,
		workerCount: 4,
	}

	cmd := &cobra.Command{
		Use:   "audio-process",
		Short: "Process pending audio analysis jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAudioProcess(cmd.Context(), cmd, opts, cfg)
		},
	}

	cmd.Flags().IntVar(&cfg.batchSize, "batch-size", cfg.batchSize, "Number of audio jobs to fetch per batch")
	cmd.Flags().IntVar(&cfg.workerCount, "workers", cfg.workerCount, "Number of concurrent audio workers")
	cmd.Flags().BoolVar(&cfg.processAll, "all", false, "Process audio jobs until the queue is empty")

	return cmd
}

func runAudioProcess(ctx context.Context, cmd *cobra.Command, opts *options, cfg audioProcessConfig) error {
	if err := opts.ensureLogger(cmd.ErrOrStderr()); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	logger := opts.logger

	if opts.dbPath == "" {
		return errors.New("db-path must be set to process audio jobs")
	}
	if cfg.batchSize <= 0 {
		return errors.New("batch-size must be greater than zero")
	}
	if cfg.workerCount <= 0 {
		return errors.New("workers must be greater than zero")
	}
	if opts.libraryRoot == "" {
		opts.libraryRoot = defaultLibraryRoot
	}

	dbPath, err := filepath.Abs(opts.dbPath)
	if err != nil {
		return fmt.Errorf("resolve db path: %w", err)
	}

	store, err := opts.newAudioStore(sqlite.Config{Path: dbPath})
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	runStartedAt := time.Now().UTC()
	runID, err := store.StartAudioProcessingRun(ctx, runStartedAt)
	if err != nil {
		return fmt.Errorf("start audio processing run: %w", err)
	}

	analyzer := opts.newAudioAnalyzer(opts.libraryRoot)
	claimedBy := fmt.Sprintf("audio-process-%d", os.Getpid())
	summary := sqlite.AudioProcessingRunSummary{Status: "completed"}
	totalProcessed := 0
	for {
		select {
		case <-ctx.Done():
			summary.Status = "canceled"
			summary.CompletedAt = time.Now().UTC()
			_ = store.CompleteAudioProcessingRun(ctx, runID, summary)
			return ctx.Err()
		default:
		}

		jobs, err := store.ClaimPendingAudioJobs(ctx, sqlite.ClaimOptions{
			Limit:      cfg.batchSize,
			ClaimedBy:  claimedBy,
			StaleAfter: audioClaimStaleAfter,
			Now:        time.Now().UTC(),
		})
		if err != nil {
			summary.Status = "failed"
			summary.CompletedAt = time.Now().UTC()
			_ = store.CompleteAudioProcessingRun(ctx, runID, summary)
			return fmt.Errorf("claim audio jobs: %w", err)
		}
		if len(jobs) == 0 {
			if totalProcessed == 0 {
				logger.Info("no pending audio jobs found")
			}
			break
		}
		summary.JobsClaimed += len(jobs)

		logger.Info("processing audio batch",
			"jobs", len(jobs),
			"workers", cfg.workerCount,
		)
		batchSummary, err := processAudioBatch(ctx, store, analyzer, jobs, cfg.workerCount, logger)
		summary.JobsCompleted += batchSummary.completed
		summary.JobsFailed += batchSummary.failed
		if err != nil {
			summary.Status = "failed"
			summary.CompletedAt = time.Now().UTC()
			_ = store.CompleteAudioProcessingRun(ctx, runID, summary)
			return err
		}
		totalProcessed += len(jobs)

		if !cfg.processAll {
			break
		}
	}

	summary.CompletedAt = time.Now().UTC()
	if err := store.CompleteAudioProcessingRun(ctx, runID, summary); err != nil {
		return fmt.Errorf("complete audio processing run: %w", err)
	}
	logger.Info("audio processing complete", "processed_jobs", totalProcessed)
	return nil
}

type audioBatchSummary struct {
	completed int
	failed    int
}

func processAudioBatch(ctx context.Context, store audioJobStore, analyzer audioAnalyzer, jobs []sqlite.AudioJob, workers int, logger *slog.Logger) (audioBatchSummary, error) {
	jobCh := make(chan sqlite.AudioJob)
	errCh := make(chan error, len(jobs)+workers)
	resultCh := make(chan audioBatchSummary, len(jobs))

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			workerLogger := logger.With("worker", workerID)
			for job := range jobCh {
				select {
				case <-ctx.Done():
					errCh <- ctx.Err()
					return
				default:
				}

				workerLogger.Info("processing audio job",
					"job_id", job.ID,
					"track_id", job.Track.ID,
					"title", job.Track.Title,
					"path", job.Track.Path,
				)

				result, err := analyzer.Analyze(ctx, job.Track.Path)
				if err != nil {
					workerLogger.Error("audio job failed", "job_id", job.ID, "error", err)
					_ = store.FailAudioJob(ctx, job.ID, err)
					errCh <- err
					resultCh <- audioBatchSummary{failed: 1}
					continue
				}

				if err := store.UpsertTrackAudioFeatures(ctx, sqlite.AudioFeatureRecord{
					TrackID:                job.TrackID,
					AnalyzedAt:             result.AnalyzedAt,
					FileDurationSeconds:    result.Measured.FileDurationSeconds,
					MeasuredIntegratedLUFS: result.Measured.IntegratedLUFS,
					MeasuredTruePeak:       result.Measured.TruePeak,
					ReplayGainTrackGainDB:  result.ReplayGain.TrackGainDB,
					ReplayGainTrackPeak:    result.ReplayGain.TrackPeak,
					ReplayGainAlbumGainDB:  result.ReplayGain.AlbumGainDB,
					ReplayGainAlbumPeak:    result.ReplayGain.AlbumPeak,
					EffectiveGainDB:        result.Effective.GainDB,
					EffectivePeak:          result.Effective.Peak,
					EffectiveGainSource:    result.Effective.GainSource,
					EffectivePeakSource:    result.Effective.PeakSource,
				}); err != nil {
					_ = store.FailAudioJob(ctx, job.ID, err)
					errCh <- fmt.Errorf("persist audio features for job %d: %w", job.ID, err)
					resultCh <- audioBatchSummary{failed: 1}
					continue
				}

				if err := store.CompleteAudioJob(ctx, job.ID); err != nil {
					errCh <- fmt.Errorf("complete audio job %d: %w", job.ID, err)
					resultCh <- audioBatchSummary{failed: 1}
					continue
				}

				workerLogger.Info("audio job completed", "job_id", job.ID)
				resultCh <- audioBatchSummary{completed: 1}
			}
		}(i + 1)
	}

	go func() {
		defer close(jobCh)
		for _, job := range jobs {
			select {
			case <-ctx.Done():
				return
			case jobCh <- job:
			}
		}
	}()

	wg.Wait()
	close(errCh)
	close(resultCh)

	summary := audioBatchSummary{}
	for result := range resultCh {
		summary.completed += result.completed
		summary.failed += result.failed
	}
	for err := range errCh {
		if err != nil && !errors.Is(err, context.Canceled) {
			return summary, err
		}
	}

	return summary, ctx.Err()
}
