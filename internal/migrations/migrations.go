package migrations

import (
	"database/sql"

	dbmigrations "github.com/bowmanmike/playlistgen/db/migrations"
)

// Run applies the SQLite migrations bundled with the binary.
func Run(db *sql.DB) error {
	return dbmigrations.Run(db)
}
