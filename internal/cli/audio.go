package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/bowmanmike/playlistgen/internal/storage/sqlite"
)

type audioProcessConfig struct {
	batchSize   int
	workerCount int
	processAll  bool
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

	dbPath, err := filepath.Abs(opts.dbPath)
	if err != nil {
		return fmt.Errorf("resolve db path: %w", err)
	}

	store, err := sqlite.New(sqlite.Config{Path: dbPath})
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer store.Close()

	totalProcessed := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		jobs, err := store.ListPendingAudioJobs(ctx, cfg.batchSize)
		if err != nil {
			return fmt.Errorf("list audio jobs: %w", err)
		}
		if len(jobs) == 0 {
			if totalProcessed == 0 {
				logger.Info("no pending audio jobs found")
			}
			break
		}

		logger.Info("processing audio batch",
			"jobs", len(jobs),
			"workers", cfg.workerCount,
		)
		if err := processAudioBatch(ctx, store, jobs, cfg.workerCount, logger); err != nil {
			return err
		}
		totalProcessed += len(jobs)

		if !cfg.processAll {
			break
		}
	}

	logger.Info("audio processing complete", "processed_jobs", totalProcessed)
	return nil
}

func processAudioBatch(ctx context.Context, store *sqlite.Store, jobs []sqlite.AudioJob, workers int, logger *slog.Logger) error {
	jobCh := make(chan sqlite.AudioJob)
	errCh := make(chan error, workers)

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

				if err := simulateAudioWork(ctx); err != nil {
					workerLogger.Error("audio job failed", "job_id", job.ID, "error", err)
					_ = store.FailAudioJob(ctx, job.ID, err)
					errCh <- err
					continue
				}

				if err := store.CompleteAudioJob(ctx, job.ID); err != nil {
					errCh <- fmt.Errorf("complete audio job %d: %w", job.ID, err)
					continue
				}

				workerLogger.Info("audio job completed", "job_id", job.ID)
			}
		}(i + 1)
	}

	go func() {
		for _, job := range jobs {
			select {
			case <-ctx.Done():
				return
			case jobCh <- job:
			}
		}
		close(jobCh)
	}()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
	}

	return ctx.Err()
}

func simulateAudioWork(ctx context.Context) error {
	timer := time.NewTimer(100 * time.Millisecond)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
