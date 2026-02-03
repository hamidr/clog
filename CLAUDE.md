# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

clog is a Go CLI tool that acts as a Claude Code hook to capture session events, harvest conversation transcripts into a DuckDB database, and provide semantic and text search over historical interactions.

## Build & Run

```bash
# Build
go build -o clog .

# Nix
nix build .#clog          # bare binary
nix build .#clog-ollama   # wrapper with Ollama defaults
```

## CLI

All modes are explicit flags — running `clog` with no flags prints usage.

```bash
# Ingest a hook event from stdin (always exits 0)
echo '{"session_id":"...","cwd":"/...","hook_event_name":"Stop"}' | clog -i

# Embed unembedded messages
clog -e
clog --embed -n 500

# Semantic search (requires embeddings)
clog -s "query"
clog --search "query" -n 5

# Text search (case-insensitive substring, no embeddings needed)
clog -t "pattern"
clog --text-search "pattern" -n 10

# Search tool call events (requires PostToolUse hook)
clog -c "bash"
clog --commands "*" -n 10
clog -c "bash" -v              # include tool responses
```

There are no tests in the codebase currently.

## Architecture

### Package layout

- `main.go` — CLI entry point with flag parsing, orchestrates all modes
- `internal/config/` — Path resolution: log base (`~/.claude/logs`), project slug (`/foo/bar` → `foo__bar`), per-project DB path
- `internal/model/` — Domain types (`Session`, `Event`, `Message`, `SearchResult`) and JSON payload parsing from Claude Code hook schema
- `internal/store/` — DuckDB operations: schema init, session upsert, event insert, message persistence with atomic offset tracking, embedding storage, VSS-based similarity search, ILIKE text search
- `internal/transcript/` — Incremental JSONL parser for Claude Code transcripts; extracts user/assistant messages only; tracks file offset for resumable harvesting
- `internal/embedding/` — `Embedder` interface with `HTTPEmbedder` impl; supports Ollama (local), Voyage AI, and OpenAI; provider selected via env vars

### Data flow

```
clog -i (stdin JSON) → ParsePayload → DuckDB (UpsertSession + InsertEvent)
                                     ↓ (on Stop event with transcript_path)
                             transcript.Harvest (incremental JSONL read)
                                     ↓
                             SaveHarvestedMessages (atomic tx with offset)
                                     ↓ (later, manual)
                     clog -e → batch API calls → SaveEmbedding
                                     ↓
                     clog -s → embed query → SearchSimilar (VSS cosine)
```

### Key design decisions

- Ingest mode (`-i`) swallows all errors to stderr and exits 0 — it must never block Claude Code
- Transcript harvesting is incremental: `transcript_offsets` table tracks bytes read per file
- Messages are deduplicated by UUID (`ON CONFLICT DO NOTHING`)
- Embedding dimension is provider-dependent; the `message_embeddings` table schema is created dynamically with the correct `FLOAT[N]` array size
- DuckDB's VSS extension is used for `array_cosine_similarity` in search queries
- The `Event` struct uses pointer fields (`*string`, `*bool`) so each hook event type only populates relevant columns
- Embedding provider is auto-detected from env vars; Ollama requires both `OLLAMA_EMBED_MODEL` and `OLLAMA_HOST` to be set explicitly (no hardcoded defaults)

### Ollama wrapper

`clog-ollama.sh` (also the `clog-ollama` flake package) sets `OLLAMA_HOST` and `OLLAMA_EMBED_MODEL` defaults, then calls `clog`.

## Environment Variables

- `OLLAMA_EMBED_MODEL` — Ollama model name (checked first for embeddings)
- `OLLAMA_HOST` — Ollama address (required when using Ollama, usually `http://localhost:11434`)
- `VOYAGE_API_KEY` — Voyage AI API key
- `OPENAI_API_KEY` — OpenAI API key (fallback for embeddings)

## Storage

- Database: `~/.claude/logs/<project-slug>/events.duckdb`
- Project slug: CWD with leading `/` stripped and `/` replaced by `__`
