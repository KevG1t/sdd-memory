package store

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// obsColumns is the canonical column projection for observation reads. Keep it
// in sync with scanObservation. Order: 17 columns.
const obsColumns = `id, sync_id, session_id, type, title, project, scope, topic_key, content, tool_name, normalized_hash, revision_count, duplicate_count, last_seen_at, deleted_at, created_at, updated_at`

// dedupeWindow is the look-back period for hash-based dedup. An observation
// with the same normalized_hash + project + scope + type + title created within
// this window is treated as a duplicate rather than a new record.
// default (15 minutes). Kept as a package-level constant
// because sdd-memory has no Config struct and adding one is deferred.
const dedupeWindow = 15 * time.Minute

// dedupeWindowArg returns the SQLite datetime modifier for the dedup window.
func dedupeWindowArg() string {
	return fmt.Sprintf("-%d minutes", int(dedupeWindow.Minutes()))
}

// normalizedHash returns a SHA-256 hex digest of content after collapsing
// whitespace and lowercasing. Matches sdd-memory's hashNormalized function so
// dedup semantics are consistent when observations are synced cross-machine.
func normalizedHash(content string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(content), " "))
	h := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(h[:])
}

// generateHex generates n random bytes encoded as lowercase hex.
func generateHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type LocalStore struct {
	db *sql.DB
}

func NewLocalStore(dbPath string) (*LocalStore, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	return &LocalStore{db: db}, nil
}

func (s *LocalStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *LocalStore) Init() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			project TEXT NOT NULL,
			started_at TEXT NOT NULL,
			summary TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS observations (
			id TEXT PRIMARY KEY,
			session_id TEXT,
			type TEXT NOT NULL DEFAULT 'note',
			title TEXT NOT NULL DEFAULT '',
			project TEXT,
			scope TEXT NOT NULL,
			topic_key TEXT,
			content TEXT NOT NULL,
			revision_count INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS sync_mutations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			table_name TEXT NOT NULL,
			record_id TEXT NOT NULL,
			operation TEXT NOT NULL CHECK(operation IN ('upsert', 'delete')),
			payload TEXT,
			timestamp TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'chunked', 'synced'))
		);`,
		`CREATE TABLE IF NOT EXISTS sync_state (
			project TEXT PRIMARY KEY,
			last_mutation_id INTEGER NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS user_prompts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			sync_id TEXT,
			session_id TEXT,
			content TEXT NOT NULL,
			project TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		);`,
		`CREATE TABLE IF NOT EXISTS prompt_tombstones (
			sync_id TEXT PRIMARY KEY,
			session_id TEXT,
			project TEXT,
			deleted_at TEXT NOT NULL
		);`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("init db error: %w", err)
		}
	}

	// Phase 1 migrations for databases created before the sdd-memory-aligned contract.
	// ALTER TABLE ADD COLUMN is idempotent here only by ignoring the error when
	// the column already exists.
	for _, alter := range []string{
		`ALTER TABLE observations ADD COLUMN session_id TEXT`,
		`ALTER TABLE observations ADD COLUMN type TEXT NOT NULL DEFAULT 'note'`,
		`ALTER TABLE observations ADD COLUMN title TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE observations ADD COLUMN topic_key TEXT NOT NULL DEFAULT ''`,
	} {
		s.db.Exec(alter)
	}

	// Carry forward any legacy `topic` column values into topic_key.
	s.db.Exec(`UPDATE observations SET topic_key = topic WHERE topic_key = '' AND topic IS NOT NULL`)

	// Phase 2 migrations: sdd-memory engine port columns.
	// Each ALTER TABLE is ignored if the column already exists (SQLite returns
	// "duplicate column name" which we swallow). The UPDATE migrations are
	// safe to run multiple times (they are no-ops on already-migrated rows).
	for _, alter := range []string{
		`ALTER TABLE observations ADD COLUMN sync_id TEXT`,
		`ALTER TABLE observations ADD COLUMN tool_name TEXT`,
		`ALTER TABLE observations ADD COLUMN normalized_hash TEXT`,
		`ALTER TABLE observations ADD COLUMN duplicate_count INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE observations ADD COLUMN last_seen_at TEXT`,
		`ALTER TABLE observations ADD COLUMN deleted_at TEXT`,
	} {
		s.db.Exec(alter) // swallow error: "duplicate column name" is OK
	}

	// Backfill sync_id from id for pre-existing rows.
	s.db.Exec(`UPDATE observations SET sync_id = id WHERE sync_id IS NULL`)

	// Nullability normalization: treat empty string as NULL at the app layer.
	// SQLite cannot ALTER COLUMN to drop NOT NULL, so we normalize at the data layer.
	s.db.Exec(`UPDATE observations SET project = NULL WHERE project = ''`)
	s.db.Exec(`UPDATE observations SET topic_key = NULL WHERE topic_key = ''`)

	// Drop and rebuild FTS index to handle any schema changes.
	s.db.Exec(`DROP TABLE IF EXISTS observations_fts`)

	// Standalone (not external-content) FTS5 table: it keeps its own copy of the
	// indexed text, so a plain DELETE-by-rowid is safe even after the source row
	// has already been updated. The external-content variant corrupts on upsert
	// because it recomputes delete tokens from the post-update content row.
	if _, err := s.db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts USING fts5(
		project, scope, topic_key, title, content
	);`); err != nil {
		return fmt.Errorf("init fts error: %w", err)
	}

	// Repopulate the index from any pre-existing non-deleted observations.
	// Use ifnull to avoid null propagation into the FTS table.
	s.db.Exec(`
		INSERT INTO observations_fts (rowid, project, scope, topic_key, title, content)
		SELECT rowid, ifnull(project,''), ifnull(scope,''), ifnull(topic_key,''), title, content
		FROM observations
		WHERE deleted_at IS NULL
	`)

	return nil
}

