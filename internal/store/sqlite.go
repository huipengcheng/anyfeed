package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/huipeng/anyfeed/internal/source"
	_ "modernc.org/sqlite"
)

// SQLiteStore implements Store using SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLite store.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return store, nil
}

// migrate runs database migrations.
func (s *SQLiteStore) migrate() error {
	for _, migration := range migrations {
		if _, err := s.db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w (sql: %s)", err, migration)
		}
	}
	slog.Info("database migrations completed")
	return nil
}

// SaveEntries saves entries to storage (deduplicates by ID).
func (s *SQLiteStore) SaveEntries(ctx context.Context, entries []*source.Entry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	entryStmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO entries (
			id, source_name, source_type, title, content, url, author, 
			published_at, received_at, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare entry statement: %w", err)
	}
	defer entryStmt.Close()

	tagStmt, err := tx.PrepareContext(ctx, `
		INSERT OR IGNORE INTO entry_tags (entry_id, tag) VALUES (?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare tag statement: %w", err)
	}
	defer tagStmt.Close()

	for _, entry := range entries {
		metadata, err := json.Marshal(entry.Metadata)
		if err != nil {
			metadata = []byte("{}")
		}

		_, err = entryStmt.ExecContext(ctx,
			entry.ID,
			entry.SourceName,
			entry.SourceType,
			entry.Title,
			entry.Content,
			entry.URL,
			entry.Author,
			entry.PublishedAt.UTC(),
			entry.ReceivedAt.UTC(),
			string(metadata),
		)
		if err != nil {
			return fmt.Errorf("failed to insert entry: %w", err)
		}

		// Insert tags
		for _, tag := range entry.Tags {
			if _, err := tagStmt.ExecContext(ctx, entry.ID, tag); err != nil {
				slog.Warn("failed to insert tag", "entry_id", entry.ID, "tag", tag, "error", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	slog.Debug("saved entries", "count", len(entries))
	return nil
}

// GetEntries retrieves entries with optional filters.
func (s *SQLiteStore) GetEntries(ctx context.Context, opts QueryOptions) ([]*source.Entry, error) {
	query := `
		SELECT DISTINCT e.id, e.source_name, e.source_type, e.title, e.content, 
		       e.url, e.author, e.published_at, e.received_at, e.metadata
		FROM entries e
	`

	var conditions []string
	var args []interface{}

	// Join with tags if filtering by tags
	if len(opts.Tags) > 0 {
		query += ` JOIN entry_tags et ON e.id = et.entry_id`
		placeholders := make([]string, len(opts.Tags))
		for i, tag := range opts.Tags {
			placeholders[i] = "?"
			args = append(args, tag)
		}
		conditions = append(conditions, fmt.Sprintf("et.tag IN (%s)", strings.Join(placeholders, ",")))
	}

	// Filter by source names
	if len(opts.SourceNames) > 0 {
		placeholders := make([]string, len(opts.SourceNames))
		for i, name := range opts.SourceNames {
			placeholders[i] = "?"
			args = append(args, name)
		}
		conditions = append(conditions, fmt.Sprintf("e.source_name IN (%s)", strings.Join(placeholders, ",")))
	}

	// Filter by time
	if !opts.Since.IsZero() {
		conditions = append(conditions, "e.published_at > ?")
		args = append(args, opts.Since.UTC())
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY e.published_at DESC"

	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	}

	if opts.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, opts.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query entries: %w", err)
	}
	defer rows.Close()

	var entries []*source.Entry
	for rows.Next() {
		entry, err := s.scanEntry(rows)
		if err != nil {
			return nil, err
		}

		// Load tags for entry
		tags, err := s.getEntryTags(ctx, entry.ID)
		if err != nil {
			slog.Warn("failed to load tags for entry", "entry_id", entry.ID, "error", err)
		}
		entry.Tags = tags

		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating entries: %w", err)
	}

	return entries, nil
}

// scanEntry scans a single entry from a database row.
func (s *SQLiteStore) scanEntry(rows *sql.Rows) (*source.Entry, error) {
	var entry source.Entry
	var publishedAt, receivedAt time.Time
	var metadataStr string
	var content, url, author sql.NullString

	if err := rows.Scan(
		&entry.ID,
		&entry.SourceName,
		&entry.SourceType,
		&entry.Title,
		&content,
		&url,
		&author,
		&publishedAt,
		&receivedAt,
		&metadataStr,
	); err != nil {
		return nil, fmt.Errorf("failed to scan entry: %w", err)
	}

	entry.Content = content.String
	entry.URL = url.String
	entry.Author = author.String
	entry.PublishedAt = publishedAt
	entry.ReceivedAt = receivedAt

	if metadataStr != "" {
		if err := json.Unmarshal([]byte(metadataStr), &entry.Metadata); err != nil {
			entry.Metadata = make(map[string]string)
		}
	} else {
		entry.Metadata = make(map[string]string)
	}

	return &entry, nil
}

// getEntryTags retrieves tags for an entry.
func (s *SQLiteStore) getEntryTags(ctx context.Context, entryID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT tag FROM entry_tags WHERE entry_id = ?", entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// GetEntryByID retrieves a single entry by ID.
func (s *SQLiteStore) GetEntryByID(ctx context.Context, id string) (*source.Entry, error) {
	query := `
		SELECT id, source_name, source_type, title, content, url, author, 
		       published_at, received_at, metadata
		FROM entries WHERE id = ?
	`

	rows, err := s.db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query entry: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}

	entry, err := s.scanEntry(rows)
	if err != nil {
		return nil, err
	}

	tags, err := s.getEntryTags(ctx, entry.ID)
	if err != nil {
		slog.Warn("failed to load tags for entry", "entry_id", entry.ID, "error", err)
	}
	entry.Tags = tags

	return entry, nil
}

// DeleteOldEntries removes entries beyond retention limit.
func (s *SQLiteStore) DeleteOldEntries(ctx context.Context, sourceName string, keepCount int) error {
	// Get IDs of entries to keep
	query := `
		DELETE FROM entries
		WHERE source_name = ? AND id NOT IN (
			SELECT id FROM entries
			WHERE source_name = ?
			ORDER BY published_at DESC
			LIMIT ?
		)
	`

	result, err := s.db.ExecContext(ctx, query, sourceName, sourceName, keepCount)
	if err != nil {
		return fmt.Errorf("failed to delete old entries: %w", err)
	}

	deleted, _ := result.RowsAffected()
	if deleted > 0 {
		slog.Info("deleted old entries", "source", sourceName, "count", deleted)
	}

	return nil
}

// GetEntriesCount returns the total count of entries for a source.
func (s *SQLiteStore) GetEntriesCount(ctx context.Context, sourceName string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM entries WHERE source_name = ?", sourceName).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count entries: %w", err)
	}
	return count, nil
}

// GetAllSourcesCount returns the count of entries for all sources.
func (s *SQLiteStore) GetAllSourcesCount(ctx context.Context) (map[string]int, error) {
	query := `SELECT source_name, COUNT(*) FROM entries GROUP BY source_name`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to count entries by source: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, fmt.Errorf("failed to scan count: %w", err)
		}
		counts[name] = count
	}
	return counts, rows.Err()
}

// SaveWebState saves the web source state.
func (s *SQLiteStore) SaveWebState(ctx context.Context, sourceName, hash, content string) error {
	query := `
		INSERT INTO web_state (source_name, last_hash, last_content, checked_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(source_name) DO UPDATE SET
			last_hash = excluded.last_hash,
			last_content = excluded.last_content,
			checked_at = excluded.checked_at
	`
	_, err := s.db.ExecContext(ctx, query, sourceName, hash, content, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to save web state: %w", err)
	}
	return nil
}

// GetWebState retrieves the web source state.
func (s *SQLiteStore) GetWebState(ctx context.Context, sourceName string) (hash string, content string, err error) {
	query := `SELECT last_hash, last_content FROM web_state WHERE source_name = ?`
	var hashNull, contentNull sql.NullString
	err = s.db.QueryRowContext(ctx, query, sourceName).Scan(&hashNull, &contentNull)
	if err == sql.ErrNoRows {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("failed to get web state: %w", err)
	}
	return hashNull.String, contentNull.String, nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}
