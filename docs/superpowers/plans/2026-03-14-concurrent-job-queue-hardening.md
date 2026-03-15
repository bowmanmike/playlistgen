# Concurrent Job Queue Hardening Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make audio and embedding jobs safe under concurrent runners by moving claim ownership and deduplication into SQLite, while fixing cancellation behavior and updating the project documentation to match the new queue model.

**Architecture:** Keep the existing `track_audio_analysis` and `track_embedding_jobs` tables, but turn them into claimable queues. Runners must atomically claim jobs in the database before processing them, sync must ensure there is at most one active job per track/job type, and stale claims must be reclaimable after a timeout. Preserve the current CLI shape and store-centered queue logic so the change stays local to migrations, SQL queries, storage, and the audio command.

**Tech Stack:** Go, SQLite, Goose migrations, sqlc, Cobra, standard `testing` package

---

## File Structure

- Modify: `db/migrations/0005_create_processing_jobs.sql`
  Responsibility: understand current queue schema and mirror its naming/style in the new migration.
- Create: `db/migrations/0007_harden_processing_job_queue.sql`
  Responsibility: add claim metadata, processing state support, active-job uniqueness, and indexes needed for claim/reclaim queries.
- Modify: `db/queries/tracks.sql`
  Responsibility: define claim, ensure-job, reclaim, and status update queries for audio and embedding jobs.
- Modify: `internal/db/tracks.sql.go`
  Responsibility: regenerated sqlc output for the new queries.
- Modify: `internal/storage/sqlite/store.go`
  Responsibility: move queue rules into store methods, replace append-only enqueueing with ensure/reset semantics, add atomic claim APIs, and support stale-claim recovery.
- Modify: `internal/storage/sqlite/store_test.go`
  Responsibility: regression coverage for dedupe, claim atomicity, stale reclaim, and updated sync behavior.
- Create: `internal/cli/audio_test.go`
  Responsibility: coverage for cancellation and command interaction with the claim-based store API.
- Modify: `internal/cli/audio.go`
  Responsibility: switch from read-only job listing to database claims and fix goroutine shutdown on cancellation.
- Modify: `PROJECT_PLAN.md`
  Responsibility: document the queue accurately, fix formatting drift, and describe current implementation status.

## Chunk 1: Queue Schema

### Task 1: Define the queue state model

**Files:**
- Modify: `db/migrations/0005_create_processing_jobs.sql`
- Create: `db/migrations/0007_harden_processing_job_queue.sql`
- Test: `internal/storage/sqlite/store_test.go`

- [ ] **Step 1: Write failing migration expectations in store tests**

```go
func TestSaveTracksDoesNotCreateDuplicatePendingJobs(t *testing.T) {
    // Save the same changed track twice and assert one active audio job
    // and one active embedding job remain for the track.
}
```

- [ ] **Step 2: Run the targeted test to verify it fails**

Run: `GOCACHE=$(pwd)/.gocache go test ./internal/storage/sqlite -run TestSaveTracksDoesNotCreateDuplicatePendingJobs -v`
Expected: FAIL because sync currently inserts duplicate pending rows.

- [ ] **Step 3: Add the migration**

```sql
ALTER TABLE track_audio_analysis ADD COLUMN claimed_at TEXT;
ALTER TABLE track_audio_analysis ADD COLUMN claimed_by TEXT;

ALTER TABLE track_embedding_jobs ADD COLUMN claimed_at TEXT;
ALTER TABLE track_embedding_jobs ADD COLUMN claimed_by TEXT;

CREATE UNIQUE INDEX idx_audio_one_active_job_per_track
ON track_audio_analysis(track_id)
WHERE status IN ('pending', 'processing');

CREATE UNIQUE INDEX idx_embedding_one_active_job_per_track
ON track_embedding_jobs(track_id)
WHERE status IN ('pending', 'processing');

CREATE INDEX idx_audio_claim_lookup
ON track_audio_analysis(status, claimed_at, created_at, id);
```

- [ ] **Step 4: Re-run the targeted test**