func (s *LocalStore) logMutation(tx *sql.Tx, tableName, recordID, operation string, payload interface{}) error {
	var payloadStr *string
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		str := string(b)
		payloadStr = &str
	}

	_, err := tx.Exec(`
		INSERT INTO sync_mutations (table_name, record_id, operation, payload, timestamp, status)
		VALUES (?, ?, ?, ?, ?, 'pending')`,
		tableName, recordID, operation, payloadStr, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *LocalStore) GetObservation(id string) (*Observation, error) {
	row := s.db.QueryRow(`SELECT `+obsColumns+` FROM observations WHERE id = ? AND deleted_at IS NULL`, id)
	obs, err := scanObservation(row)
	if err != nil {
		return nil, err
	}
	return obs, nil
}

// FindByTopicKey locates an existing non-deleted observation for upsert by its
// stable topic_key within a project/scope.
func (s *LocalStore) FindByTopicKey(project, scope, topicKey string) (*Observation, error) {
	row := s.db.QueryRow(`SELECT `+obsColumns+` FROM observations WHERE ifnull(project,'') = ifnull(?,'') AND scope = ? AND topic_key = ? AND deleted_at IS NULL ORDER BY datetime(updated_at) DESC, datetime(created_at) DESC LIMIT 1`, project, scope, topicKey)
	return scanObservation(row)
}

func (s *LocalStore) GetLastMutationID(project string) (int64, error) {
	var id int64
	err := s.db.QueryRow(`SELECT last_mutation_id FROM sync_state WHERE project = ?`, project).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return id, err
}

func (s *LocalStore) SetLastMutationID(project string, id int64) error {
	_, err := s.db.Exec(`
		INSERT INTO sync_state (project, last_mutation_id)
		VALUES (?, ?)
		ON CONFLICT(project) DO UPDATE SET last_mutation_id = excluded.last_mutation_id
	`, project, id)
	return err
}

func (s *LocalStore) GetPendingMutations() ([]SyncMutation, error) {
	rows, err := s.db.Query(`SELECT id, table_name, record_id, operation, payload FROM sync_mutations WHERE status = 'pending' ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []SyncMutation
	for rows.Next() {
		var mut SyncMutation
		var payload *string
		if err := rows.Scan(&mut.ID, &mut.TableName, &mut.RecordID, &mut.Operation, &payload); err != nil {
			return nil, err
		}
		if payload != nil {
			mut.Payload = *payload
		}
		res = append(res, mut)
	}
	return res, nil
}

func (s *LocalStore) MarkMutationsSynced(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	args := make([]interface{}, len(ids))
	placeholders := make([]string, len(ids))
	for i, id := range ids {
		args[i] = id
		placeholders[i] = "?"
	}
	query := fmt.Sprintf(`UPDATE sync_mutations SET status = 'synced' WHERE id IN (%s)`, strings.Join(placeholders, ","))
	_, err := s.db.Exec(query, args...)
	return err
}

// Stats returns system statistics. Observation count excludes soft-deleted rows.
// Prompt count uses user_prompts with tombstone exclusion.
func (s *LocalStore) Stats() (*Stats, error) {
	stats := &Stats{}
	s.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&stats.TotalSessions)
	s.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE deleted_at IS NULL`).Scan(&stats.TotalObservations)

	// Count prompts from user_prompts, excluding tombstoned entries.
	s.db.QueryRow(`
		SELECT COUNT(*) FROM user_prompts
		LEFT JOIN prompt_tombstones ON user_prompts.sync_id = prompt_tombstones.sync_id
		WHERE prompt_tombstones.sync_id IS NULL
	`).Scan(&stats.TotalPrompts)

	rows, err := s.db.Query(`SELECT DISTINCT ifnull(project,'') FROM observations WHERE deleted_at IS NULL AND project IS NOT NULL`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p string
			rows.Scan(&p)
			if p != "" {
				stats.Projects = append(stats.Projects, p)
			}
		}
	}
	return stats, nil
}

