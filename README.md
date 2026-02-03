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
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "clog -i"
          }
        ]
      }
    ],
    "SessionEnd": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "clog -i"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "clog -i"
          }
        ]
      }
    ]
  }
}
```

Replace `clog` with `clog-ollama` if using the Ollama wrapper.

## Teaching Claude Code to use clog

Add the following to your global `~/.claude/CLAUDE.md` so Claude Code knows how to retrieve past conversations:

````markdown
## Retrieving previous conversations (clog)

`clog-ollama` is installed as a Claude Code hook and logs all session transcripts to a DuckDB database. Use it to recall past conversations when context from previous sessions would be helpful.

- **Text search** (no embeddings needed):
  ```bash
  clog-ollama -t "search pattern" -n 10
  ```
  Case-insensitive substring match across all harvested messages. Returns messages with timestamps, roles (`[user]`/`[assistant]`), and session IDs.

- **Semantic search** (requires embeddings via `clog-ollama -e`):
  ```bash
  clog-ollama -s "natural language query" -n 5
  ```
  Embeds the query and finds similar messages via cosine similarity.

- Use `-n` to control how many results are returned (default varies by mode).

When to use:
- The user references something from a past session ("remember when we...", "like we did before")
- You need to find how a problem was previously solved
- The user asks to search their conversation history
````

Replace `clog-ollama` with `clog` if not using the Ollama wrapper.

## Storage

Per-project DuckDB database at `~/.claude/logs/<project-slug>/events.duckdb`, where the project slug is the working directory with `/` replaced by `__`.