Run: `GOCACHE=$(pwd)/.gocache go test ./internal/storage/sqlite -run TestSaveTracksDoesNotCreateDuplicatePendingJobs -v`
Expected: still FAIL until queue logic is updated, but migrations apply cleanly.

- [ ] **Step 5: Commit**

```bash
git add db/migrations/0007_harden_processing_job_queue.sql internal/storage/sqlite/store_test.go
git commit -m "Add queue claim metadata"
```

### Task 2: Encode stale-claim recovery expectations

**Files:**
- Modify: `internal/storage/sqlite/store_test.go`
- Create: `db/migrations/0007_harden_processing_job_queue.sql`
- Test: `internal/storage/sqlite/store_test.go`

- [ ] **Step 1: Add a failing stale-reclaim test**

```go
func TestClaimPendingAudioJobsReclaimsStaleProcessingJobs(t *testing.T) {
    // Seed a processing row with an old claimed_at timestamp.
    // Claim jobs and assert the same row is returned to the new runner.
}
```

- [ ] **Step 2: Run the targeted test to verify it fails**

Run: `GOCACHE=$(pwd)/.gocache go test ./internal/storage/sqlite -run TestClaimPendingAudioJobsReclaimsStaleProcessingJobs -v`
Expected: FAIL because the store has no reclaim path.

- [ ] **Step 3: Confirm the migration indexes support reclaim scans**

```sql
CREATE INDEX idx_embedding_claim_lookup
ON track_embedding_jobs(status, claimed_at, created_at, id);
```

- [ ] **Step 4: Re-run the targeted test**

Run: `GOCACHE=$(pwd)/.gocache go test ./internal/storage/sqlite -run TestClaimPendingAudioJobsReclaimsStaleProcessingJobs -v`
Expected: still FAIL until claim logic exists, but schema is ready.

- [ ] **Step 5: Commit**

```bash
git add db/migrations/0007_harden_processing_job_queue.sql internal/storage/sqlite/store_test.go
git commit -m "Prepare queue schema for stale claim recovery"
```

## Chunk 2: Store Queries And APIs

### Task 3: Add sqlc queries for ensure/reset and claim

**Files:**
- Modify: `db/queries/tracks.sql`
- Modify: `internal/db/tracks.sql.go`
- Test: `internal/storage/sqlite/store_test.go`

- [ ] **Step 1: Add failing tests for claim and dedupe behavior**

```go
func TestClaimPendingAudioJobsClaimsRowsAtomically(t *testing.T) {
    // Claim once and assert the row status becomes processing with claimant metadata.
}
```

- [ ] **Step 2: Run the targeted tests to verify they fail**

Run: `GOCACHE=$(pwd)/.gocache go test ./internal/storage/sqlite -run 'TestClaimPendingAudioJobsClaimsRowsAtomically|TestSaveTracksDoesNotCreateDuplicatePendingJobs' -v`
Expected: FAIL because sqlc/store do not support claim or ensure semantics.

- [ ] **Step 3: Replace insert-only queue queries with ensure and claim queries**

```sql
-- name: EnsureTrackAudioJob :exec
INSERT INTO track_audio_analysis (...)
VALUES (...)
ON CONFLICT(track_id) WHERE status IN ('pending', 'processing')
DO UPDATE SET
  status = 'pending',
  processed_at = NULL,
  error = NULL,
  attempts = 0,
  last_attempt_at = NULL,
  claimed_at = NULL,
  claimed_by = NULL;

-- name: ClaimPendingAudioJobs :many
WITH claimable AS (
  SELECT id
  FROM track_audio_analysis
  WHERE status = 'pending'
     OR (status = 'processing' AND claimed_at < ?)
  ORDER BY created_at, id
  LIMIT ?
)
UPDATE track_audio_analysis
SET status = 'processing', claimed_at = ?, claimed_by = ?
WHERE id IN (SELECT id FROM claimable)
RETURNING id, ...;
```

- [ ] **Step 4: Regenerate sqlc output**

Run: `sqlc generate`
Expected: `internal/db/tracks.sql.go` updates with the new query methods and parameter structs.

