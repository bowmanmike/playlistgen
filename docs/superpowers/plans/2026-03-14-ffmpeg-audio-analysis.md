# FFmpeg Audio Analysis Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace simulated audio work with real file-based analysis that stores durable audio features and ReplayGain data for each track.

**Architecture:** Keep `track_audio_analysis` as the queue table and add separate durable tables for `track_audio_features` and `audio_processing_runs`. Implement an `internal/audio` package that resolves `/library`-relative files, invokes `ffprobe`/`ffmpeg` plus tag readers behind interfaces, computes effective values from ReplayGain-first precedence, and upserts results through the SQLite store. Wire `audio-process` to record run-level status and use the new analyzer, and update the Docker image so the deployed container includes `ffmpeg` and `ffprobe`.

**Tech Stack:** Go, SQLite, Goose migrations, sqlc, Cobra, `ffmpeg`, `ffprobe`, Docker

---

## File Structure

- Create: `db/migrations/0008_add_audio_feature_tables.sql`
  Responsibility: add durable audio feature storage and run logging tables.
- Modify: `db/queries/tracks.sql`
  Responsibility: add upsert and run-log queries alongside existing queue queries.
- Modify: `internal/db/models.go`
  Responsibility: regenerated sqlc models for audio feature and run tables.
- Modify: `internal/db/tracks.sql.go`
  Responsibility: regenerated sqlc query bindings for features and run logging.
- Create: `internal/audio/analyzer.go`
  Responsibility: define the analysis service, result structs, and precedence logic.
- Create: `internal/audio/ffmpeg.go`
  Responsibility: wrap `ffmpeg`/`ffprobe` execution behind narrow interfaces.
- Create: `internal/audio/replaygain.go`
  Responsibility: parse raw ReplayGain tags from files into normalized values.
- Create: `internal/audio/path.go`
  Responsibility: resolve Navidrome paths against the configured library root safely.
- Create: `internal/audio/analyzer_test.go`
  Responsibility: unit coverage for precedence, fallback, and path resolution behavior.
- Modify: `internal/storage/sqlite/store.go`
  Responsibility: add APIs to upsert audio features and create/update processing runs.
- Modify: `internal/storage/sqlite/store_test.go`
  Responsibility: integration coverage for feature persistence and run logging.
- Modify: `internal/cli/root.go`
  Responsibility: add audio library root config plumbing.
- Modify: `internal/cli/audio.go`
  Responsibility: replace simulated work with real analysis and run logging.
- Modify: `internal/cli/audio_test.go`
  Responsibility: cover CLI-level run logging and failure handling.
- Modify: `Dockerfile`
  Responsibility: install `ffmpeg` and `ffprobe` in the runtime image.
- Modify: `PROJECT_PLAN.md`
  Responsibility: mark ffmpeg analysis implemented/scaffolded accurately once complete.

## Chunk 1: Durable Audio Data Model

### Task 1: Add the audio features and run logging schema

**Files:**
- Create: `db/migrations/0008_add_audio_feature_tables.sql`
- Modify: `internal/storage/sqlite/store_test.go`
- Test: `internal/storage/sqlite/store_test.go`

- [ ] **Step 1: Write a failing integration test for durable audio features**

```go
func TestUpsertTrackAudioFeaturesPersistsMeasuredAndReplayGainValues(t *testing.T) {
    // Open a temp DB, run migrations via New, upsert one track, then persist a
    // feature row and assert one durable row exists with both measured and raw values.
}
```

- [ ] **Step 2: Run the targeted test to verify it fails**

Run: `go test ./internal/storage/sqlite -run TestUpsertTrackAudioFeaturesPersistsMeasuredAndReplayGainValues -v`
Expected: FAIL because no audio feature table or store API exists.

- [ ] **Step 3: Add the migration for features and run logging**

```sql
CREATE TABLE track_audio_features (
    track_id INTEGER NOT NULL PRIMARY KEY,
    analyzed_at TEXT NOT NULL,
    file_duration_seconds REAL NOT NULL,
    measured_integrated_lufs REAL,
    measured_true_peak REAL,
    replaygain_track_gain_db REAL,
    replaygain_track_peak REAL,
    replaygain_album_gain_db REAL,
    replaygain_album_peak REAL,
    effective_gain_db REAL,
    effective_peak REAL,
    effective_gain_source TEXT NOT NULL,
    effective_peak_source TEXT NOT NULL,
    FOREIGN KEY(track_id) REFERENCES tracks(id) ON DELETE CASCADE
);

CREATE TABLE audio_processing_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at TEXT NOT NULL,
    completed_at TEXT,
    status TEXT NOT NULL DEFAULT 'in_progress',
    jobs_claimed INTEGER NOT NULL DEFAULT 0,
    jobs_completed INTEGER NOT NULL DEFAULT 0,
    jobs_failed INTEGER NOT NULL DEFAULT 0
);
```

