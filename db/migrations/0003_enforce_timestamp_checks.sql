-- +goose Up
-- +goose StatementBegin
CREATE TRIGGER IF NOT EXISTS trg_tracks_created_at_validate
BEFORE INSERT ON tracks
FOR EACH ROW
WHEN datetime(NEW.created_at) IS NULL
BEGIN
    SELECT RAISE(FAIL, 'tracks.created_at must be RFC3339');
END;

CREATE TRIGGER IF NOT EXISTS trg_tracks_created_at_validate_update
BEFORE UPDATE ON tracks
FOR EACH ROW
WHEN datetime(NEW.created_at) IS NULL
BEGIN
    SELECT RAISE(FAIL, 'tracks.created_at must be RFC3339');
END;

CREATE TRIGGER IF NOT EXISTS trg_syncs_started_at_validate
BEFORE INSERT ON navidrome_syncs
FOR EACH ROW
WHEN datetime(NEW.started_at) IS NULL
BEGIN
    SELECT RAISE(FAIL, 'navidrome_syncs.started_at must be RFC3339');
END;

CREATE TRIGGER IF NOT EXISTS trg_syncs_started_at_validate_update
BEFORE UPDATE ON navidrome_syncs
FOR EACH ROW
WHEN datetime(NEW.started_at) IS NULL
BEGIN
    SELECT RAISE(FAIL, 'navidrome_syncs.started_at must be RFC3339');
END;

CREATE TRIGGER IF NOT EXISTS trg_syncs_completed_at_validate
BEFORE INSERT ON navidrome_syncs
FOR EACH ROW
WHEN NEW.completed_at IS NOT NULL AND datetime(NEW.completed_at) IS NULL
BEGIN
    SELECT RAISE(FAIL, 'navidrome_syncs.completed_at must be RFC3339 when set');
END;

CREATE TRIGGER IF NOT EXISTS trg_syncs_completed_at_validate_update
BEFORE UPDATE ON navidrome_syncs
FOR EACH ROW
WHEN NEW.completed_at IS NOT NULL AND datetime(NEW.completed_at) IS NULL
BEGIN
    SELECT RAISE(FAIL, 'navidrome_syncs.completed_at must be RFC3339 when set');
END;

CREATE TRIGGER IF NOT EXISTS trg_track_sync_status_validate
BEFORE INSERT ON navidrome_track_sync_status
FOR EACH ROW
WHEN datetime(NEW.last_synced_at) IS NULL
BEGIN
    SELECT RAISE(FAIL, 'navidrome_track_sync_status.last_synced_at must be RFC3339');
END;

CREATE TRIGGER IF NOT EXISTS trg_track_sync_status_validate_update
BEFORE UPDATE ON navidrome_track_sync_status
FOR EACH ROW
WHEN datetime(NEW.last_synced_at) IS NULL
BEGIN
    SELECT RAISE(FAIL, 'navidrome_track_sync_status.last_synced_at must be RFC3339');
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS trg_track_sync_status_validate_update;
DROP TRIGGER IF EXISTS trg_track_sync_status_validate;
DROP TRIGGER IF EXISTS trg_syncs_completed_at_validate_update;
DROP TRIGGER IF EXISTS trg_syncs_completed_at_validate;
DROP TRIGGER IF EXISTS trg_syncs_started_at_validate_update;
DROP TRIGGER IF EXISTS trg_syncs_started_at_validate;
DROP TRIGGER IF EXISTS trg_tracks_created_at_validate_update;
DROP TRIGGER IF EXISTS trg_tracks_created_at_validate;
