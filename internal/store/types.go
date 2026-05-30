package store

import "time"

type Observation struct {
	ID            string    `json:"id"`
	Project       string    `json:"project"`
	Scope         string    `json:"scope"`
	Topic         string    `json:"topic"`
	Content       string    `json:"content"`
	RevisionCount int       `json:"revisionCount"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type SyncMutation struct {
	ID        int64       `json:"id"`
	TableName string      `json:"tableName"`
	RecordID  string      `json:"recordId"`
	Operation string      `json:"operation"`
	Payload   interface{} `json:"payload,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Status    string      `json:"status"`
}

type Session struct {
	ID        string    `json:"id"`
	Project   string    `json:"project"`
	StartedAt time.Time `json:"startedAt"`
	Summary   *string   `json:"summary,omitempty"`
}

type SessionSummary struct {
	Session
	ObservationCount int `json:"observationCount"`
}

type Stats struct {
	TotalSessions     int      `json:"totalSessions"`
	TotalObservations int      `json:"totalObservations"`
	TotalPrompts      int      `json:"totalPrompts"`
	Projects          []string `json:"projects"`
}