- [ ] **Step 5: Commit**

```bash
git add db/queries/tracks.sql internal/db/tracks.sql.go internal/storage/sqlite/store_test.go
git commit -m "Add claimable queue sql queries"
```

### Task 4: Implement store-level queue APIs

**Files:**
- Modify: `internal/storage/sqlite/store.go`
- Modify: `internal/storage/sqlite/store_test.go`
- Test: `internal/storage/sqlite/store_test.go`

- [ ] **Step 1: Add failing tests for the public store methods**

```go
func TestStoreClaimPendingAudioJobsSetsClaimFields(t *testing.T) {}
func TestCompleteAudioJobClearsClaimFields(t *testing.T) {}
func TestFailAudioJobRetainsAttemptsAndClearsOwnership(t *testing.T) {}
```

- [ ] **Step 2: Run the targeted tests to verify they fail**

Run: `GOCACHE=$(pwd)/.gocache go test ./internal/storage/sqlite -run 'TestStoreClaimPendingAudioJobsSetsClaimFields|TestCompleteAudioJobClearsClaimFields|TestFailAudioJobRetainsAttemptsAndClearsOwnership' -v`
Expected: FAIL because the store does not expose claim-based queue methods yet.

- [ ] **Step 3: Implement claim-based store methods**

```go
type ClaimOptions struct {
    Limit          int
    ClaimedBy      string
    StaleAfter     time.Duration
    Now            time.Time
}

func (s *Store) ClaimPendingAudioJobs(ctx context.Context, opts ClaimOptions) ([]AudioJob, error) {
    // Normalize opts, run the sqlc claim query, and map rows to AudioJob values.
}
```

- [ ] **Step 4: Update sync enqueueing to ensure a single active job**

```go
func enqueueProcessingJobs(ctx context.Context, queries *db.Queries, trackID int64) error {
    if err := queries.EnsureTrackAudioJob(ctx, params); err != nil { ... }
    if err := queries.EnsureTrackEmbeddingJob(ctx, params); err != nil { ... }
    return nil
}
```

- [ ] **Step 5: Re-run the storage tests**

Run: `GOCACHE=$(pwd)/.gocache go test ./internal/storage/sqlite -v`
Expected: PASS, including duplicate-prevention, claim, and reclaim regressions.

- [ ] **Step 6: Commit**

```bash
git add internal/storage/sqlite/store.go internal/storage/sqlite/store_test.go
git commit -m "Implement claim-based queue store"
```

## Chunk 3: Runner Behavior

### Task 5: Switch the audio command to claims instead of plain reads

**Files:**
- Modify: `internal/cli/audio.go`
- Create: `internal/cli/audio_test.go`
- Test: `internal/cli/audio_test.go`

- [ ] **Step 1: Add a failing command test for claim usage**

```go
func TestRunAudioProcessClaimsJobsFromStore(t *testing.T) {
    // Stub store methods and assert runAudioProcess calls ClaimPendingAudioJobs,
    // not a read-only listing path.
}
```

- [ ] **Step 2: Add a failing cancellation regression test**

```go
func TestProcessAudioBatchReturnsOnContextCancel(t *testing.T) {
    // Cancel the context while the producer is active and assert the function returns.
}
```

- [ ] **Step 3: Run the targeted CLI tests to verify they fail**

Run: `GOCACHE=$(pwd)/.gocache go test ./internal/cli -run 'TestRunAudioProcessClaimsJobsFromStore|TestProcessAudioBatchReturnsOnContextCancel' -v`
Expected: FAIL because the command still lists pending jobs directly and can hang on cancellation.

- [ ] **Step 4: Refactor the command around a claim-capable store interface**

```go
type audioJobStore interface {
    ClaimPendingAudioJobs(context.Context, sqlite.ClaimOptions) ([]sqlite.AudioJob, error)
    CompleteAudioJob(context.Context, int64) error
    FailAudioJob(context.Context, int64, error) error
    Close() error
}
```

- [ ] **Step 5: Fix worker shutdown semantics**