- [ ] **Step 4: Re-run the targeted test**

Run: `go test ./internal/storage/sqlite -run TestUpsertTrackAudioFeaturesPersistsMeasuredAndReplayGainValues -v`
Expected: still FAIL until the store writes the new table, but migrations apply cleanly.

- [ ] **Step 5: Commit**

```bash
git add db/migrations/0008_add_audio_feature_tables.sql internal/storage/sqlite/store_test.go
git commit -m "Add audio feature storage schema"
```

### Task 2: Add run logging expectations

**Files:**
- Modify: `internal/storage/sqlite/store_test.go`
- Create: `db/migrations/0008_add_audio_feature_tables.sql`
- Test: `internal/storage/sqlite/store_test.go`

- [ ] **Step 1: Add a failing test for audio processing runs**

```go
func TestAudioProcessingRunLifecycle(t *testing.T) {
    // Start a run, complete it with claimed/completed/failed counts, then assert
    // the durable row records timestamps and status.
}
```

- [ ] **Step 2: Run the targeted test to verify it fails**

Run: `go test ./internal/storage/sqlite -run TestAudioProcessingRunLifecycle -v`
Expected: FAIL because no run table or store API exists.

- [ ] **Step 3: Add any missing indexes or timestamp constraints in the migration**

```sql
CREATE INDEX idx_audio_processing_runs_started_at ON audio_processing_runs(started_at);
```

- [ ] **Step 4: Re-run the targeted test**

Run: `go test ./internal/storage/sqlite -run TestAudioProcessingRunLifecycle -v`
Expected: still FAIL until store/query support is added.

- [ ] **Step 5: Commit**

```bash
git add db/migrations/0008_add_audio_feature_tables.sql internal/storage/sqlite/store_test.go
git commit -m "Add audio processing run schema"
```

## Chunk 2: SQL And Store APIs

### Task 3: Add sqlc queries for features and run logs

**Files:**
- Modify: `db/queries/tracks.sql`
- Modify: `internal/db/tracks.sql.go`
- Modify: `internal/db/models.go`
- Test: `internal/storage/sqlite/store_test.go`

- [ ] **Step 1: Add failing store tests that require query-backed APIs**

```go
func TestStoreUpsertTrackAudioFeaturesUpdatesExistingRow(t *testing.T) {}
func TestStoreCompleteAudioProcessingRunUpdatesCounters(t *testing.T) {}
```

- [ ] **Step 2: Run the targeted tests to verify they fail**

Run: `go test ./internal/storage/sqlite -run 'TestStoreUpsertTrackAudioFeaturesUpdatesExistingRow|TestStoreCompleteAudioProcessingRunUpdatesCounters' -v`
Expected: FAIL because the query layer does not support feature upserts or run lifecycle writes.

- [ ] **Step 3: Add sqlc queries**

```sql
-- name: UpsertTrackAudioFeatures :exec
INSERT INTO track_audio_features (...)
VALUES (...)
ON CONFLICT(track_id) DO UPDATE SET ...;

-- name: CreateAudioProcessingRun :one
INSERT INTO audio_processing_runs (started_at, status)
VALUES (?, 'in_progress')
RETURNING id;

-- name: CompleteAudioProcessingRun :exec
UPDATE audio_processing_runs
SET completed_at = ?, status = ?, jobs_claimed = ?, jobs_completed = ?, jobs_failed = ?
WHERE id = ?;
```

- [ ] **Step 4: Regenerate sqlc output**

Run: `sqlc generate`
Expected: `internal/db/tracks.sql.go` and `internal/db/models.go` update with the new query bindings and models.

- [ ] **Step 5: Commit**

```bash
git add db/queries/tracks.sql internal/db/tracks.sql.go internal/db/models.go internal/storage/sqlite/store_test.go
git commit -m "Add audio feature sql queries"
```

### Task 4: Implement store methods for features and run logs

**Files:**
- Modify: `internal/storage/sqlite/store.go`
- Modify: `internal/storage/sqlite/store_test.go`
- Test: `internal/storage/sqlite/store_test.go`

- [ ] **Step 1: Add failing tests for public store methods**

```go
func TestStoreStartAudioProcessingRunReturnsID(t *testing.T) {}
func TestStoreCompleteAudioJobAndPersistFeatures(t *testing.T) {}
```

