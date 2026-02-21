package sqlite

const insertTrackSQL = `INSERT INTO tracks (navidrome_id, title, artist, artist_id, album, album_id, album_artist, genre, year, track_number, disc_number, duration_seconds, bitrate, file_size, path, content_type, suffix, created_at)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(navidrome_id) DO UPDATE SET
            title=excluded.title,
            artist=excluded.artist,
            artist_id=excluded.artist_id,
            album=excluded.album,
            album_id=excluded.album_id,
            album_artist=excluded.album_artist,
            genre=excluded.genre,
            year=excluded.year,
            track_number=excluded.track_number,
            disc_number=excluded.disc_number,
            duration_seconds=excluded.duration_seconds,
            bitrate=excluded.bitrate,
            file_size=excluded.file_size,
            path=excluded.path,
            content_type=excluded.content_type,
            suffix=excluded.suffix,
            created_at=excluded.created_at`

const upsertTrackSyncStatusSQL = `INSERT INTO navidrome_track_sync_status (track_id, navidrome_id, last_synced_at, sync_id)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(track_id) DO UPDATE SET
            last_synced_at=excluded.last_synced_at,
            sync_id=excluded.sync_id`

const createSyncSQL = `INSERT INTO navidrome_syncs (started_at, status)
VALUES (?, 'in_progress')`

const completeSyncSQL = `UPDATE navidrome_syncs
SET completed_at = ?, status = ?, tracks_processed = ?, tracks_updated = ?
WHERE id = ?`
