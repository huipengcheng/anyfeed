package store

// migrations contains database migration statements.
var migrations = []string{
	// Migration 1: Initial schema
	`CREATE TABLE IF NOT EXISTS entries (
		id          TEXT PRIMARY KEY,
		source_name TEXT NOT NULL,
		source_type TEXT NOT NULL,
		title       TEXT NOT NULL,
		content     TEXT,
		url         TEXT,
		author      TEXT,
		published_at DATETIME,
		received_at DATETIME NOT NULL,
		metadata    TEXT,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,

	`CREATE INDEX IF NOT EXISTS idx_entries_source_name ON entries(source_name)`,
	`CREATE INDEX IF NOT EXISTS idx_entries_published_at ON entries(published_at DESC)`,
	`CREATE INDEX IF NOT EXISTS idx_entries_received_at ON entries(received_at DESC)`,

	// Source tags table
	`CREATE TABLE IF NOT EXISTS source_tags (
		source_name TEXT NOT NULL,
		tag         TEXT NOT NULL,
		PRIMARY KEY (source_name, tag)
	)`,

	`CREATE INDEX IF NOT EXISTS idx_source_tags_tag ON source_tags(tag)`,

	// Web source state for change detection
	`CREATE TABLE IF NOT EXISTS web_state (
		source_name  TEXT PRIMARY KEY,
		last_hash    TEXT,
		last_content TEXT,
		checked_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,

	// Entry tags table (for filtering entries by tags)
	`CREATE TABLE IF NOT EXISTS entry_tags (
		entry_id TEXT NOT NULL,
		tag      TEXT NOT NULL,
		PRIMARY KEY (entry_id, tag),
		FOREIGN KEY (entry_id) REFERENCES entries(id) ON DELETE CASCADE
	)`,

	`CREATE INDEX IF NOT EXISTS idx_entry_tags_tag ON entry_tags(tag)`,

	// Schema version tracking
	`CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY
	)`,
}
