# FFmpeg Audio Analysis Design

## Goal

Implement real audio analysis for tracks queued by `audio-process`, using
`ffmpeg`/`ffprobe` plus ReplayGain tags from library files mounted into the
`playlistgen` container.

## Current Context

- `sync` already stores Navidrome track metadata, including the Navidrome
  `path`, and enqueues audio jobs.
- `audio-process` already claims jobs safely from SQLite and executes worker
  batches, but its work is still simulated.
- The `playlistgen` container has the media library mounted read-only at
  `/library`, and Navidrome-reported track paths are expected to resolve within
  that mount.
- The production path for audio analysis is the `playlistgen` Docker container,
  so `ffmpeg` and `ffprobe` must be installed in that image rather than assumed
  to exist only on a developer host.

## Design Summary

- Keep `track_audio_analysis` as the operational queue table only.
- Add a separate durable table for analysis results, one row per track.
- Analyze files by resolving the stored Navidrome path against a configurable
  library root with default `/library`.
- Persist both measured values and raw ReplayGain tag values.
- Persist effective values derived from explicit precedence rules.
- Treat failed jobs as terminal for now; future runs will not retry them unless
  they are explicitly re-queued.
- Log each `audio-process` invocation in a dedicated run table.

## Data Model

### Queue Table

`track_audio_analysis` remains responsible only for queue state:

- pending / processing / completed / failed state
- claim metadata
- attempts / error / timestamps

It should not store durable audio feature data.

### Results Table

Add a durable `track_audio_features` table with one row per `tracks.id`.

Fields should include:

- `track_id`
- `analyzed_at`
- `status` or a success indicator if needed
- file-derived duration
- measured integrated loudness
- measured true peak
- raw ReplayGain track gain
- raw ReplayGain track peak
- raw ReplayGain album gain
- raw ReplayGain album peak
- effective gain
- effective peak
- effective gain source
- effective peak source

The results table is the source for future playlist logic. Queue state is not.

### Run Logging Table

Add `audio_processing_runs` to log each CLI execution:

- `started_at`
- `completed_at`
- `status`
- `jobs_claimed`
- `jobs_completed`
- `jobs_failed`

This is intentionally lightweight and focused on run-level visibility.

## Precedence Rules

Store both measured and raw ReplayGain values, but define effective fields so
consumers do not need to re-implement the policy.

Precedence:

- ReplayGain tags win when present and parseable.
- Album-level ReplayGain fields are included in the first milestone because the
  playback model is expected to use album gain.
- Measured `ffmpeg` values are fallback and diagnostics.

Measured values should never be discarded, even when ReplayGain wins.

## Processing Flow

For each claimed audio job:

1. Read the stored Navidrome track path from the DB.
2. Resolve it against the library root, default `/library`.
3. Run file inspection/analysis:
   - read duration from the file
   - measure integrated loudness
   - measure true peak
   - read ReplayGain tags
4. Compute effective values using the precedence rules.
5. Upsert `track_audio_features` for the track.
6. Mark the queue job `completed`.

On failure:

- mark the queue row `failed`
- do not auto-retry
- leave any prior successful `track_audio_features` row intact

## Configuration

Add a library root configuration setting for audio analysis:

- default: `/library`
- available as CLI flag and environment variable

The stored Navidrome `path` remains canonical metadata. The library root is
only for resolving that relative path inside the `playlistgen` runtime.

## Container Requirements

The `playlistgen` image must include:

- `ffmpeg`
- `ffprobe`

The implementation should update the Docker image build so `audio-process`
works in the deployed container without extra manual package installation.

## Error Handling

First milestone behavior should stay simple:

- malformed/missing files fail the job
- ffmpeg/ffprobe failures fail the job
- tag parsing failures should be handled explicitly; if tags are unreadable but
  measured analysis succeeds, persist measured values and empty tag fields
- failed jobs remain failed until explicitly re-queued later

## Testing Strategy

Tests should cover:

- path resolution against `/library`
- successful feature extraction and upsert
- ReplayGain precedence over measured values
- fallback to measured values when ReplayGain is missing
- preserving prior successful features on later failure
- run logging counts for claimed/completed/failed jobs

Unit tests should stub external command execution so `ffmpeg` does not run in
normal package tests.

## Out of Scope

This milestone does not include:

- automatic retries for failed audio jobs
- embedding generation
- vector search
- playlist generation rules
- comparing/measuring disagreement policies beyond simple precedence
