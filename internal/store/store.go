// Package store manages all DuckDB persistence operations.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"clog/internal/model"

	_ "github.com/duckdb/duckdb-go/v2"
)

// Store wraps a DuckDB connection and exposes domain-specific persistence.
type Store struct {
	db *sql.DB
}

// Open creates a new Store connected to the given DuckDB file.
func Open(dbPath string) (*Store, error) {
	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open duckdb %s: %w", dbPath, err)
	}
	return &Store{db: db}, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// InitCoreSchema creates the base tables and indexes if they don't exist.
func (s *Store) InitCoreSchema() error {
	_, err := s.db.Exec(coreSchema)
	if err != nil {
		return fmt.Errorf("init core schema: %w", err)
	}
	return nil
}

// InitEmbeddingSchema installs the vss extension and creates the embeddings table.
func (s *Store) InitEmbeddingSchema(dimension int) error {
	if _, err := s.db.Exec("INSTALL vss"); err != nil {
		return fmt.Errorf("install vss extension: %w", err)
	}
	if _, err := s.db.Exec("LOAD vss"); err != nil {
		return fmt.Errorf("load vss extension: %w", err)
	}
	if _, err := s.db.Exec(embeddingSchema(dimension)); err != nil {
		return fmt.Errorf("create embedding table: %w", err)
	}
	return nil
}

// LoadVSS loads the vss extension for the current connection (required for search).
func (s *Store) LoadVSS() error {
	_, err := s.db.Exec("LOAD vss")
	return err
}

// --- Session operations ---

// UpsertSession inserts or updates a session record.
func (s *Store) UpsertSession(session model.Session) error {
	_, err := s.db.Exec(`
		INSERT INTO sessions (session_id, cwd, transcript_path, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (session_id) DO UPDATE SET transcript_path = excluded.transcript_path
	`, session.ID, session.CWD, nullStr(session.TranscriptPath), session.CreatedAt)
	return err
}

// --- Event operations ---

// InsertEvent persists a hook event.
func (s *Store) InsertEvent(e model.Event) error {
	_, err := s.db.Exec(`
		INSERT INTO events (
			session_id, event_type, timestamp, permission_mode,
			source, model, agent_type, prompt,
			tool_name, tool_input, tool_use_id, tool_response,
			permission_suggestions, error, is_interrupt,
			message, title, notification_type,
			agent_id, agent_transcript_path, stop_hook_active,
			trigger_type, custom_instructions, reason
		) VALUES (
			?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?, ?,
			?, ?, ?,
			?, ?, ?,
			?, ?, ?,
			?, ?, ?
		)`,
		e.SessionID, e.EventType, e.Timestamp, e.PermissionMode,
		e.Source, e.Model, e.AgentType, e.Prompt,
		e.ToolName, rawJSON(e.ToolInput), e.ToolUseID, rawJSON(e.ToolResponse),
		rawJSON(e.PermissionSuggestions), e.Error, e.IsInterrupt,
		e.Message, e.Title, e.NotificationType,
		e.AgentID, e.AgentTranscriptPath, e.StopHookActive,
		e.TriggerType, e.CustomInstructions, e.Reason,
	)
	return err
}

// --- Message operations ---

