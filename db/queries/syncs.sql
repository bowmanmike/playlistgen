-- name: CreateSync :one
INSERT INTO navidrome_syncs (started_at, status)
VALUES (?, 'in_progress')
RETURNING id;

-- name: CompleteSync :exec
UPDATE navidrome_syncs
SET completed_at = ?, status = ?, tracks_processed = ?, tracks_updated = ?, tracks_deleted = ?
WHERE id = ?;
