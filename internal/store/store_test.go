package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"clog/internal/model"
)

// --- appendTimeClauses ---

func TestAppendTimeClauses_WhenFilterIsNil_ShouldReturnEmptyStringAndUnchangedParams(t *testing.T) {
	params := []interface{}{"existing"}
	clause, out := appendTimeClauses(nil, "m.timestamp", true, params)

	if clause != "" {
		t.Errorf("expected empty clause, got %q", clause)
	}
	if len(out) != 1 {
		t.Errorf("expected params unchanged (len=1), got len=%d", len(out))
	}
}

func TestAppendTimeClauses_WhenOnlySinceSet_ShouldReturnSingleAndClause(t *testing.T) {
	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	tf := &model.TimeFilter{Since: &since}
	params := []interface{}{"existing"}

	clause, out := appendTimeClauses(tf, "m.timestamp", true, params)

	if clause != " AND m.timestamp >= ?" {
		t.Errorf("expected ' AND m.timestamp >= ?', got %q", clause)
	}
	if len(out) != 2 {
		t.Errorf("expected 2 params, got %d", len(out))
	}
	if !out[1].(time.Time).Equal(since) {
		t.Errorf("expected since param, got %v", out[1])
	}
}

func TestAppendTimeClauses_WhenOnlyUntilSet_ShouldReturnSingleAndClause(t *testing.T) {
	until := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	tf := &model.TimeFilter{Until: &until}
	params := []interface{}{}

	clause, out := appendTimeClauses(tf, "ts", true, params)

	if clause != " AND ts <= ?" {
		t.Errorf("expected ' AND ts <= ?', got %q", clause)
	}
	if len(out) != 1 {
		t.Errorf("expected 1 param, got %d", len(out))
	}
}

func TestAppendTimeClauses_WhenBothSet_ShouldReturnTwoAndClauses(t *testing.T) {
	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	tf := &model.TimeFilter{Since: &since, Until: &until}
	params := []interface{}{"p1"}

	clause, out := appendTimeClauses(tf, "m.timestamp", true, params)

	expected := " AND m.timestamp >= ? AND m.timestamp <= ?"
	if clause != expected {
		t.Errorf("expected %q, got %q", expected, clause)
	}
	if len(out) != 3 {
		t.Errorf("expected 3 params, got %d", len(out))
	}
}

func TestAppendTimeClauses_WhenHasWhereIsFalse_ShouldUseWhereForFirstClause(t *testing.T) {
	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	tf := &model.TimeFilter{Since: &since}

	clause, _ := appendTimeClauses(tf, "m.timestamp", false, nil)

	if clause != " WHERE m.timestamp >= ?" {
		t.Errorf("expected ' WHERE m.timestamp >= ?', got %q", clause)
	}
}

func TestAppendTimeClauses_WhenHasWhereIsFalseAndBothSet_ShouldUseWhereForFirstThenAnd(t *testing.T) {
	since := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	tf := &model.TimeFilter{Since: &since, Until: &until}

	clause, _ := appendTimeClauses(tf, "ts", false, nil)

	expected := " WHERE ts >= ? AND ts <= ?"
	if clause != expected {
		t.Errorf("expected %q, got %q", expected, clause)
	}
}

func TestAppendTimeClauses_WhenFilterHasNoFields_ShouldReturnEmpty(t *testing.T) {
	tf := &model.TimeFilter{} // non-nil but both fields nil
	clause, out := appendTimeClauses(tf, "ts", true, []interface{}{"x"})

	if clause != "" {
		t.Errorf("expected empty clause, got %q", clause)
	}
	if len(out) != 1 {
		t.Errorf("expected params unchanged, got len=%d", len(out))
	}
}

// --- Integration tests with DuckDB ---