// SaveHarvestedMessages inserts messages and updates the transcript offset atomically.
func (s *Store) SaveHarvestedMessages(messages []model.Message, transcriptPath string, newOffset int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO messages (session_id, uuid, parent_uuid, role, content, raw_content, model, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (uuid) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	for _, m := range messages {
		if _, err := stmt.Exec(
			m.SessionID,
			nullStr(m.UUID),
			nullStr(m.ParentUUID),
			m.Role,
			nullStr(m.Content),
			nullStr(m.RawContent),
			nullStr(m.Model),
			m.Timestamp,
		); err != nil {
			return fmt.Errorf("insert message %s: %w", m.UUID, err)
		}
	}

	if _, err := tx.Exec(`
		INSERT INTO transcript_offsets (transcript_path, last_offset)
		VALUES (?, ?)
		ON CONFLICT (transcript_path) DO UPDATE SET last_offset = excluded.last_offset
	`, transcriptPath, newOffset); err != nil {
		return fmt.Errorf("update offset: %w", err)
	}

	return tx.Commit()
}

// GetOffset returns the last read offset for a transcript file.
func (s *Store) GetOffset(path string) (int64, error) {
	var offset int64
	err := s.db.QueryRow(
		`SELECT last_offset FROM transcript_offsets WHERE transcript_path = ?`, path,
	).Scan(&offset)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return offset, err
}

// --- Embedding operations ---

// UnembeddedMessages returns messages that lack embeddings.
func (s *Store) UnembeddedMessages(limit int) ([]model.StoredMessage, error) {
	rows, err := s.db.Query(`
		SELECT m.id, m.session_id, m.role, m.content, m.timestamp
		FROM messages m
		LEFT JOIN message_embeddings e ON m.id = e.message_id
		WHERE e.message_id IS NULL
		  AND m.content IS NOT NULL
		  AND m.content != ''
		ORDER BY m.id
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.StoredMessage
	for rows.Next() {
		var m model.StoredMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SaveEmbedding persists a single message embedding.
func (s *Store) SaveEmbedding(messageID int64, embedding []float32) error {
	query := fmt.Sprintf(
		`INSERT INTO message_embeddings (message_id, embedding) VALUES (?, %s::FLOAT[%d])
		 ON CONFLICT DO NOTHING`,
		formatFloatArray(embedding), len(embedding),
	)
	_, err := s.db.Exec(query, messageID)
	return err
}

// SearchSimilar finds the top-k messages most similar to the given embedding.
func (s *Store) SearchSimilar(embedding []float32, limit int, tf *model.TimeFilter) ([]model.SearchResult, error) {
	params := []interface{}{}
	timeClause, params := appendTimeClauses(tf, "m.timestamp", false, params)

	query := fmt.Sprintf(`
		SELECT m.id, m.session_id, m.role, m.content,
		       array_cosine_similarity(e.embedding, %s::FLOAT[%d]) AS score,
		       m.timestamp
		FROM messages m
		JOIN message_embeddings e ON m.id = e.message_id
		%s
		ORDER BY score DESC
		LIMIT ?
	`, formatFloatArray(embedding), len(embedding), timeClause)

	params = append(params, limit)
	rows, err := s.db.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.SearchResult
	for rows.Next() {
		var r model.SearchResult
		if err := rows.Scan(&r.ID, &r.SessionID, &r.Role, &r.Content, &r.Score, &r.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// TextSearch performs a case-insensitive text search across messages.
func (s *Store) TextSearch(pattern string, limit int, tf *model.TimeFilter) ([]model.SearchResult, error) {
	params := []interface{}{"%"+pattern+"%"}
	timeClause, params := appendTimeClauses(tf, "m.timestamp", true, params)

	query := fmt.Sprintf(`
		SELECT m.id, m.session_id, m.role, m.content, 0.0 AS score, m.timestamp
		FROM messages m
		WHERE m.content ILIKE ?
		%s
		ORDER BY m.timestamp DESC
		LIMIT ?
	`, timeClause)

	params = append(params, limit)
	rows, err := s.db.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.SearchResult
	for rows.Next() {
		var r model.SearchResult
		if err := rows.Scan(&r.ID, &r.SessionID, &r.Role, &r.Content, &r.Score, &r.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- Tool search ---

// ToolSearch queries PostToolUse events, optionally filtered by tool name.
func (s *Store) ToolSearch(toolName string, limit int, tf *model.TimeFilter) ([]model.ToolResult, error) {
	var query string
	var params []interface{}

	if toolName == "" || toolName == "*" {
		params = []interface{}{}
		timeClause, p := appendTimeClauses(tf, "timestamp", true, params)
		params = p
		query = fmt.Sprintf(`
			SELECT session_id, tool_name, CAST(tool_input AS VARCHAR),
			       CAST(tool_response AS VARCHAR), timestamp
			FROM events
			WHERE event_type = 'PostToolUse'
			  AND tool_name IS NOT NULL
			%s
			ORDER BY timestamp DESC
			LIMIT ?
		`, timeClause)
	} else {
		params = []interface{}{toolName}
		timeClause, p := appendTimeClauses(tf, "timestamp", true, params)
		params = p
		query = fmt.Sprintf(`
			SELECT session_id, tool_name, CAST(tool_input AS VARCHAR),
			       CAST(tool_response AS VARCHAR), timestamp
			FROM events
			WHERE event_type = 'PostToolUse'
			  AND tool_name ILIKE '%%' || ? || '%%'
			%s
			ORDER BY timestamp DESC
			LIMIT ?
		`, timeClause)
	}

	params = append(params, limit)
	rows, err := s.db.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.ToolResult
	for rows.Next() {
		var r model.ToolResult
		var toolInput, toolResponse sql.NullString
		if err := rows.Scan(&r.SessionID, &r.ToolName, &toolInput, &toolResponse, &r.Timestamp); err != nil {
			return nil, err
		}
		if toolInput.Valid {
			r.ToolInput = toolInput.String
		}
		if toolResponse.Valid {
			r.ToolResponse = toolResponse.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- Session message retrieval ---

// SessionMessages returns messages for a session in chronological order.
func (s *Store) SessionMessages(sessionID string, limit int) ([]model.StoredMessage, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, role, content, timestamp
		FROM messages
		WHERE session_id = ? AND content IS NOT NULL AND content != ''
		ORDER BY timestamp ASC
		LIMIT ?
	`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.StoredMessage
	for rows.Next() {
		var m model.StoredMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// --- Summary operations ---

// SaveSummary persists or updates a session summary.
func (s *Store) SaveSummary(sessionID, summary, modelName string) error {
	_, err := s.db.Exec(`
		INSERT INTO session_summaries (session_id, summary, model, generated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (session_id) DO UPDATE
		SET summary = excluded.summary, model = excluded.model, generated_at = excluded.generated_at
	`, sessionID, summary, modelName, time.Now().UTC())
	return err
}

// ListSummaries returns session summaries ordered by generation time.
func (s *Store) ListSummaries(limit int, tf *model.TimeFilter) ([]model.SummaryResult, error) {
	params := []interface{}{}
	timeClause, params := appendTimeClauses(tf, "ss.generated_at", false, params)

	query := fmt.Sprintf(`
		SELECT ss.session_id, ss.summary, ss.model, ss.generated_at, s.cwd
		FROM session_summaries ss
		JOIN sessions s ON ss.session_id = s.session_id
		%s
		ORDER BY ss.generated_at DESC
		LIMIT ?
	`, timeClause)

	params = append(params, limit)
	rows, err := s.db.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.SummaryResult
	for rows.Next() {
		var r model.SummaryResult
		if err := rows.Scan(&r.SessionID, &r.Summary, &r.Model, &r.GeneratedAt, &r.CWD); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- helpers ---

// appendTimeClauses builds SQL fragments for time filtering.
// If hasWhere is true, clauses use "AND"; otherwise the first clause uses "WHERE".
func appendTimeClauses(tf *model.TimeFilter, tsCol string, hasWhere bool, params []interface{}) (string, []interface{}) {
	if tf == nil {
		return "", params
	}

	var clauses []string
	if tf.Since != nil {
		clauses = append(clauses, fmt.Sprintf("%s >= ?", tsCol))
		params = append(params, *tf.Since)
	}
	if tf.Until != nil {
		clauses = append(clauses, fmt.Sprintf("%s <= ?", tsCol))
		params = append(params, *tf.Until)
	}

	if len(clauses) == 0 {
		return "", params
	}

	var sb strings.Builder
	for i, c := range clauses {
		if i == 0 && !hasWhere {
			sb.WriteString(" WHERE ")
		} else {
			sb.WriteString(" AND ")
		}
		sb.WriteString(c)
	}
	return sb.String(), params
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func rawJSON(r json.RawMessage) interface{} {
	if r == nil {
		return nil
	}
	return string(r)
}

func formatFloatArray(v []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'g', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}
