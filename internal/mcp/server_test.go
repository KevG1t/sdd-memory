package mcp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/KevG1t/sdd-memory/internal/store"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// newTestStore creates a LocalStore backed by a t.TempDir() SQLite database.
func newTestStore(t *testing.T) *store.LocalStore {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := store.NewLocalStore(path)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// newTestServer creates a Server backed by a fresh test store.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	return NewServer(newTestStore(t))
}

// callTool is a helper that builds a CallToolRequest and calls the given handler.
func callTool(ctx context.Context, handler func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error), args map[string]interface{}) (map[string]interface{}, error) {
	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = args
	result, err := handler(ctx, req)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	// Extract text content and parse JSON.
	if len(result.Content) == 0 {
		return map[string]interface{}{}, nil
	}
	txt, ok := result.Content[0].(mcpgo.TextContent)
	if !ok {
		return map[string]interface{}{}, nil
	}
	var resp map[string]interface{}
	json.Unmarshal([]byte(txt.Text), &resp)
	return resp, nil
}

func TestHandleMemSave_NewInsert(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	resp, err := callTool(ctx, srv.handleMemSave, map[string]interface{}{
		"content": "hello from mem_save test",
		"project": "p",
		"type":    "note",
	})
	if err != nil {
		t.Fatalf("handleMemSave: %v", err)
	}

	status, _ := resp["status"].(string)
	if status != "saved" {
		t.Errorf("status: got %q, want %q", status, "saved")
	}
	id, _ := resp["id"].(string)
	if id == "" {
		t.Error("expected non-empty id")
	}
	syncID, _ := resp["sync_id"].(string)
	if syncID == "" {
		t.Error("expected non-empty sync_id")
	}

	// Verify round-trip via GetObservation.
	obs, err := srv.store.GetObservation(id)
	if err != nil || obs == nil {
		t.Fatalf("GetObservation(%q): obs=%v err=%v", id, obs, err)
	}
	if obs.Content != "hello from mem_save test" {
		t.Errorf("content mismatch: got %q", obs.Content)
	}
}

func TestHandleMemSave_TopicKeyRevision(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// First save.
	resp1, _ := callTool(ctx, srv.handleMemSave, map[string]interface{}{
		"content":   "initial content",
		"project":   "p",
		"scope":     "project",
		"topic_key": "k",
	})
	id1, _ := resp1["id"].(string)

	// Second save with same topic_key+project+scope.
	resp2, _ := callTool(ctx, srv.handleMemSave, map[string]interface{}{
		"content":   "revised content",
		"project":   "p",
		"scope":     "project",
		"topic_key": "k",
	})
	id2, _ := resp2["id"].(string)

	if id1 != id2 {
		t.Errorf("expected same id on topic_key revision: %q vs %q", id1, id2)
	}

	obs, _ := srv.store.GetObservation(id1)
	if obs == nil {
		t.Fatal("observation not found")
	}
	if obs.RevisionCount != 2 {
		t.Errorf("revision_count: got %d, want 2", obs.RevisionCount)
	}
}

func TestHandleMemSearch_Returns(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Save an observation with unique content.
	resp, _ := callTool(ctx, srv.handleMemSave, map[string]interface{}{
		"content": "unique searchable architecture content zeta",
		"project": "p",
	})
	id, _ := resp["id"].(string)

	// Search for it.
	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"query": "unique searchable architecture content zeta",
	}
	result, err := srv.handleMemSearch(ctx, req)
	if err != nil {
		t.Fatalf("handleMemSearch: %v", err)
	}
	txt, _ := result.Content[0].(mcpgo.TextContent)
	if txt.Text == "" {
		t.Fatal("empty search result")
	}
	if !containsID(t, txt.Text, id) {
		t.Errorf("expected id %q in search results, got: %s", id, txt.Text)
	}
}