// openTestStore creates an in-memory DuckDB store with core schema initialized.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	if err := st.InitCoreSchema(); err != nil {
		t.Fatalf("init core schema: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// seedMessages inserts messages at specific timestamps for testing time filters.
func seedMessages(t *testing.T, st *Store) {
	t.Helper()
	session := model.Session{
		ID:        "test-session",
		CWD:       "/tmp/test",
		CreatedAt: time.Now(),
	}
	if err := st.UpsertSession(session); err != nil {
		t.Fatalf("upsert session: %v", err)
	}

	messages := []model.Message{
		{SessionID: "test-session", UUID: "msg-old", Role: "user", Content: "old message about auth", Timestamp: time.Now().Add(-48 * time.Hour)},
		{SessionID: "test-session", UUID: "msg-recent", Role: "assistant", Content: "recent message about auth", Timestamp: time.Now().Add(-1 * time.Hour)},
		{SessionID: "test-session", UUID: "msg-now", Role: "user", Content: "current message about deploy", Timestamp: time.Now()},
	}
	if err := st.SaveHarvestedMessages(messages, "/tmp/test.jsonl", 999); err != nil {
		t.Fatalf("save messages: %v", err)
	}
}

// seedEvents inserts PostToolUse events at specific timestamps.
func seedEvents(t *testing.T, st *Store) {
	t.Helper()

	toolName := func(s string) *string { return &s }

	events := []model.Event{
		{
			SessionID: "test-session",
			EventType: "PostToolUse",
			Timestamp: time.Now().Add(-48 * time.Hour),
			ToolName:  toolName("Bash"),
			ToolInput: json.RawMessage(`{"command":"echo old"}`),
		},
		{
			SessionID: "test-session",
			EventType: "PostToolUse",
			Timestamp: time.Now().Add(-1 * time.Hour),
			ToolName:  toolName("Bash"),
			ToolInput: json.RawMessage(`{"command":"echo recent"}`),
		},
		{
			SessionID: "test-session",
			EventType: "PostToolUse",
			Timestamp: time.Now(),
			ToolName:  toolName("Read"),
			ToolInput: json.RawMessage(`{"file_path":"/tmp/now.txt"}`),
		},
	}
	for _, e := range events {
		if err := st.InsertEvent(e); err != nil {
			t.Fatalf("insert event: %v", err)
		}
	}
}

// --- TextSearch with TimeFilter ---

func TestTextSearch_WhenNoTimeFilter_ShouldReturnAllMatches(t *testing.T) {
	st := openTestStore(t)
	seedMessages(t, st)

	results, err := st.TextSearch("auth", 10, nil)
	if err != nil {
		t.Fatalf("text search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results matching 'auth', got %d", len(results))
	}
}

func TestTextSearch_WhenSinceFilterSet_ShouldExcludeOlderMessages(t *testing.T) {
	st := openTestStore(t)
	seedMessages(t, st)

	since := time.Now().Add(-6 * time.Hour)
	tf := &model.TimeFilter{Since: &since}

	results, err := st.TextSearch("auth", 10, tf)
	if err != nil {
		t.Fatalf("text search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (only recent), got %d", len(results))
	}
	if len(results) > 0 && results[0].Content != "recent message about auth" {
		t.Errorf("expected recent message, got %q", results[0].Content)
	}
}

func TestTextSearch_WhenUntilFilterSet_ShouldExcludeNewerMessages(t *testing.T) {
	st := openTestStore(t)
	seedMessages(t, st)

	until := time.Now().Add(-6 * time.Hour)
	tf := &model.TimeFilter{Until: &until}

	results, err := st.TextSearch("auth", 10, tf)
	if err != nil {
		t.Fatalf("text search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (only old), got %d", len(results))
	}
	if len(results) > 0 && results[0].Content != "old message about auth" {
		t.Errorf("expected old message, got %q", results[0].Content)
	}
}

func TestTextSearch_WhenBothSinceAndUntilSet_ShouldReturnMessagesInRange(t *testing.T) {
	st := openTestStore(t)
	seedMessages(t, st)

	since := time.Now().Add(-6 * time.Hour)
	until := time.Now().Add(-30 * time.Minute)
	tf := &model.TimeFilter{Since: &since, Until: &until}

	results, err := st.TextSearch("message", 10, tf)
	if err != nil {
		t.Fatalf("text search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result in range, got %d", len(results))
	}
	if len(results) > 0 && results[0].Content != "recent message about auth" {
		t.Errorf("expected recent auth message, got %q", results[0].Content)
	}
}

func TestTextSearch_WhenTimeRangeExcludesAll_ShouldReturnNoResults(t *testing.T) {
	st := openTestStore(t)
	seedMessages(t, st)

	since := time.Now().Add(-100 * time.Hour)
	until := time.Now().Add(-99 * time.Hour)
	tf := &model.TimeFilter{Since: &since, Until: &until}

	results, err := st.TextSearch("auth", 10, tf)
	if err != nil {
		t.Fatalf("text search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// --- ToolSearch with TimeFilter ---

func TestToolSearch_WhenNoTimeFilter_ShouldReturnAllEvents(t *testing.T) {
	st := openTestStore(t)
	seedMessages(t, st)
	seedEvents(t, st)

	results, err := st.ToolSearch("*", 10, nil)
	if err != nil {
		t.Fatalf("tool search: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 tool events, got %d", len(results))
	}
}

func TestToolSearch_WhenSinceFilterSet_ShouldExcludeOlderEvents(t *testing.T) {
	st := openTestStore(t)
	seedMessages(t, st)
	seedEvents(t, st)

	since := time.Now().Add(-6 * time.Hour)
	tf := &model.TimeFilter{Since: &since}

	results, err := st.ToolSearch("*", 10, tf)
	if err != nil {
		t.Fatalf("tool search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 recent tool events, got %d", len(results))
	}
}

func TestToolSearch_WhenFilteredByToolNameWithTimeFilter_ShouldApplyBothFilters(t *testing.T) {
	st := openTestStore(t)
	seedMessages(t, st)
	seedEvents(t, st)

	since := time.Now().Add(-6 * time.Hour)
	tf := &model.TimeFilter{Since: &since}

	results, err := st.ToolSearch("Bash", 10, tf)
	if err != nil {
		t.Fatalf("tool search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 recent Bash event, got %d", len(results))
	}
}

func TestToolSearch_WhenUntilFilterSet_ShouldExcludeNewerEvents(t *testing.T) {
	st := openTestStore(t)
	seedMessages(t, st)
	seedEvents(t, st)

	until := time.Now().Add(-6 * time.Hour)
	tf := &model.TimeFilter{Until: &until}

	results, err := st.ToolSearch("*", 10, tf)
	if err != nil {
		t.Fatalf("tool search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 old tool event, got %d", len(results))
	}
}

func TestToolSearch_WhenTimeRangeExcludesAll_ShouldReturnNoResults(t *testing.T) {
	st := openTestStore(t)
	seedMessages(t, st)
	seedEvents(t, st)

	since := time.Now().Add(-200 * time.Hour)
	until := time.Now().Add(-199 * time.Hour)
	tf := &model.TimeFilter{Since: &since, Until: &until}

	results, err := st.ToolSearch("*", 10, tf)
	if err != nil {
		t.Fatalf("tool search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// --- Open / Close ---

func TestOpen_WhenGivenValidPath_ShouldReturnStore(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.duckdb")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer st.Close()
	if st == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestInitCoreSchema_WhenCalledTwice_ShouldBeIdempotent(t *testing.T) {
	st := openTestStore(t)
	// Schema already initialized in openTestStore; calling again should not error.
	if err := st.InitCoreSchema(); err != nil {
		t.Fatalf("expected idempotent schema init, got error: %v", err)
	}
}

// --- UpsertSession ---

func TestUpsertSession_WhenGivenNewSession_ShouldInsertIt(t *testing.T) {
	st := openTestStore(t)
	session := model.Session{
		ID:        "sess-1",
		CWD:       "/tmp/project",
		CreatedAt: time.Now(),
	}
	if err := st.UpsertSession(session); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify by querying directly.
	var id string
	err := st.db.QueryRow("SELECT session_id FROM sessions WHERE session_id = ?", "sess-1").Scan(&id)
	if err != nil {
		t.Fatalf("query session: %v", err)
	}
	if id != "sess-1" {
		t.Errorf("expected session_id 'sess-1', got %q", id)
	}
}

func TestUpsertSession_WhenSessionAlreadyExists_ShouldUpdateTranscriptPath(t *testing.T) {
	st := openTestStore(t)
	session := model.Session{
		ID:             "sess-1",
		CWD:            "/tmp/project",
		TranscriptPath: "/old/path.jsonl",
		CreatedAt:      time.Now(),
	}
	if err := st.UpsertSession(session); err != nil {
		t.Fatalf("insert: %v", err)
	}

	session.TranscriptPath = "/new/path.jsonl"
	if err := st.UpsertSession(session); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var path *string
	err := st.db.QueryRow("SELECT transcript_path FROM sessions WHERE session_id = ?", "sess-1").Scan(&path)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if path == nil || *path != "/new/path.jsonl" {
		t.Errorf("expected updated transcript_path, got %v", path)
	}
}

func TestUpsertSession_WhenTranscriptPathEmpty_ShouldStoreNull(t *testing.T) {
	st := openTestStore(t)
	session := model.Session{
		ID:        "sess-null",
		CWD:       "/tmp",
		CreatedAt: time.Now(),
	}
	if err := st.UpsertSession(session); err != nil {
		t.Fatalf("insert: %v", err)
	}

	var path *string
	err := st.db.QueryRow("SELECT transcript_path FROM sessions WHERE session_id = ?", "sess-null").Scan(&path)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if path != nil {
		t.Errorf("expected NULL transcript_path, got %v", *path)
	}
}

// --- InsertEvent ---

func TestInsertEvent_WhenGivenMinimalEvent_ShouldPersistIt(t *testing.T) {
	st := openTestStore(t)
	st.UpsertSession(model.Session{ID: "sess-1", CWD: "/tmp", CreatedAt: time.Now()})

	event := model.Event{
		SessionID: "sess-1",
		EventType: "Stop",
		Timestamp: time.Now(),
	}
	if err := st.InsertEvent(event); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	var count int
	st.db.QueryRow("SELECT COUNT(*) FROM events WHERE session_id = 'sess-1'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 event, got %d", count)
	}
}

func TestInsertEvent_WhenGivenToolEvent_ShouldPersistToolFields(t *testing.T) {
	st := openTestStore(t)
	st.UpsertSession(model.Session{ID: "sess-1", CWD: "/tmp", CreatedAt: time.Now()})

	toolName := "Bash"
	event := model.Event{
		SessionID: "sess-1",
		EventType: "PostToolUse",
		Timestamp: time.Now(),
		ToolName:  &toolName,
		ToolInput: json.RawMessage(`{"command":"ls"}`),
	}
	if err := st.InsertEvent(event); err != nil {
		t.Fatalf("insert event: %v", err)
	}

	var name *string
	st.db.QueryRow("SELECT tool_name FROM events WHERE session_id = 'sess-1'").Scan(&name)
	if name == nil || *name != "Bash" {
		t.Errorf("expected tool_name 'Bash', got %v", name)
	}
}

func TestInsertEvent_WhenMultipleEvents_ShouldAssignDistinctIDs(t *testing.T) {
	st := openTestStore(t)
	st.UpsertSession(model.Session{ID: "sess-1", CWD: "/tmp", CreatedAt: time.Now()})

	for i := 0; i < 3; i++ {
		st.InsertEvent(model.Event{SessionID: "sess-1", EventType: "Stop", Timestamp: time.Now()})
	}

	var count int
	st.db.QueryRow("SELECT COUNT(DISTINCT id) FROM events WHERE session_id = 'sess-1'").Scan(&count)
	if count != 3 {
		t.Errorf("expected 3 distinct events, got %d", count)
	}
}

// --- SaveHarvestedMessages / GetOffset ---

func TestSaveHarvestedMessages_WhenGivenMessages_ShouldPersistThem(t *testing.T) {
	st := openTestStore(t)
	st.UpsertSession(model.Session{ID: "sess-1", CWD: "/tmp", CreatedAt: time.Now()})

	messages := []model.Message{
		{SessionID: "sess-1", UUID: "m1", Role: "user", Content: "hello", Timestamp: time.Now()},
		{SessionID: "sess-1", UUID: "m2", Role: "assistant", Content: "hi", Timestamp: time.Now()},
	}
	if err := st.SaveHarvestedMessages(messages, "/path.jsonl", 100); err != nil {
		t.Fatalf("save: %v", err)
	}

	var count int
	st.db.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id = 'sess-1'").Scan(&count)
	if count != 2 {
		t.Errorf("expected 2 messages, got %d", count)
	}
}

func TestSaveHarvestedMessages_WhenDuplicateUUID_ShouldNotCreateDuplicate(t *testing.T) {
	st := openTestStore(t)
	st.UpsertSession(model.Session{ID: "sess-1", CWD: "/tmp", CreatedAt: time.Now()})

	msg := model.Message{SessionID: "sess-1", UUID: "dup-1", Role: "user", Content: "hello", Timestamp: time.Now()}

	st.SaveHarvestedMessages([]model.Message{msg}, "/path.jsonl", 50)
	st.SaveHarvestedMessages([]model.Message{msg}, "/path.jsonl", 100)

	var count int
	st.db.QueryRow("SELECT COUNT(*) FROM messages WHERE uuid = 'dup-1'").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 message (deduped), got %d", count)
	}
}

func TestSaveHarvestedMessages_ShouldUpdateTranscriptOffset(t *testing.T) {
	st := openTestStore(t)
	st.UpsertSession(model.Session{ID: "sess-1", CWD: "/tmp", CreatedAt: time.Now()})

	msg := model.Message{SessionID: "sess-1", UUID: "m1", Role: "user", Content: "hi", Timestamp: time.Now()}
	st.SaveHarvestedMessages([]model.Message{msg}, "/path.jsonl", 500)

	offset, err := st.GetOffset("/path.jsonl")
	if err != nil {
		t.Fatalf("get offset: %v", err)
	}
	if offset != 500 {
		t.Errorf("expected offset 500, got %d", offset)
	}
}

func TestGetOffset_WhenPathNotFound_ShouldReturnZero(t *testing.T) {
	st := openTestStore(t)

	offset, err := st.GetOffset("/nonexistent.jsonl")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if offset != 0 {
		t.Errorf("expected offset 0, got %d", offset)
	}
}

func TestGetOffset_WhenOffsetUpdated_ShouldReturnLatestValue(t *testing.T) {
	st := openTestStore(t)
	st.UpsertSession(model.Session{ID: "sess-1", CWD: "/tmp", CreatedAt: time.Now()})

	msg1 := model.Message{SessionID: "sess-1", UUID: "m1", Role: "user", Content: "a", Timestamp: time.Now()}
	msg2 := model.Message{SessionID: "sess-1", UUID: "m2", Role: "user", Content: "b", Timestamp: time.Now()}

	st.SaveHarvestedMessages([]model.Message{msg1}, "/path.jsonl", 100)
	st.SaveHarvestedMessages([]model.Message{msg2}, "/path.jsonl", 200)

	offset, err := st.GetOffset("/path.jsonl")
	if err != nil {
		t.Fatalf("get offset: %v", err)
	}
	if offset != 200 {
		t.Errorf("expected latest offset 200, got %d", offset)
	}
}

// --- TextSearch without time filter ---

func TestTextSearch_WhenPatternMatchesSubstring_ShouldReturnResults(t *testing.T) {
	st := openTestStore(t)
	st.UpsertSession(model.Session{ID: "sess-1", CWD: "/tmp", CreatedAt: time.Now()})
	st.SaveHarvestedMessages([]model.Message{
		{SessionID: "sess-1", UUID: "m1", Role: "user", Content: "fix the authentication bug", Timestamp: time.Now()},
		{SessionID: "sess-1", UUID: "m2", Role: "assistant", Content: "I'll look at the auth module", Timestamp: time.Now()},
		{SessionID: "sess-1", UUID: "m3", Role: "user", Content: "deploy to production", Timestamp: time.Now()},
	}, "/test.jsonl", 999)

	results, err := st.TextSearch("auth", 10, nil)
	if err != nil {
		t.Fatalf("text search: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestTextSearch_WhenPatternIsCaseInsensitive_ShouldMatchRegardlessOfCase(t *testing.T) {
	st := openTestStore(t)
	st.UpsertSession(model.Session{ID: "sess-1", CWD: "/tmp", CreatedAt: time.Now()})
	st.SaveHarvestedMessages([]model.Message{
		{SessionID: "sess-1", UUID: "m1", Role: "user", Content: "Hello World", Timestamp: time.Now()},
	}, "/test.jsonl", 999)

	results, err := st.TextSearch("hello", 10, nil)
	if err != nil {
		t.Fatalf("text search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (case-insensitive), got %d", len(results))
	}
}

func TestTextSearch_WhenNoMatches_ShouldReturnEmptySlice(t *testing.T) {
	st := openTestStore(t)
	st.UpsertSession(model.Session{ID: "sess-1", CWD: "/tmp", CreatedAt: time.Now()})
	st.SaveHarvestedMessages([]model.Message{
		{SessionID: "sess-1", UUID: "m1", Role: "user", Content: "hello", Timestamp: time.Now()},
	}, "/test.jsonl", 999)

	results, err := st.TextSearch("zzzznotfound", 10, nil)
	if err != nil {
		t.Fatalf("text search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestTextSearch_ShouldRespectLimit(t *testing.T) {
	st := openTestStore(t)
	st.UpsertSession(model.Session{ID: "sess-1", CWD: "/tmp", CreatedAt: time.Now()})
	var msgs []model.Message
	for i := 0; i < 10; i++ {
		msgs = append(msgs, model.Message{
			SessionID: "sess-1",
			UUID:      fmt.Sprintf("m%d", i),
			Role:      "user",
			Content:   "matching content",
			Timestamp: time.Now(),
		})
	}
	st.SaveHarvestedMessages(msgs, "/test.jsonl", 999)

	results, err := st.TextSearch("matching", 3, nil)
	if err != nil {
		t.Fatalf("text search: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results (limited), got %d", len(results))
	}
}

// --- ToolSearch without time filter ---

func TestToolSearch_WhenWildcard_ShouldReturnAllToolEvents(t *testing.T) {
	st := openTestStore(t)
	st.UpsertSession(model.Session{ID: "sess-1", CWD: "/tmp", CreatedAt: time.Now()})
	seedEvents(t, st)

	results, err := st.ToolSearch("*", 10, nil)
	if err != nil {
		t.Fatalf("tool search: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 events, got %d", len(results))
	}
}

func TestToolSearch_WhenFilteredByName_ShouldReturnOnlyMatching(t *testing.T) {
	st := openTestStore(t)
	st.UpsertSession(model.Session{ID: "sess-1", CWD: "/tmp", CreatedAt: time.Now()})
	seedEvents(t, st)

	results, err := st.ToolSearch("Read", 10, nil)
	if err != nil {
		t.Fatalf("tool search: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 Read event, got %d", len(results))
	}
	if len(results) > 0 && results[0].ToolName != "Read" {
		t.Errorf("expected tool name 'Read', got %q", results[0].ToolName)
	}
}

func TestToolSearch_WhenEmptyPattern_ShouldReturnAll(t *testing.T) {
	st := openTestStore(t)
	st.UpsertSession(model.Session{ID: "sess-1", CWD: "/tmp", CreatedAt: time.Now()})
	seedEvents(t, st)

	results, err := st.ToolSearch("", 10, nil)
	if err != nil {
		t.Fatalf("tool search: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 events for empty pattern, got %d", len(results))
	}
}

func TestToolSearch_ShouldIncludeToolInputAndResponse(t *testing.T) {
	st := openTestStore(t)
	st.UpsertSession(model.Session{ID: "sess-1", CWD: "/tmp", CreatedAt: time.Now()})

	toolName := "Bash"
	event := model.Event{
		SessionID:    "sess-1",
		EventType:    "PostToolUse",
		Timestamp:    time.Now(),
		ToolName:     &toolName,
		ToolInput:    json.RawMessage(`{"command":"echo hi"}`),
		ToolResponse: json.RawMessage(`{"output":"hi"}`),
	}
	st.InsertEvent(event)

	results, err := st.ToolSearch("Bash", 10, nil)
	if err != nil {
		t.Fatalf("tool search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ToolInput == "" {
		t.Error("expected non-empty tool input")
	}
	if results[0].ToolResponse == "" {
		t.Error("expected non-empty tool response")
	}
}

// --- helpers ---

func TestNullStr_WhenGivenEmptyString_ShouldReturnNil(t *testing.T) {
	got := nullStr("")
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestNullStr_WhenGivenNonEmptyString_ShouldReturnString(t *testing.T) {
	got := nullStr("hello")
	if got != "hello" {
		t.Errorf("expected 'hello', got %v", got)
	}
}

func TestRawJSON_WhenGivenNil_ShouldReturnNil(t *testing.T) {
	got := rawJSON(nil)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestRawJSON_WhenGivenData_ShouldReturnString(t *testing.T) {
	got := rawJSON(json.RawMessage(`{"key":"value"}`))
	if got != `{"key":"value"}` {
		t.Errorf("expected JSON string, got %v", got)
	}
}

func TestFormatFloatArray_WhenGivenValues_ShouldReturnBracketedString(t *testing.T) {
	got := formatFloatArray([]float32{0.1, 0.2, 0.3})
	if got != "[0.1,0.2,0.3]" {
		t.Errorf("expected '[0.1,0.2,0.3]', got %q", got)
	}
}

func TestFormatFloatArray_WhenGivenSingleValue_ShouldReturnSingleElement(t *testing.T) {
	got := formatFloatArray([]float32{1.5})
	if got != "[1.5]" {
		t.Errorf("expected '[1.5]', got %q", got)
	}
}

func TestFormatFloatArray_WhenGivenEmptySlice_ShouldReturnEmptyBrackets(t *testing.T) {
	got := formatFloatArray([]float32{})
	if got != "[]" {
		t.Errorf("expected '[]', got %q", got)
	}
}

// --- Ensure unused import doesn't fail build ---

var _ = os.DevNull
