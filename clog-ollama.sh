#!/bin/sh
export OLLAMA_HOST="${OLLAMA_HOST:-http://localhost:11434}"
export OLLAMA_EMBED_MODEL="${OLLAMA_EMBED_MODEL:-nomic-embed-text}"
export OLLAMA_CHAT_MODEL="${OLLAMA_CHAT_MODEL:-llama3.2}"
exec clog "$@"
