package store

import "time"

// Observation is the core memory record. Field names and JSON tags mirror the
// Engram wire contract (snake_case, topic_key/type/title) so SpecAI agents and
// the Engram MCP clients can talk to this Lite server without translation.
//
// Note: unlike Engram, the ID stays a string ("obs-<hex>") because it doubles
// as the record_id for the local sync_mutations log and the cloud sync layer.
type Observation struct {
	ID             string    `json:"id"`
	SyncID         *string   `json:"sync_id,omitempty"`
	SessionID      string    `json:"session_id"`
	Type           string    `json:"type"`
	Title          string    `json:"title"`
	Project        *string   `json:"project,omitempty"`      // was NOT NULL; now nullable
	Scope          string    `json:"scope"`
	TopicKey       *string   `json:"topic_key,omitempty"`    // was NOT NULL DEFAULT ''; now nullable
	Content        string    `json:"content"`
	ToolName       *string   `json:"tool_name,omitempty"`
	NormalizedHash *string   `json:"normalized_hash,omitempty"`
	RevisionCount  int       `json:"revision_count"`
	DuplicateCount int       `json:"duplicate_count"`
	LastSeenAt     *string   `json:"last_seen_at,omitempty"`
	DeletedAt      *string   `json:"deleted_at,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AddObservationParams carries parameters for the three-branch AddObservation write path.
// Callers pass empty string for Project/TopicKey/ToolName; the store normalizes to NULL.
type AddObservationParams struct {
	SessionID string
	Type      string
	Title     string
	Content   string
	Project   string // empty string → stored as NULL
	Scope     string
	TopicKey  string // empty string → Branch A skipped
	ToolName  string
	// fromSync is set internally by SaveObservationFromSync to bypass mutation logging.
	fromSync bool
}

// AddPromptParams carries parameters for writing a user prompt to user_prompts.
type AddPromptParams struct {
	SessionID string
	Content   string
	Project   string
}

// Prompt is a user prompt record stored in user_prompts.
type Prompt struct {
	ID        int64     `json:"id"`
	SyncID    *string   `json:"sync_id,omitempty"`
	SessionID string    `json:"session_id"`
	Content   string    `json:"content"`
	Project   string    `json:"project"`
	CreatedAt time.Time `json:"created_at"`
}

// StrVal dereferences a *string with empty-string fallback.
// Exported for use by callers outside the store package (e.g. TUI, MCP handlers).
func StrVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// strVal is an internal alias for StrVal.
func strVal(p *string) string { return StrVal(p) }

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
