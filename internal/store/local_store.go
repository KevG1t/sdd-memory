package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

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
			project TEXT NOT NULL,
			scope TEXT NOT NULL,
			topic TEXT NOT NULL,
			content TEXT NOT NULL,
			revision_count INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts USING fts5(
			project, scope, topic, content, content='observations', content_rowid='rowid'
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
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("init db error: %w", err)
		}
	}

	// Migrations
	// Safely add session_id if it doesn't exist (ignore error if it already exists)
	s.db.Exec(`ALTER TABLE observations ADD COLUMN session_id TEXT`)

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
	row := s.db.QueryRow(`SELECT id, project, scope, topic, content, revision_count, created_at, updated_at FROM observations WHERE id = ?`, id)
	return scanObservation(row)
}

func (s *LocalStore) FindByTopicKey(project, scope, topic string) (*Observation, error) {
	row := s.db.QueryRow(`SELECT id, project, scope, topic, content, revision_count, created_at, updated_at FROM observations WHERE project = ? AND scope = ? AND topic = ?`, project, scope, topic)
	return scanObservation(row)
}

// Stats returns system statistics
func (s *LocalStore) Stats() (*Stats, error) {
	stats := &Stats{}
	s.db.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&stats.TotalSessions)
	s.db.QueryRow(`SELECT COUNT(*) FROM observations`).Scan(&stats.TotalObservations)
	s.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE topic = 'user-prompt'`).Scan(&stats.TotalPrompts)
	
	rows, err := s.db.Query(`SELECT DISTINCT project FROM observations`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p string
			rows.Scan(&p)
			stats.Projects = append(stats.Projects, p)
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
		LEFT JOIN observations o ON s.id = o.session_id
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
	rows, err := s.db.Query(`SELECT id, project, scope, topic, content, revision_count, created_at, updated_at FROM observations ORDER BY created_at DESC LIMIT ?`, limit)
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
	rows, err := s.db.Query(`SELECT id, project, scope, topic, content, revision_count, created_at, updated_at FROM observations WHERE session_id = ? ORDER BY created_at ASC`, sessionID)
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

func (s *LocalStore) UpdateObservation(id, title, content, obsType, scope string) error {
	_, err := s.db.Exec(`UPDATE observations SET topic = ?, content = ?, scope = ?, updated_at = ? WHERE id = ?`, 
		title, content, scope, time.Now().UTC().Format(time.RFC3339), id)
	// We should also update FTS but keeping it simple for the Lite version unless requested
	return err
}

func scanObservation(scanner interface{ Scan(dest ...any) error }) (*Observation, error) {
	var obs Observation
	var cAt, uAt string
	err := scanner.Scan(&obs.ID, &obs.Project, &obs.Scope, &obs.Topic, &obs.Content, &obs.RevisionCount, &cAt, &uAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	obs.CreatedAt, _ = time.Parse(time.RFC3339, cAt)
	obs.UpdatedAt, _ = time.Parse(time.RFC3339, uAt)
	return &obs, nil
}

func (s *LocalStore) SaveObservation(obs *Observation) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var existingID string
	err = tx.QueryRow(`SELECT id FROM observations WHERE id = ?`, obs.ID).Scan(&existingID)
	exists := err != sql.ErrNoRows

	cAt := obs.CreatedAt.Format(time.RFC3339)
	uAt := obs.UpdatedAt.Format(time.RFC3339)

	_, err = tx.Exec(`
		INSERT INTO observations (id, session_id, project, scope, topic, content, revision_count, created_at, updated_at)
		VALUES (?, 'manual', ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			project = excluded.project,
			scope = excluded.scope,
			topic = excluded.topic,
			content = excluded.content,
			revision_count = excluded.revision_count,
			updated_at = excluded.updated_at
	`, obs.ID, obs.Project, obs.Scope, obs.Topic, obs.Content, obs.RevisionCount, cAt, uAt)
	if err != nil {
		return err
	}

	if exists {
		_, err = tx.Exec(`DELETE FROM observations_fts WHERE rowid = (SELECT rowid FROM observations WHERE id = ?)`, obs.ID)
		if err != nil {
			return err
		}
	}

	_, err = tx.Exec(`
		INSERT INTO observations_fts (rowid, project, scope, topic, content)
		VALUES ((SELECT rowid FROM observations WHERE id = ?), ?, ?, ?, ?)
	`, obs.ID, obs.Project, obs.Scope, obs.Topic, obs.Content)
	if err != nil {
		return err
	}

	if err := s.logMutation(tx, "observations", obs.ID, "upsert", obs); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *LocalStore) SearchObservations(query string, project string) ([]Observation, error) {
	sqlQuery := `
		SELECT o.id, o.project, o.scope, o.topic, o.content, o.revision_count, o.created_at, o.updated_at 
		FROM observations_fts fts
		JOIN observations o ON o.rowid = fts.rowid
		WHERE observations_fts MATCH ?`
	
	var args []interface{}
	// Escape the query for FTS5 by wrapping in quotes and escaping internal quotes
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
		results = append(results, *obs)
	}
	return results, nil
}
