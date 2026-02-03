package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"clog/internal/config"
	"clog/internal/embedding"
	"clog/internal/model"
	"clog/internal/store"
	"clog/internal/transcript"
)

const embeddingBatchSize = 64

func main() {
	ingest := flag.Bool("i", false, "")
	ingestLong := flag.Bool("ingest", false, "read a Claude Code hook event from stdin")
	embed := flag.Bool("e", false, "")
	embedLong := flag.Bool("embed", false, "embed unembedded messages")
	search := flag.String("s", "", "")
	searchLong := flag.String("search", "", "semantic search query")
	text := flag.String("t", "", "")
	textLong := flag.String("text-search", "", "case-insensitive text search pattern")
	n := flag.Int("n", 0, "max results or messages")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `clog - Claude Code session logger with search

usage: clog [options]

options:
  -i, --ingest               read a Claude Code hook event from stdin
  -e, --embed                embed unembedded messages
  -s, --search QUERY         semantic search over embeddings
  -t, --text-search PATTERN  case-insensitive substring search
  -n NUM                     max results/messages (default: varies per mode)

environment:
  OLLAMA_EMBED_MODEL   local Ollama model (checked first)
  OLLAMA_HOST          Ollama address (usually http://localhost:11434)
  VOYAGE_API_KEY       Voyage AI API key
  OPENAI_API_KEY       OpenAI API key
`)
	}

	flag.Parse()

	// Merge short and long forms.
	if *ingestLong {
		*ingest = true
	}
	if *embedLong {
		*embed = true
	}
	if *searchLong != "" {
		*search = *searchLong
	}
	if *textLong != "" {
		*text = *textLong
	}

	mode := 0
	if *ingest {
		mode++
	}
	if *embed {
		mode++
	}
	if *search != "" {
		mode++
	}
	if *text != "" {
		mode++
	}

	if mode == 0 {
		flag.Usage()
		os.Exit(2)
	}
	if mode > 1 {
		fmt.Fprintln(os.Stderr, "clog: specify only one of -i, -e, -s, -t")
		os.Exit(2)
	}

	var err error
	switch {
	case *ingest:
		if err := runHook(); err != nil {
			fmt.Fprintf(os.Stderr, "clog: %v\n", err)
		}
		os.Exit(0) // never block Claude
	case *embed:
		if *n == 0 {
			*n = 10000
		}
		err = runEmbed(*n)
	case *search != "":
		if *n == 0 {
			*n = 10
		}
		err = runSearch(*search, *n)
	case *text != "":
		if *n == 0 {
			*n = 20
		}
		err = runTextSearch(*text, *n)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "clog: %v\n", err)
		os.Exit(1)
	}
}

// --- Hook mode (stdin, always exits 0) ---

func runHook() error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}

	parsed, err := model.ParsePayload(data)
	if err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	cfg := config.Default()
	dbPath := cfg.DBPath(parsed.Session.CWD)

	if err := os.MkdirAll(cfg.LogDir(parsed.Session.CWD), 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	if err := st.InitCoreSchema(); err != nil {
		return err
	}

	if err := st.UpsertSession(parsed.Session); err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}

	if err := st.InsertEvent(parsed.Event); err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	if parsed.Event.EventType == "Stop" && parsed.Session.TranscriptPath != "" {
		if err := harvestMessages(st, parsed.Session.ID, parsed.Session.TranscriptPath); err != nil {
			fmt.Fprintf(os.Stderr, "clog: harvest: %v\n", err)
		}
	}

	return nil
}

func harvestMessages(st *store.Store, sessionID, transcriptPath string) error {
	offset, err := st.GetOffset(transcriptPath)
	if err != nil {
		return err
	}

	result, err := transcript.Harvest(sessionID, transcriptPath, offset)
	if err != nil {
		return err
	}

	if len(result.Messages) == 0 {
		return nil
	}

	return st.SaveHarvestedMessages(result.Messages, transcriptPath, result.NewOffset)
}

// --- Embed mode ---

func runEmbed(limit int) error {
	st, err := openCurrentProjectStore()
	if err != nil {
		return err
	}
	defer st.Close()

	emb, err := embedding.NewFromEnv()
	if err != nil {
		return err
	}

	if err := st.InitEmbeddingSchema(emb.Dimension()); err != nil {
		return fmt.Errorf("init embedding schema: %w", err)
	}

	messages, err := st.UnembeddedMessages(limit)
	if err != nil {
		return fmt.Errorf("query un-embedded messages: %w", err)
	}

	if len(messages) == 0 {
		fmt.Println("All messages already embedded.")
		return nil
	}

	fmt.Printf("Embedding %d messages...\n", len(messages))

	for i := 0; i < len(messages); i += embeddingBatchSize {
		end := i + embeddingBatchSize
		if end > len(messages) {
			end = len(messages)
		}
		batch := messages[i:end]

		texts := make([]string, len(batch))
		for j, m := range batch {
			texts[j] = m.Content
		}

		embeddings, err := emb.Embed(texts)
		if err != nil {
			return fmt.Errorf("embed batch %d-%d: %w", i, end, err)
		}

		for j, m := range batch {
			if err := st.SaveEmbedding(m.ID, embeddings[j]); err != nil {
				fmt.Fprintf(os.Stderr, "save embedding for message %d: %v\n", m.ID, err)
			}
		}

		fmt.Printf("  %d / %d\n", end, len(messages))
	}

	fmt.Println("Done.")
	return nil
}

// --- Search mode (semantic) ---

func runSearch(query string, limit int) error {
	st, err := openCurrentProjectStore()
	if err != nil {
		return err
	}
	defer st.Close()

	emb, err := embedding.NewFromEnv()
	if err != nil {
		return err
	}

	if err := st.LoadVSS(); err != nil {
		return fmt.Errorf("load vss: %w", err)
	}

	vecs, err := emb.Embed([]string{query})
	if err != nil {
		return fmt.Errorf("embed query: %w", err)
	}

	results, err := st.SearchSimilar(vecs[0], limit)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No results. Run 'clog embed' first to generate embeddings.")
		return nil
	}

	printResults(results)
	return nil
}

// --- Text search mode (ILIKE, no embeddings needed) ---

func runTextSearch(pattern string, limit int) error {
	st, err := openCurrentProjectStore()
	if err != nil {
		return err
	}
	defer st.Close()

	results, err := st.TextSearch(pattern, limit)
	if err != nil {
		return fmt.Errorf("text search: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No results.")
		return nil
	}

	printResults(results)
	return nil
}

// --- Helpers ---

func openCurrentProjectStore() (*store.Store, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get cwd: %w", err)
	}

	cfg := config.Default()
	dbPath := cfg.DBPath(cwd)

	if !fileExists(dbPath) {
		return nil, fmt.Errorf("no database found at %s — run a Claude Code session in this project first", dbPath)
	}

	return store.Open(dbPath)
}

func printResults(results []model.SearchResult) {
	for i, r := range results {
		content := r.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		if r.Score > 0 {
			fmt.Printf("[%d] score=%.4f  %s  [%s]  session=%s\n",
				i+1, r.Score, r.Timestamp.Format("2006-01-02 15:04"), r.Role, r.SessionID[:8])
		} else {
			fmt.Printf("[%d] %s  [%s]  session=%s\n",
				i+1, r.Timestamp.Format("2006-01-02 15:04"), r.Role, r.SessionID[:8])
		}
		fmt.Printf("    %s\n\n", content)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
