package store

import "time"

// Observation is the core memory record. Field names and JSON tags mirror the
// Engram wire contract (snake_case, topic_key/type/title) so SpecAI agents and
// the Engram MCP clients can talk to this Lite server without translation.
//
// Note: unlike Engram, the ID stays a string ("obs-<hex>") because it doubles
// as the record_id for the local sync_mutations log and the cloud sync layer.
type Observation struct {
	ID            string    `json:"id"`
	SessionID     string    `json:"session_id"`
	Type          string    `json:"type"`
	Title         string    `json:"title"`
	Project       string    `json:"project"`
	Scope         string    `json:"scope"`
	TopicKey      string    `json:"topic_key"`
	Content       string    `json:"content"`
	RevisionCount int       `json:"revision_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type SyncMutation struct {
	ID        int64       `json:"id"`
	TableName string      `json:"table_name"`
	RecordID  string      `json:"record_id"`
	Operation string      `json:"operation"`
	Payload   interface{} `json:"payload,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Status    string      `json:"status"`
}

type Session struct {
	ID        string    `json:"id"`
	Project   string    `json:"project"`
	StartedAt time.Time `json:"started_at"`
	Summary   *string   `json:"summary,omitempty"`
}

type SessionSummary struct {
	Session
	ObservationCount int `json:"observation_count"`
}

type Stats struct {
	TotalSessions     int      `json:"total_sessions"`
	TotalObservations int      `json:"total_observations"`
	TotalPrompts      int      `json:"total_prompts"`
	Projects          []string `json:"projects"`
}
