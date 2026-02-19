# AI-Assisted Playlists – Project Plan

## Overview

Build a self-hosted, AI-assisted playlist generator for a Navidrome media library.

Core principles:

- Use **local embeddings** for semantic retrieval.
- Use **rule-based logic** for duration, diversity, and energy shaping.
- Keep Navidrome as the **source of truth**.
- Run entirely locally (no external APIs required).
- Start CLI-first; add UI later.

Target library size: ~8–9k tracks  
Server: i7-6700T, 16GB RAM  
All services run via Docker Compose.

---

# Architecture

## High-Level Flow

1. Sync metadata from Navidrome
2. Resolve local file paths
3. Analyze audio (ffmpeg, optional but recommended)
4. Generate embeddings (Ollama)
5. Store data in SQLite (+ sqlite-vec)
6. Generate playlists via:
   - Prompt embedding
   - Vector search (KNN)
   - Rule-based selection
   - Output `.m3u8`

Navidrome remains canonical.

---

# Tech Decisions

## Language
- **Go**

## CLI
- `cobra`

## Database
- SQLite
- `sqlc` for typed queries
- `goose` (or migrate) for migrations

## Vector Search
- `sqlite-vec` extension

## Embeddings
- **Ollama** (local embedding model)

## Audio Analysis
- `ffmpeg`
- Extract:
  - LUFS (integrated loudness)
  - true peak
  - optional RMS
- ReplayGain read from tags if available

## Logging
- `slog`

## Concurrency
- `errgroup`
- Bounded worker pool

## Deployment
- Docker Compose
- Job-container pattern
- Host `systemd` timer for periodic sync

---

# Docker Deployment Model

## Compose Service

```yaml
services:
  playlistgen:
    image: playlistgen:latest
    volumes:
      - /mnt/media:/library:ro
      - ./playlistgen-data:/data
      - ./playlists:/playlists
    environment:
      - DB_PATH=/data/playlist.db
      - NAVIDROME_URL=http://navidrome:4533
      - OLLAMA_URL=http://ollama:11434