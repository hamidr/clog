# clog

A Claude Code hook that logs session events and conversation transcripts into a per-project DuckDB database, with semantic and text search.

## Install

### Nix flake

```sh
# bare binary
nix profile install .#clog

# wrapper with Ollama defaults (nomic-embed-text, localhost:11434)
nix profile install .#clog-ollama
```

### From source

```sh
go build -o clog .
```

## Usage

```sh
clog -i                          # ingest a hook event from stdin
clog -e [-n NUM]                 # embed unembedded messages
clog -s [-n NUM] "query"         # semantic search (requires embeddings)
clog -t [-n NUM] "pattern"       # case-insensitive text search

clog --ingest                    # long forms
clog --embed
clog --search "query"
clog --text-search "pattern"
```

## Embedding providers

The first matching provider is used:

| Variable | Provider | Model |
|---|---|---|
| `OLLAMA_EMBED_MODEL` | Ollama (local) | value of the variable (e.g. `nomic-embed-text`) |
| `VOYAGE_API_KEY` | Voyage AI | `voyage-3-lite` |
| `OPENAI_API_KEY` | OpenAI | `text-embedding-3-small` |

When using Ollama, `OLLAMA_HOST` must also be set (usually `http://localhost:11434`). The `clog-ollama` wrapper sets both defaults.

## Hook setup

Register in your Claude Code hooks config (`~/.claude/settings.json`):

```json
{
  "hooks": {
    "Stop": [
      {
        "type": "command",
        "command": "clog -i"
      }
    ]
  }
}
```

## Storage

Per-project DuckDB database at `~/.claude/logs/<project-slug>/events.duckdb`, where the project slug is the working directory with `/` replaced by `__`.