func TestHandleMemSearch_ExcludesSoftDeleted(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	resp, _ := callTool(ctx, srv.handleMemSave, map[string]interface{}{
		"content": "soft deleted search content theta",
		"project": "p",
	})
	id, _ := resp["id"].(string)

	// Soft-delete it.
	if err := srv.store.DeleteObservation(id, false); err != nil {
		t.Fatalf("DeleteObservation: %v", err)
	}

	// Search should NOT return it.
	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{
		"query": "soft deleted search content theta",
	}
	result, _ := srv.handleMemSearch(ctx, req)
	txt, _ := result.Content[0].(mcpgo.TextContent)
	if containsID(t, txt.Text, id) {
		t.Error("soft-deleted observation should not appear in search results")
	}
}

func TestHandleMemGetObservation_NotFound(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Save and soft-delete.
	resp, _ := callTool(ctx, srv.handleMemSave, map[string]interface{}{
		"content": "observation to soft delete for get test",
		"project": "p",
	})
	id, _ := resp["id"].(string)
	srv.store.DeleteObservation(id, false)

	// mem_get_observation should return not-found error.
	getResp, err := callTool(ctx, srv.handleMemGetObservation, map[string]interface{}{
		"id": id,
	})
	if err != nil {
		t.Fatalf("handleMemGetObservation error: %v", err)
	}
	// callTool returns a nil map for error results; a soft-deleted observation
	// must surface as a not-found error, so getResp must be nil.
	if getResp != nil {
		t.Errorf("expected nil (error result) for soft-deleted observation, got %v", getResp)
	}
	// Verify directly via store.
	obs, _ := srv.store.GetObservation(id)
	if obs != nil {
		t.Error("GetObservation should return nil for soft-deleted observation")
	}
}

func TestHandleMemSavePrompt_WritesToUserPrompts(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	statsB, _ := srv.store.Stats()
	if statsB.TotalPrompts != 0 {
		t.Fatalf("expected TotalPrompts=0 before, got %d", statsB.TotalPrompts)
	}

	resp, err := callTool(ctx, srv.handleMemSavePrompt, map[string]interface{}{
		"content":    "what is X?",
		"project":    "p",
		"session_id": "sess-1",
	})
	if err != nil {
		t.Fatalf("handleMemSavePrompt: %v", err)
	}
	status, _ := resp["status"].(string)
	if status != "saved" {
		t.Errorf("status: got %q, want saved", status)
	}

	statsA, _ := srv.store.Stats()
	if statsA.TotalPrompts != 1 {
		t.Errorf("TotalPrompts: got %d, want 1", statsA.TotalPrompts)
	}

	// No new observation row should have been created.
	if statsA.TotalObservations != 0 {
		t.Errorf("TotalObservations: got %d, want 0 (prompt should not create observation)", statsA.TotalObservations)
	}
}

func TestHandleMemStats(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Insert 2 observations.
	srv.store.AddObservation(store.AddObservationParams{Type: "note", Title: "T1", Content: "obs 1 content", Scope: "project"})
	srv.store.AddObservation(store.AddObservationParams{Type: "note", Title: "T2", Content: "obs 2 content", Scope: "project"})

	// Insert 1 that we will soft-delete.
	idDel, _ := srv.store.AddObservation(store.AddObservationParams{Type: "note", Title: "T3", Content: "obs 3 deleted", Scope: "project"})
	srv.store.DeleteObservation(idDel, false)

	// Insert 1 prompt.
	srv.store.AddPrompt(store.AddPromptParams{Content: "a prompt", Project: "p", SessionID: "s"})

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]interface{}{}
	result, err := srv.handleMemStats(ctx, req)
	if err != nil {
		t.Fatalf("handleMemStats: %v", err)
	}
	txt, _ := result.Content[0].(mcpgo.TextContent)

	var stats map[string]interface{}
	json.Unmarshal([]byte(txt.Text), &stats)

	totalObs := int(stats["total_observations"].(float64))
	if totalObs != 2 {
		t.Errorf("TotalObservations: got %d, want 2", totalObs)
	}
	totalPrompts := int(stats["total_prompts"].(float64))
	if totalPrompts != 1 {
		t.Errorf("TotalPrompts: got %d, want 1", totalPrompts)
	}
}

// containsID checks whether a JSON response string contains the given id.
func containsID(t *testing.T, jsonStr, id string) bool {
	t.Helper()
	var arr []map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &arr); err != nil {
		return false
	}
	for _, item := range arr {
		if item["id"] == id {
			return true
		}
	}
	return false
}