```go
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
```

- [ ] **Step 6: Re-run the CLI package tests**

Run: `GOCACHE=$(pwd)/.gocache go test ./internal/cli -v`
Expected: PASS, including the cancellation regression.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/audio.go internal/cli/audio_test.go
git commit -m "Claim jobs atomically in audio runner"
```

### Task 6: Verify behavior under overlapping claim attempts

**Files:**
- Modify: `internal/storage/sqlite/store_test.go`
- Modify: `internal/cli/audio_test.go`
- Test: `internal/storage/sqlite/store_test.go`

- [ ] **Step 1: Add a concurrency regression test**

```go
func TestClaimPendingAudioJobsDoesNotReturnSameJobTwice(t *testing.T) {
    // Start two goroutines that claim from the same DB and assert disjoint job IDs.
}
```

- [ ] **Step 2: Run the targeted test**

Run: `GOCACHE=$(pwd)/.gocache go test ./internal/storage/sqlite -run TestClaimPendingAudioJobsDoesNotReturnSameJobTwice -v`
Expected: PASS after the claim implementation; FAIL if duplicate claims remain possible.

- [ ] **Step 3: Add a narrow CLI smoke test if needed**

```go
func TestRunAudioProcessExitsWhenNoJobsCanBeClaimed(t *testing.T) {}
```

- [ ] **Step 4: Re-run the queue-focused tests**

Run: `GOCACHE=$(pwd)/.gocache go test ./internal/storage/sqlite ./internal/cli -run 'TestClaimPendingAudioJobsDoesNotReturnSameJobTwice|TestRunAudioProcessExitsWhenNoJobsCanBeClaimed' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/sqlite/store_test.go internal/cli/audio_test.go
git commit -m "Cover concurrent queue claims"
```

## Chunk 4: Documentation And Final Verification

### Task 7: Bring `PROJECT_PLAN.md` back in sync

**Files:**
- Modify: `PROJECT_PLAN.md`
- Test: none

- [ ] **Step 1: Update the queue description**

```md
- Sync records per-track queue intent and ensures one active processing job per track.
- Workers claim jobs atomically in SQLite before processing.
```

- [ ] **Step 2: Fix the broken Docker YAML fence and stale checklist items**

```md
- Mark incremental sync as implemented.
- Clarify audio processing is scaffolded, not ffmpeg-backed yet.
- Remove "sqlc (planned)" since generated sqlc code is already in use.
```

- [ ] **Step 3: Review the rendered markdown locally**

Run: `sed -n '1,220p' PROJECT_PLAN.md`
Expected: valid fences, accurate status, and no obviously stale claims.

- [ ] **Step 4: Commit**

```bash
git add PROJECT_PLAN.md
git commit -m "Refresh project plan documentation"
```

### Task 8: Run full verification before handoff

**Files:**
- Modify: none
- Test: `internal/storage/sqlite/store_test.go`, `internal/cli/audio_test.go`, full repo

- [ ] **Step 1: Format the code**

Run: `gofmt -w internal/storage/sqlite/store.go internal/storage/sqlite/store_test.go internal/cli/audio.go internal/cli/audio_test.go`
Expected: no output.

- [ ] **Step 2: Run the full test suite**

Run: `GOCACHE=$(pwd)/.gocache go test ./...`
Expected: PASS.

- [ ] **Step 3: Run vet**

Run: `GOCACHE=$(pwd)/.gocache go vet ./...`
Expected: no findings.

- [ ] **Step 4: Review the diff**

Run: `git status --short`
Expected: only the intended migration, SQL, generated code, Go code, tests, and `PROJECT_PLAN.md` changes.

- [ ] **Step 5: Commit the verification-ready state**

```bash
git add db/migrations/0007_harden_processing_job_queue.sql db/queries/tracks.sql internal/db/tracks.sql.go internal/storage/sqlite/store.go internal/storage/sqlite/store_test.go internal/cli/audio.go internal/cli/audio_test.go PROJECT_PLAN.md
git commit -m "Harden processing queue for concurrent runners"
```
