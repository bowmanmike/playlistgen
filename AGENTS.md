# Repository Guidelines

## Project Structure & Module Organization
`cmd/playlistgen` hosts the Cobra CLI entry point; keep every command and flag definition there so help text stays centralized. Place core logic under `internal/` packages such as `navidrome` (API sync), `audio` (ffmpeg analysis), `embeddings` (Ollama + sqlite-vec), and `playlist` (rule engine). SQLite migrations live in `db/migrations` for Goose, while `sqlc` query specs sit beside the packages that consume them. Docker artifacts belong in `deploy/compose`, and generated playlists should always write to `/playlists` (mounted into the container). Tests (`*_test.go`) sit next to their sources so `go test ./...` covers everything.

## Build, Test, and Development Commands
`go run ./cmd/playlistgen sync` performs a Navidrome metadata sync end-to-end. Set `NAVIDROME_URL` to the Subsonic base (e.g., `http://navidrome:4533/rest`) along with `NAVIDROME_USERNAME`/`NAVIDROME_PASSWORD` before running; use `--db-path`/`PLAYLISTGEN_DB_PATH` if you don’t want the default `data/db.sqlite`. `go build -o playlistgen ./cmd/playlistgen` produces the binary used in Docker. Execute `go test ./...` before every push; add `-run TestName` for focused work. Run `go fmt ./... && go vet ./...` to enforce formatting and static checks, and `sqlc generate` whenever query definitions change. Use `docker compose -f deploy/compose/docker-compose.yml up playlistgen` to exercise the full stack locally.

## Coding Style & Naming Conventions
Default to `gofmt`/`goimports` (tabs, grouped imports). CLI commands and flags use kebab-case (`playlistgen generate --energy-low`). Packages stay snake-free: `internal/navidrome`, not `internal/navidrome_client`. Exported types and funcs read as domain concepts (`PlaylistGenerator`, `EmbeddingStore`). Keep files focused (<350 lines) and document complex pipelines with concise block comments describing the rule or algorithm being enforced.

## Testing Guidelines
Lean on the standard `testing` package plus table-driven cases (`TestGeneratePlaylist_DiversityCap`). Integration tests that touch SQLite should create temporary databases under `t.TempDir()` and load Goose migrations before assertions. For audio/embedding flows, stub external binaries via interfaces so unit tests avoid running ffmpeg/Ollama. Every bug fix ships with a regression test, and PRs should state whether `go test ./...` and relevant Docker smoke tests passed.

## Commit & Pull Request Guidelines
Commits follow short, imperative subjects (e.g., `Add sqlite-vec store`). Multiline bodies note schema changes, new flags, or migration impacts. PRs link the corresponding PROJECT_PLAN milestone, describe architectural context (e.g., “implements step 2: Resolve local file paths”), include test output snippets, and mention any Docker-compose or configuration changes. Request review only after formatting, vetting, sqlc generation, and migrations have all been reapplied.