- [ ] **Step 2: Run the targeted tests to verify they fail**

Run: `go test ./internal/storage/sqlite -run 'TestStoreStartAudioProcessingRunReturnsID|TestStoreCompleteAudioJobAndPersistFeatures' -v`
Expected: FAIL because the store lacks those methods.

- [ ] **Step 3: Implement audio feature and run store APIs**

```go
type AudioFeatureRecord struct {
    TrackID int64
    AnalyzedAt time.Time
    FileDurationSeconds float64
    MeasuredIntegratedLUFS *float64
    ...
}

func (s *Store) UpsertTrackAudioFeatures(ctx context.Context, record AudioFeatureRecord) error { ... }
func (s *Store) StartAudioProcessingRun(ctx context.Context, startedAt time.Time) (int64, error) { ... }
func (s *Store) CompleteAudioProcessingRun(ctx context.Context, runID int64, summary AudioProcessingRunSummary) error { ... }
```

- [ ] **Step 4: Re-run the full storage package tests**

Run: `go test ./internal/storage/sqlite -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/sqlite/store.go internal/storage/sqlite/store_test.go
git commit -m "Implement audio feature store APIs"
```

## Chunk 3: Analysis Package

### Task 5: Build path resolution and precedence logic

**Files:**
- Create: `internal/audio/path.go`
- Create: `internal/audio/analyzer.go`
- Create: `internal/audio/analyzer_test.go`
- Test: `internal/audio/analyzer_test.go`

- [ ] **Step 1: Write failing analyzer tests for path resolution and precedence**

```go
func TestResolveLibraryPathJoinsConfiguredRoot(t *testing.T) {}
func TestEffectiveValuesPreferReplayGainOverMeasuredValues(t *testing.T) {}
func TestEffectiveValuesFallBackToMeasuredValues(t *testing.T) {}
```

- [ ] **Step 2: Run the targeted tests to verify they fail**

Run: `go test ./internal/audio -run 'TestResolveLibraryPathJoinsConfiguredRoot|TestEffectiveValuesPreferReplayGainOverMeasuredValues|TestEffectiveValuesFallBackToMeasuredValues' -v`
Expected: FAIL because the package does not exist yet.

- [ ] **Step 3: Implement minimal path and precedence code**

```go
func ResolveLibraryPath(root, navPath string) (string, error) {
    // Clean root, trim leading separators from navPath, join, and reject escapes.
}

func EffectiveValues(raw RawReplayGain, measured MeasuredAudio) EffectiveAudio {
    // Album/track ReplayGain wins when parseable; measured values become fallback.
}
```

- [ ] **Step 4: Re-run the targeted tests**

Run: `go test ./internal/audio -run 'TestResolveLibraryPathJoinsConfiguredRoot|TestEffectiveValuesPreferReplayGainOverMeasuredValues|TestEffectiveValuesFallBackToMeasuredValues' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/audio/path.go internal/audio/analyzer.go internal/audio/analyzer_test.go
git commit -m "Add audio path and precedence logic"
```

### Task 6: Add stub-friendly ffmpeg and tag-reading adapters

**Files:**
- Create: `internal/audio/ffmpeg.go`
- Create: `internal/audio/replaygain.go`
- Modify: `internal/audio/analyzer.go`
- Modify: `internal/audio/analyzer_test.go`
- Test: `internal/audio/analyzer_test.go`

- [ ] **Step 1: Add failing tests for analyzer orchestration**

```go
func TestAnalyzerUsesMeasuredAndTagDataToBuildRecord(t *testing.T) {}
func TestAnalyzerHandlesMissingReplayGainTags(t *testing.T) {}
```

- [ ] **Step 2: Run the targeted tests to verify they fail**

Run: `go test ./internal/audio -run 'TestAnalyzerUsesMeasuredAndTagDataToBuildRecord|TestAnalyzerHandlesMissingReplayGainTags' -v`
Expected: FAIL because there are no ffmpeg/tag adapters.

- [ ] **Step 3: Implement interfaces and analyzer orchestration**

```go
type ProbeRunner interface {
    Measure(context.Context, string) (MeasuredAudio, error)
}

type ReplayGainReader interface {
    Read(context.Context, string) (RawReplayGain, error)
}

type Analyzer struct {
    Root string
    Probe ProbeRunner
    Tags ReplayGainReader
}
```

- [ ] **Step 4: Re-run the analyzer package tests**

Run: `go test ./internal/audio -v`
Expected: PASS with stubbed command runners only.

- [ ] **Step 5: Commit**