func (s *LocalStore) CreateSession(id, project string) error {
	_, err := s.db.Exec(`INSERT OR IGNORE INTO sessions (id, project, started_at) VALUES (?, ?, ?)`, id, project, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (s *LocalStore) UpdateSessionSummary(id, summary string) error {
	_, err := s.db.Exec(`UPDATE sessions SET summary = ? WHERE id = ?`, summary, id)
	return err
}

func (s *LocalStore) RecentSessions(limit int) ([]SessionSummary, error) {
	rows, err := s.db.Query(`
		SELECT s.id, s.project, s.started_at, s.summary, COUNT(o.id)
		FROM sessions s
		LEFT JOIN observations o ON s.id = o.session_id AND o.deleted_at IS NULL
		GROUP BY s.id
		ORDER BY s.started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []SessionSummary
	for rows.Next() {
		var sess SessionSummary
		var start string
		rows.Scan(&sess.ID, &sess.Project, &start, &sess.Summary, &sess.ObservationCount)
		sess.StartedAt, _ = time.Parse(time.RFC3339, start)
		res = append(res, sess)
	}
	return res, nil
}

func (s *LocalStore) RecentObservations(limit int) ([]Observation, error) {
	rows, err := s.db.Query(`SELECT `+obsColumns+` FROM observations WHERE deleted_at IS NULL ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Observation
	for rows.Next() {
		obs, _ := scanObservation(rows)
		if obs != nil {
			res = append(res, *obs)
		}
	}
	return res, nil
}

func (s *LocalStore) ObservationsBySession(sessionID string) ([]Observation, error) {
	rows, err := s.db.Query(`SELECT `+obsColumns+` FROM observations WHERE session_id = ? AND deleted_at IS NULL ORDER BY created_at ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []Observation
	for rows.Next() {
		obs, _ := scanObservation(rows)
		if obs != nil {
			res = append(res, *obs)
		}
	}
	return res, nil
}

// UpdateObservation patches the mutable fields of an observation and refreshes
// its FTS row.
func (s *LocalStore) UpdateObservation(id, title, content, obsType, scope string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`UPDATE observations SET title = ?, content = ?, type = ?, scope = ?, updated_at = ? WHERE id = ?`,
		title, content, obsType, scope, time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}

	if err := refreshFTS(tx, id); err != nil {
		return err
	}

	return tx.Commit()
}

func scanObservation(scanner interface{ Scan(dest ...any) error }) (*Observation, error) {
	var obs Observation
	var syncID, sessionID, toolName, normHash, lastSeenAt, deletedAt sql.NullString
	var project, topicKey sql.NullString
	var cAt, uAt string

	err := scanner.Scan(
		&obs.ID, &syncID, &sessionID, &obs.Type, &obs.Title,
		&project, &obs.Scope, &topicKey, &obs.Content, &toolName,
		&normHash, &obs.RevisionCount, &obs.DuplicateCount,
		&lastSeenAt, &deletedAt, &cAt, &uAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Assign nullable string columns to pointer fields.
	if syncID.Valid {
		obs.SyncID = &syncID.String
	}
	obs.SessionID = sessionID.String
	if project.Valid {
		obs.Project = &project.String
	}
	if topicKey.Valid {
		obs.TopicKey = &topicKey.String
	}
	if toolName.Valid {
		obs.ToolName = &toolName.String
	}
	if normHash.Valid {
		obs.NormalizedHash = &normHash.String
	}
	if lastSeenAt.Valid {
		obs.LastSeenAt = &lastSeenAt.String
	}
	if deletedAt.Valid {
		obs.DeletedAt = &deletedAt.String
	}

	obs.CreatedAt, _ = time.Parse(time.RFC3339, cAt)
	obs.UpdatedAt, _ = time.Parse(time.RFC3339, uAt)
	return &obs, nil
}

// refreshFTS rewrites the FTS row for a single observation. Call inside a tx.
// Standalone FTS5 (not external-content): this DELETE+INSERT pattern is
// safe on upsert because the index holds its own copy of text — it does
// not re-read the source row during the DELETE phase.
func refreshFTS(tx *sql.Tx, id string) error {
	if _, err := tx.Exec(`DELETE FROM observations_fts WHERE rowid = (SELECT rowid FROM observations WHERE id = ?)`, id); err != nil {
		return err
	}
	_, err := tx.Exec(`
		INSERT INTO observations_fts (rowid, project, scope, topic_key, title, content)
		SELECT rowid, ifnull(project,''), ifnull(scope,''), ifnull(topic_key,''), title, content
		FROM observations WHERE id = ?
	`, id)
	return err
}

// removeFTS removes the FTS5 index row for an observation.
// Call inside an active transaction at soft-delete and hard-delete time.
func removeFTS(tx *sql.Tx, id string) error {
	_, err := tx.Exec(
		`DELETE FROM observations_fts WHERE rowid = (SELECT rowid FROM observations WHERE id = ?)`,
		id,
	)
	return err
}

// addObservationTx implements the three-branch dedup logic inside a transaction.
// logMut controls whether a sync_mutation is enqueued (false for inbound sync).
func (s *LocalStore) addObservationTx(tx *sql.Tx, p AddObservationParams, logMut bool) (string, error) {
	hash := normalizedHash(p.Content)
	now := time.Now().UTC().Format(time.RFC3339)

	// Normalize empty strings to nil for nullable pointer comparisons.
	var projectArg interface{}
	if p.Project != "" {
		projectArg = p.Project
	}
	var topicKeyArg interface{}
	if p.TopicKey != "" {
		topicKeyArg = p.TopicKey
	}
	var toolNameArg interface{}
	if p.ToolName != "" {
		toolNameArg = p.ToolName
	}

	// --- Branch A: topic_key revision ---
	if p.TopicKey != "" {
		var existingID string
		err := tx.QueryRow(`
			SELECT id FROM observations
			WHERE topic_key = ?
			  AND ifnull(project, '') = ifnull(?, '')
			  AND scope = ?
			  AND deleted_at IS NULL
			ORDER BY datetime(updated_at) DESC, datetime(created_at) DESC
			LIMIT 1
		`, p.TopicKey, p.Project, p.Scope).Scan(&existingID)

		if err == nil && existingID != "" {
			// Update in place.
			_, err = tx.Exec(`
				UPDATE observations SET
					type = ?,
					title = ?,
					content = ?,
					tool_name = ?,
					topic_key = ?,
					normalized_hash = ?,
					revision_count = revision_count + 1,
					last_seen_at = ?,
					updated_at = ?
				WHERE id = ?
			`, p.Type, p.Title, p.Content, toolNameArg, topicKeyArg, hash, now, now, existingID)
			if err != nil {
				return "", fmt.Errorf("branch A update: %w", err)
			}
			if err := refreshFTS(tx, existingID); err != nil {
				return "", fmt.Errorf("branch A refreshFTS: %w", err)
			}
			if logMut {
				obs, _ := s.getObservationTx(tx, existingID)
				if err := s.logMutation(tx, "observations", existingID, "upsert", obs); err != nil {
					return "", fmt.Errorf("branch A logMutation: %w", err)
				}
			}
			return existingID, nil
		} else if err != nil && err != sql.ErrNoRows {
			return "", fmt.Errorf("branch A select: %w", err)
		}
	}

	// --- Branch B: hash dedup within window ---
	{
		var existingID string
		err := tx.QueryRow(`
			SELECT id FROM observations
			WHERE normalized_hash = ?
			  AND ifnull(project, '') = ifnull(?, '')
			  AND scope = ?
			  AND type = ?
			  AND title = ?
			  AND deleted_at IS NULL
			  AND datetime(created_at) >= datetime('now', ?)
			ORDER BY created_at DESC
			LIMIT 1
		`, hash, p.Project, p.Scope, p.Type, p.Title, dedupeWindowArg()).Scan(&existingID)

		if err == nil && existingID != "" {
			_, err = tx.Exec(`
				UPDATE observations SET
					duplicate_count = duplicate_count + 1,
					last_seen_at = ?,
					updated_at = ?
				WHERE id = ?
			`, now, now, existingID)
			if err != nil {
				return "", fmt.Errorf("branch B update: %w", err)
			}
			if err := refreshFTS(tx, existingID); err != nil {
				return "", fmt.Errorf("branch B refreshFTS: %w", err)
			}
			if logMut {
				obs, _ := s.getObservationTx(tx, existingID)
				if err := s.logMutation(tx, "observations", existingID, "upsert", obs); err != nil {
					return "", fmt.Errorf("branch B logMutation: %w", err)
				}
			}
			return existingID, nil
		} else if err != nil && err != sql.ErrNoRows {
			return "", fmt.Errorf("branch B select: %w", err)
		}
	}

	// --- Branch C: new insert ---
	newID := "obs-" + generateHex(8)
	syncID := newID

	sessionID := p.SessionID
	if sessionID == "" {
		sessionID = "manual"
	}

	_, err := tx.Exec(`
		INSERT INTO observations
			(id, sync_id, session_id, type, title, content, tool_name, project,
			 scope, topic_key, normalized_hash, revision_count, duplicate_count,
			 last_seen_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, 1, ?, ?, ?)
	`, newID, syncID, sessionID, p.Type, p.Title, p.Content, toolNameArg, projectArg,
		p.Scope, topicKeyArg, hash, now, now, now)
	if err != nil {
		return "", fmt.Errorf("branch C insert: %w", err)
	}
	if err := refreshFTS(tx, newID); err != nil {
		return "", fmt.Errorf("branch C refreshFTS: %w", err)
	}
	if logMut {
		obs, _ := s.getObservationTx(tx, newID)
		if err := s.logMutation(tx, "observations", newID, "upsert", obs); err != nil {
			return "", fmt.Errorf("branch C logMutation: %w", err)
		}
	}
	return newID, nil
}

// getObservationTx reads an observation within an existing transaction.
func (s *LocalStore) getObservationTx(tx *sql.Tx, id string) (*Observation, error) {
	row := tx.QueryRow(`SELECT `+obsColumns+` FROM observations WHERE id = ?`, id)
	return scanObservation(row)
}

// AddObservation is the primary write path. It executes the three-branch dedup
// logic inside a single transaction and returns the string id of the affected row.
func (s *LocalStore) AddObservation(p AddObservationParams) (string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	id, err := s.addObservationTx(tx, p, !p.fromSync)
	if err != nil {
		return "", err
	}

	return id, tx.Commit()
}

// SaveObservation is a compatibility shim that calls AddObservation. Preserves
// existing callers in MCP handlers during the transition.
func (s *LocalStore) SaveObservation(obs *Observation) error {
	p := AddObservationParams{
		SessionID: obs.SessionID,
		Type:      obs.Type,
		Title:     obs.Title,
		Content:   obs.Content,
		Project:   strVal(obs.Project),
		Scope:     obs.Scope,
		TopicKey:  strVal(obs.TopicKey),
		ToolName:  strVal(obs.ToolName),
	}
	_, err := s.AddObservation(p)
	return err
}

// SaveObservationFromSync calls the three-branch logic WITHOUT enqueuing a
// sync_mutation, preserving the echo-loop prevention guarantee. FTS IS refreshed
// so inbound observations are searchable locally.
func (s *LocalStore) SaveObservationFromSync(obs *Observation) error {
	p := AddObservationParams{
		SessionID: obs.SessionID,
		Type:      obs.Type,
		Title:     obs.Title,
		Content:   obs.Content,
		Project:   strVal(obs.Project),
		Scope:     obs.Scope,
		TopicKey:  strVal(obs.TopicKey),
		ToolName:  strVal(obs.ToolName),
		fromSync:  true,
	}
	_, err := s.AddObservation(p)
	return err
}

// DeleteObservation soft-deletes (default) or hard-deletes an observation.
// Soft-delete: sets deleted_at = now(), removes the FTS row, enqueues a
//
//	sync_mutation (upsert) so the deletion propagates to cloud.
//
// Hard-delete: removes the row and the FTS row. Does NOT enqueue a sync
//
//	mutation (used for local cleanup only).
func (s *LocalStore) DeleteObservation(id string, hard bool) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := removeFTS(tx, id); err != nil {
		return err
	}

	if hard {
		_, err = tx.Exec(`DELETE FROM observations WHERE id = ?`, id)
		if err != nil {
			return err
		}
		// No mutation log for hard delete (local cleanup only).
	} else {
		now := time.Now().UTC().Format(time.RFC3339)
		_, err = tx.Exec(`UPDATE observations SET deleted_at = ?, updated_at = ? WHERE id = ?`, now, now, id)
		if err != nil {
			return err
		}
		// Re-read the updated row to build mutation payload.
		obs, err := s.getObservationTx(tx, id)
		if err != nil {
			return err
		}
		if err := s.logMutation(tx, "observations", id, "upsert", obs); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *LocalStore) DeleteObservationFromSync(id string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`DELETE FROM observations_fts WHERE rowid = (SELECT rowid FROM observations WHERE id = ?)`, id)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`DELETE FROM observations WHERE id = ?`, id)
	if err != nil {
		return err
	}

	// BYPASS mutation logging
	return tx.Commit()
}

func (s *LocalStore) SearchObservations(query string, project string) ([]Observation, error) {
	sqlQuery := `
		SELECT ` + prefixColumns("o", obsColumns) + `
		FROM observations_fts fts
		JOIN observations o ON o.rowid = fts.rowid
		WHERE observations_fts MATCH ?
		  AND o.deleted_at IS NULL`

	var args []interface{}
	// Escape the query for FTS5 by wrapping in quotes and escaping internal quotes.
	escapedQuery := "\"" + strings.ReplaceAll(query, "\"", "\"\"") + "\""
	args = append(args, escapedQuery)

	if project != "" {
		sqlQuery += ` AND o.project = ?`
		args = append(args, project)
	}
	sqlQuery += ` ORDER BY rank`

	rows, err := s.db.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Observation
	for rows.Next() {
		obs, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		if obs != nil {
			results = append(results, *obs)
		}
	}
	return results, nil
}

// AddPrompt writes a user prompt to the user_prompts table.
// Prompts are NOT indexed in FTS (content is often repetitive and searching
// prompts by keyword is not a use case for the lite version).
// Returns the inserted integer id.
func (s *LocalStore) AddPrompt(p AddPromptParams) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	syncID := "prompt-" + generateHex(8)

	var projectArg interface{}
	if p.Project != "" {
		projectArg = p.Project
	}

	res, err := tx.Exec(`
		INSERT INTO user_prompts (sync_id, session_id, content, project, created_at)
		VALUES (?, ?, ?, ?, datetime('now'))
	`, syncID, p.SessionID, p.Content, projectArg)
	if err != nil {
		return 0, fmt.Errorf("AddPrompt insert: %w", err)
	}

	insertedID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Enqueue sync mutation for user_prompts.
	if err := s.logMutation(tx, "user_prompts", syncID, "upsert", map[string]interface{}{
		"id": insertedID, "sync_id": syncID, "session_id": p.SessionID,
		"content": p.Content, "project": p.Project,
	}); err != nil {
		return 0, err
	}

	return insertedID, tx.Commit()
}

// RecentPrompts returns recent user prompts, excluding tombstoned entries.
func (s *LocalStore) RecentPrompts(project string, limit int) ([]Prompt, error) {
	rows, err := s.db.Query(`
		SELECT up.id, up.sync_id, up.session_id, up.content, ifnull(up.project,''), up.created_at
		FROM user_prompts up
		LEFT JOIN prompt_tombstones pt ON up.sync_id = pt.sync_id
		WHERE pt.sync_id IS NULL
		  AND (? = '' OR up.project = ?)
		ORDER BY up.created_at DESC
		LIMIT ?
	`, project, project, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []Prompt
	for rows.Next() {
		var p Prompt
		var syncID sql.NullString
		var cAt string
		if err := rows.Scan(&p.ID, &syncID, &p.SessionID, &p.Content, &p.Project, &cAt); err != nil {
			return nil, err
		}
		if syncID.Valid {
			p.SyncID = &syncID.String
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, cAt)
		res = append(res, p)
	}
	return res, nil
}

// prefixColumns qualifies a comma-separated column list with a table alias.
func prefixColumns(alias, cols string) string {
	parts := strings.Split(cols, ", ")
	for i, p := range parts {
		parts[i] = alias + "." + p
	}
	return strings.Join(parts, ", ")
}
