-- name: UpsertTrack :exec
INSERT INTO tracks (
  navidrome_id,
  title,
  artist,
  artist_id,
  album,
  album_id,
  album_artist,
  genre,
  year,
  track_number,
  disc_number,
  duration_seconds,
  bitrate,
  file_size,
  path,
  content_type,
  suffix,
  created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(navidrome_id) DO UPDATE SET
  title = excluded.title,
  artist = excluded.artist,
  artist_id = excluded.artist_id,
  album = excluded.album,
  album_id = excluded.album_id,
  album_artist = excluded.album_artist,
  genre = excluded.genre,
  year = excluded.year,
  track_number = excluded.track_number,
  disc_number = excluded.disc_number,
  duration_seconds = excluded.duration_seconds,
  bitrate = excluded.bitrate,
  file_size = excluded.file_size,
  path = excluded.path,
  content_type = excluded.content_type,
  suffix = excluded.suffix,
  created_at = excluded.created_at;

-- name: SelectTrackID :one
SELECT id FROM tracks WHERE navidrome_id = ?;

-- name: UpsertTrackSyncStatus :exec
INSERT INTO navidrome_track_sync_status (
  track_id,
  navidrome_id,
  last_synced_at,
  sync_id
) VALUES (?, ?, ?, ?)
ON CONFLICT(track_id) DO UPDATE SET
  last_synced_at = excluded.last_synced_at,
  sync_id = excluded.sync_id;

-- name: ListTrackSyncStatus :many
SELECT track_id, navidrome_id, last_synced_at FROM navidrome_track_sync_status;

-- name: DeleteTracksByNavidromeIDs :exec
DELETE FROM tracks
WHERE navidrome_id IN (sqlc.slice('nav_ids'));

-- name: UpsertTrackAudioFeatures :exec
INSERT INTO track_audio_features (
  track_id,
  analyzed_at,
  file_duration_seconds,
  measured_integrated_lufs,
  measured_true_peak,
  replaygain_track_gain_db,
  replaygain_track_peak,
  replaygain_album_gain_db,
  replaygain_album_peak,
  effective_gain_db,
  effective_peak,
  effective_gain_source,
  effective_peak_source
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(track_id) DO UPDATE SET
  analyzed_at = excluded.analyzed_at,
  file_duration_seconds = excluded.file_duration_seconds,
  measured_integrated_lufs = excluded.measured_integrated_lufs,
  measured_true_peak = excluded.measured_true_peak,
  replaygain_track_gain_db = excluded.replaygain_track_gain_db,
  replaygain_track_peak = excluded.replaygain_track_peak,
  replaygain_album_gain_db = excluded.replaygain_album_gain_db,
  replaygain_album_peak = excluded.replaygain_album_peak,
  effective_gain_db = excluded.effective_gain_db,
  effective_peak = excluded.effective_peak,
  effective_gain_source = excluded.effective_gain_source,
  effective_peak_source = excluded.effective_peak_source;

-- name: CreateAudioProcessingRun :one
INSERT INTO audio_processing_runs (started_at, status)
VALUES (?, 'in_progress')
RETURNING id;

-- name: CompleteAudioProcessingRun :exec
UPDATE audio_processing_runs
SET completed_at = ?, status = ?, jobs_claimed = ?, jobs_completed = ?, jobs_failed = ?
WHERE id = ?;

-- name: EnsureTrackAudioJob :exec
INSERT INTO track_audio_analysis (
  track_id,
  status,
  processed_at,
  error,
  attempts,
  last_attempt_at,
  claimed_at,
  claimed_by
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(track_id) WHERE status IN ('pending', 'processing') DO UPDATE SET
  status = CASE
    WHEN track_audio_analysis.status = 'processing' THEN 'processing'
    ELSE 'pending'
  END,
  processed_at = CASE
    WHEN track_audio_analysis.status = 'processing' THEN track_audio_analysis.processed_at
    ELSE excluded.processed_at
  END,
  error = CASE
    WHEN track_audio_analysis.status = 'processing' THEN track_audio_analysis.error
    ELSE excluded.error
  END,
  attempts = CASE
    WHEN track_audio_analysis.status = 'processing' THEN track_audio_analysis.attempts
    ELSE excluded.attempts
  END,
  last_attempt_at = CASE
    WHEN track_audio_analysis.status = 'processing' THEN track_audio_analysis.last_attempt_at
    ELSE excluded.last_attempt_at
  END,
  claimed_at = CASE
    WHEN track_audio_analysis.status = 'processing' THEN track_audio_analysis.claimed_at
    ELSE excluded.claimed_at
  END,
  claimed_by = CASE
    WHEN track_audio_analysis.status = 'processing' THEN track_audio_analysis.claimed_by
    ELSE excluded.claimed_by
  END;

-- name: EnsureTrackEmbeddingJob :exec
INSERT INTO track_embedding_jobs (
  track_id,
  status,
  processed_at,
  error,
  attempts,
  last_attempt_at,
  claimed_at,
  claimed_by
) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(track_id) WHERE status IN ('pending', 'processing') DO UPDATE SET
  status = CASE
    WHEN track_embedding_jobs.status = 'processing' THEN 'processing'
    ELSE 'pending'
  END,
  processed_at = CASE
    WHEN track_embedding_jobs.status = 'processing' THEN track_embedding_jobs.processed_at
    ELSE excluded.processed_at
  END,
  error = CASE
    WHEN track_embedding_jobs.status = 'processing' THEN track_embedding_jobs.error
    ELSE excluded.error
  END,
  attempts = CASE
    WHEN track_embedding_jobs.status = 'processing' THEN track_embedding_jobs.attempts
    ELSE excluded.attempts
  END,
  last_attempt_at = CASE
    WHEN track_embedding_jobs.status = 'processing' THEN track_embedding_jobs.last_attempt_at
    ELSE excluded.last_attempt_at
  END,
  claimed_at = CASE
    WHEN track_embedding_jobs.status = 'processing' THEN track_embedding_jobs.claimed_at
    ELSE excluded.claimed_at
  END,
  claimed_by = CASE
    WHEN track_embedding_jobs.status = 'processing' THEN track_embedding_jobs.claimed_by
    ELSE excluded.claimed_by
  END;

-- name: ClaimPendingAudioJobs :many
UPDATE track_audio_analysis
SET status = 'processing',
    claimed_at = ?,
    claimed_by = ?,
    error = NULL
WHERE id IN (
  SELECT id
  FROM track_audio_analysis
  WHERE track_audio_analysis.status = 'pending'
     OR (
       track_audio_analysis.status = 'processing'
       AND track_audio_analysis.claimed_at IS NOT NULL
       AND track_audio_analysis.claimed_at <= ?
     )
  ORDER BY track_audio_analysis.created_at, track_audio_analysis.id
  LIMIT ?
)
RETURNING id, track_id;

-- name: ListAudioJobsByIDs :many
SELECT
  track_audio_analysis.id AS job_id,
  sqlc.embed(tracks)
FROM track_audio_analysis
JOIN tracks ON tracks.id = track_audio_analysis.track_id
WHERE track_audio_analysis.id IN (sqlc.slice('job_ids'))
ORDER BY track_audio_analysis.id;

-- name: ListPendingAudioJobs :many
SELECT
  track_audio_analysis.id AS job_id,
  sqlc.embed(tracks)
FROM track_audio_analysis
JOIN tracks ON tracks.id = track_audio_analysis.track_id
WHERE track_audio_analysis.status = 'pending'
ORDER BY track_audio_analysis.created_at, track_audio_analysis.id
LIMIT ?;

-- name: UpdateAudioJobStatus :exec
UPDATE track_audio_analysis
SET status = ?,
    processed_at = ?,
    error = ?,
    attempts = attempts + 1,
    last_attempt_at = ?,
    claimed_at = ?,
    claimed_by = ?
WHERE id = ?;