```bash
git add internal/audio/ffmpeg.go internal/audio/replaygain.go internal/audio/analyzer.go internal/audio/analyzer_test.go
git commit -m "Add audio analyzer adapters"
```

## Chunk 4: CLI Integration And Runtime Packaging

### Task 7: Wire the audio CLI to real analysis and run logging

**Files:**
- Modify: `internal/cli/root.go`
- Modify: `internal/cli/audio.go`
- Modify: `internal/cli/audio_test.go`
- Test: `internal/cli/audio_test.go`

- [ ] **Step 1: Add failing CLI tests for library-root config and run logging**

```go
func TestRunAudioProcessUsesConfiguredLibraryRoot(t *testing.T) {}
func TestRunAudioProcessRecordsRunSummary(t *testing.T) {}
func TestRunAudioProcessMarksJobFailedOnAnalyzerError(t *testing.T) {}
```

- [ ] **Step 2: Run the targeted CLI tests to verify they fail**

Run: `go test ./internal/cli -run 'TestRunAudioProcessUsesConfiguredLibraryRoot|TestRunAudioProcessRecordsRunSummary|TestRunAudioProcessMarksJobFailedOnAnalyzerError' -v`
Expected: FAIL because the command still uses simulated work and has no run logging.

- [ ] **Step 3: Add audio root config and real analyzer wiring**

```go
cmd.Flags().StringVar(&cfg.libraryRoot, "library-root", "/library", "Mounted music library root")

runID, _ := store.StartAudioProcessingRun(...)
jobs, _ := store.ClaimPendingAudioJobs(...)
for each job {
    record, err := analyzer.Analyze(ctx, job.Track.Path)
    if err != nil { store.FailAudioJob(...); failed++ }
    else { store.UpsertTrackAudioFeatures(...); store.CompleteAudioJob(...); completed++ }
}
store.CompleteAudioProcessingRun(runID, summary)
```

- [ ] **Step 4: Re-run the full CLI package tests**

Run: `go test ./internal/cli -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/root.go internal/cli/audio.go internal/cli/audio_test.go
git commit -m "Integrate ffmpeg audio analysis into CLI"
```

### Task 8: Install ffmpeg in the Docker image and document current status

**Files:**
- Modify: `Dockerfile`
- Modify: `PROJECT_PLAN.md`
- Test: none

- [ ] **Step 1: Update the Docker image to install runtime tools**

```dockerfile
RUN apt-get update && \
    apt-get install -y --no-install-recommends ffmpeg && \
    rm -rf /var/lib/apt/lists/*
```

- [ ] **Step 2: Update project documentation**

```md
- Mark ffmpeg-backed audio analysis implemented.
- Mention ReplayGain-first precedence and `/library` default root.
```

- [ ] **Step 3: Review the rendered docs and Dockerfile**

Run: `sed -n '1,220p' Dockerfile && sed -n '100,180p' PROJECT_PLAN.md`
Expected: `ffmpeg`/`ffprobe` installation is explicit and docs match the code.

- [ ] **Step 4: Commit**

```bash
git add Dockerfile PROJECT_PLAN.md
git commit -m "Add ffmpeg to runtime image"
```

## Chunk 5: Verification

### Task 9: Run final verification and prepare handoff

**Files:**
- Modify: none
- Test: repo-wide

- [ ] **Step 1: Format the Go code**

Run: `gofmt -w internal/audio/*.go internal/cli/audio.go internal/cli/audio_test.go internal/cli/root.go internal/storage/sqlite/store.go internal/storage/sqlite/store_test.go`
Expected: no output.

- [ ] **Step 2: Regenerate sqlc if needed**

Run: `sqlc generate`
Expected: generated files are up to date.

- [ ] **Step 3: Run the full test suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 4: Run vet**

Run: `go vet ./...`
Expected: no findings.

- [ ] **Step 5: Review the diff**

Run: `git status --short`
Expected: only the intended migration, SQL, generated code, Go files, Dockerfile, and docs are changed.

- [ ] **Step 6: Commit the verification-ready state**

```bash
git add db/migrations/0008_add_audio_feature_tables.sql db/queries/tracks.sql internal/db/models.go internal/db/tracks.sql.go internal/audio/analyzer.go internal/audio/ffmpeg.go internal/audio/path.go internal/audio/replaygain.go internal/audio/analyzer_test.go internal/storage/sqlite/store.go internal/storage/sqlite/store_test.go internal/cli/root.go internal/cli/audio.go internal/cli/audio_test.go Dockerfile PROJECT_PLAN.md
git commit -m "Implement ffmpeg audio analysis"
```
