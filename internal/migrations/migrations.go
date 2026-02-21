package migrations

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed ../../db/migrations/*.sql
var embedMigrations embed.FS

func Run(db *sql.DB) error {
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(db, "../../db/migrations"); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
