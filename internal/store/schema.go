package store

import "fmt"

const coreSchema = `
CREATE SEQUENCE IF NOT EXISTS events_id_seq START 1;
CREATE SEQUENCE IF NOT EXISTS messages_id_seq START 1;

CREATE TABLE IF NOT EXISTS sessions (
    session_id       VARCHAR PRIMARY KEY,
    cwd              VARCHAR NOT NULL,
    transcript_path  VARCHAR,
    created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS events (
    id                     BIGINT DEFAULT nextval('events_id_seq') PRIMARY KEY,
    session_id             VARCHAR NOT NULL,
    event_type             VARCHAR NOT NULL,
    timestamp              TIMESTAMP NOT NULL,
    permission_mode        VARCHAR,
    source                 VARCHAR,
    model                  VARCHAR,
    agent_type             VARCHAR,
    prompt                 VARCHAR,
    tool_name              VARCHAR,
    tool_input             JSON,
    tool_use_id            VARCHAR,
    tool_response          JSON,
    permission_suggestions JSON,
    error                  VARCHAR,
    is_interrupt           BOOLEAN,
    message                VARCHAR,
    title                  VARCHAR,
    notification_type      VARCHAR,
    agent_id               VARCHAR,
    agent_transcript_path  VARCHAR,
    stop_hook_active       BOOLEAN,
    trigger_type           VARCHAR,
    custom_instructions    VARCHAR,
    reason                 VARCHAR
);
CREATE INDEX IF NOT EXISTS idx_events_ts      ON events(timestamp);
CREATE INDEX IF NOT EXISTS idx_events_session ON events(session_id);
CREATE INDEX IF NOT EXISTS idx_events_type    ON events(event_type);

CREATE TABLE IF NOT EXISTS messages (
    id            BIGINT DEFAULT nextval('messages_id_seq') PRIMARY KEY,
    session_id    VARCHAR NOT NULL,
    uuid          VARCHAR UNIQUE,
    parent_uuid   VARCHAR,
    role          VARCHAR NOT NULL,
    content       VARCHAR,
    raw_content   JSON,
    model         VARCHAR,
    timestamp     TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_messages_ts      ON messages(timestamp);
CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);

CREATE TABLE IF NOT EXISTS transcript_offsets (
    transcript_path  VARCHAR PRIMARY KEY,
    last_offset      BIGINT NOT NULL DEFAULT 0
);
`

func embeddingSchema(dimension int) string {
	return fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS message_embeddings (
    message_id BIGINT PRIMARY KEY,
    embedding  FLOAT[%d]
);
`, dimension)
}
