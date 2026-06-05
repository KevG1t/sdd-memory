package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/KevG1t/sdd-memory/internal/store"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const serverInstructions = `SDD Memory provides persistent memory that survives across sessions and compactions.

CORE TOOLS (always available — use without ToolSearch):
  mem_save — save decisions, bugs, discoveries, conventions PROACTIVELY (do not wait to be asked)
  mem_search — find past work, decisions, or context from previous sessions
  mem_context — get recent session history (call at session start or after compaction)
  mem_session_summary — save end-of-session summary (MANDATORY before saying "done")
  mem_get_observation — get full untruncated content of a search result by ID
  mem_save_prompt — save user prompt for context
  mem_current_project — detect current project from cwd (recommended first call)

DEFERRED TOOLS (use ToolSearch when needed):
  mem_update, mem_suggest_topic_key, mem_session_start, mem_session_end,
  mem_stats, mem_delete, mem_timeline, mem_capture_passive, mem_merge_projects

PROACTIVE SAVE RULE: Call mem_save immediately after ANY decision, bug fix, discovery, or convention — not just when asked.

## CONFLICT SURFACING — when mem_save returns candidates

After every mem_save call, check the response envelope for judgment_required.

IF judgment_required IS TRUE:
  Iterate candidates[] and call mem_judge once per candidate using the
  judgment_id from that candidate entry. Do NOT use the top-level judgment_id
  for multiple candidates — each candidate has its own.

  HEURISTIC — when to ask the user vs. resolve autonomously:

  ASK the user (conversationally, as part of your next reply) when:
    - confidence is below 0.7, OR
    - the relation you would choose is supersedes or conflicts_with AND the
      observation type is architecture, policy, or decision

  RESOLVE silently (call mem_judge without asking) when:
    - confidence >= 0.7 AND the relation is not supersedes/conflicts_with, OR
    - the relation is related, compatible, scoped, or not_conflict

  HOW TO ASK (conversationally — never via blocking CLI or dashboard prompt):
    Raise it naturally in your next reply to the user. Example phrasing:
    "I noticed memory #abc123 might conflict with what we just saved.
     Want me to mark the new one as superseding it, or are they about
     different scopes? I can also mark them as compatible if both still apply."

  AFTER RESOLUTION (both paths):
    Call mem_judge with the chosen relation, a reason, and if the user gave
    explicit direction, include their words as the evidence field. This persists
    the verdict and closes the pending conflict row.`

type Server struct {
	mcpServer *server.MCPServer
	store     *store.LocalStore
}

func NewServer(localStore *store.LocalStore) *Server {
	s := &Server{
		mcpServer: server.NewMCPServer("sdd-memory", "1.0.0", server.WithInstructions(serverInstructions)),
		store:     localStore,
	}
	s.registerTools()
	return s
}

func (s *Server) Start() error {
	return server.ServeStdio(s.mcpServer)
}

func (s *Server) registerTools() {
	tools := []struct {
		name string
		desc string
		fn   func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
	}{
		{"mem_save", "Save or update a memory observation", s.handleMemSave},
		{"mem_search", "Search memory observations", s.handleMemSearch},
		{"mem_get_observation", "Get a specific memory observation by ID", s.handleMemGetObservation},
		{"mem_capture_passive", "Passively capture context", s.handleMemCapturePassive},
		{"mem_compare", "Compare observations", s.handleMemJudge}, // Lite mode aliases this
		{"mem_context", "Fetch context for current state", s.handleMemContext},
		{"mem_current_project", "Get current project directory", s.handleMemCurrentProject},
		{"mem_doctor", "Run diagnostics", s.handleMemDoctor},
		{"mem_judge", "Evaluate decision against rules", s.handleMemJudge},
		{"mem_save_prompt", "Save a prompt template", s.handleMemSavePrompt},
		{"mem_session_start", "Start a new session", s.handleMemSessionStart},
		{"mem_session_end", "End current session", s.handleMemSessionEnd},
		{"mem_session_summary", "Summarize current session", s.handleMemSessionSummary},
		{"mem_suggest_topic_key", "Suggest a topic key for content", s.handleMemSuggestTopicKey},
		{"mem_update", "Update an existing observation", s.handleMemUpdate},
	}

	for _, t := range tools {
		tool := mcp.NewTool(t.name, mcp.WithDescription(t.desc))

		tool.InputSchema = mcp.ToolInputSchema{
			Type: "object",
			Properties: map[string]interface{}{
				"project":    map[string]interface{}{"type": "string"},
				"scope":      map[string]interface{}{"type": "string"},
				"topic":      map[string]interface{}{"type": "string"},
				"topic_key":  map[string]interface{}{"type": "string"},
				"content":    map[string]interface{}{"type": "string"},
				"query":      map[string]interface{}{"type": "string"},
				"id":         map[string]interface{}{"type": "string"},
				"session_id": map[string]interface{}{"type": "string"},
				"summary":    map[string]interface{}{"type": "string"},
				"title":      map[string]interface{}{"type": "string"},
				"type":       map[string]interface{}{"type": "string"},
			},
		}
		s.mcpServer.AddTool(tool, t.fn)
	}
}

func (s *Server) handleMemSave(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid arguments format"), nil
	}

	// Default to general project if empty
	project, _ := args["project"].(string)
	if project == "" {
		project = "default"
	}

	scope, _ := args["scope"].(string)
	if scope == "" {
		scope = "project"
	}

	// Resolve the stable upsert key. Engram/SpecAI send `topic_key`; accept the
	// legacy `topic` and finally `title` as fallbacks so older callers keep
	// working.
	topicKey, _ := args["topic_key"].(string)
	title, _ := args["title"].(string)
	if topicKey == "" {
		if t, _ := args["topic"].(string); t != "" {
			topicKey = t
		} else {
			topicKey = title
		}
	}

	obsType, _ := args["type"].(string)
	if obsType == "" {
		obsType = "note"
	}

	content, _ := args["content"].(string)
	if content == "" {
		return mcp.NewToolResultError("content is required"), nil
	}

	toolName, _ := args["tool_name"].(string)
	sessionID, _ := args["session_id"].(string)

	id, err := s.store.AddObservation(store.AddObservationParams{
		SessionID: sessionID,
		Type:      obsType,
		Title:     title,
		Content:   content,
		Project:   project,
		Scope:     scope,
		TopicKey:  topicKey,
		ToolName:  toolName,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("save error: %v", err)), nil
	}

	// Retrieve sync_id from the saved observation.
	var syncID string
	obs, _ := s.store.GetObservation(id)
	if obs != nil && obs.SyncID != nil {
		syncID = *obs.SyncID
	} else {
		syncID = id
	}

	// In Lite version we just return the saved ID, no conflict detection.
	resp := map[string]interface{}{
		"status":            "saved",
		"id":                id,
		"sync_id":           syncID,
		"judgment_required": false,
	}
	b, _ := json.Marshal(resp)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleMemSearch(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid arguments format"), nil
	}
	query, _ := args["query"].(string)
	project, _ := args["project"].(string)

	if query == "" {
		return mcp.NewToolResultError("query is required"), nil
	}

	results, err := s.store.SearchObservations(query, project)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search error: %v", err)), nil
	}

	b, _ := json.MarshalIndent(results, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleMemGetObservation(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid arguments format"), nil
	}
	id, _ := args["id"].(string)

	if id == "" {
		return mcp.NewToolResultError("id is required"), nil
	}

	obs, err := s.store.GetObservation(id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get error: %v", err)), nil
	}
	if obs == nil {
		return mcp.NewToolResultError("not found"), nil
	}

	b, _ := json.MarshalIndent(obs, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

// Lite implementation: passive capture just saves it as "passive-capture"
func (s *Server) handleMemCapturePassive(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]interface{})
	content, _ := args["content"].(string)

	if content == "" {
		return mcp.NewToolResultError("content required"), nil
	}

	args["topic_key"] = "passive-capture-" + time.Now().Format("20060102150405")
	if _, ok := args["type"]; !ok {
		args["type"] = "passive"
	}
	return s.handleMemSave(ctx, request)
}

func (s *Server) handleMemJudge(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Lite mode: just return OK without persisting complex relation graphs
	return mcp.NewToolResultText("{\"status\":\"judged\"}"), nil
}

func (s *Server) handleMemContext(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Return recent sessions and observations
	sessions, _ := s.store.RecentSessions(5)
	observations, _ := s.store.RecentObservations(10)

	resp := map[string]interface{}{
		"recent_sessions":     sessions,
		"recent_observations": observations,
	}
	b, _ := json.MarshalIndent(resp, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleMemCurrentProject(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	cwd, _ := os.Getwd()
	parts := strings.Split(cwd, string(os.PathSeparator))
	proj := parts[len(parts)-1]

	resp := map[string]string{
		"project": proj,
		"path":    cwd,
	}
	b, _ := json.Marshal(resp)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleMemDoctor(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resp := map[string]string{
		"status": "healthy",
		"mode":   "lite",
	}
	b, _ := json.Marshal(resp)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleMemSavePrompt(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]interface{})
	args["topic_key"] = "user-prompt"
	if _, ok := args["type"]; !ok {
		args["type"] = "prompt"
	}
	return s.handleMemSave(ctx, request)
}

func (s *Server) handleMemSessionStart(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid args"), nil
	}
	id, _ := args["id"].(string)
	project, _ := args["project"].(string)
	if id == "" {
		id = fmt.Sprintf("sess-%d", time.Now().Unix())
	}
	if project == "" {
		project = "default"
	}

	if err := s.store.CreateSession(id, project); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Started session %s", id)), nil
}

func (s *Server) handleMemSessionEnd(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]interface{})
	id, _ := args["id"].(string)
	summary, _ := args["summary"].(string)

	if id != "" && summary != "" {
		s.store.UpdateSessionSummary(id, summary)
	}
	return mcp.NewToolResultText("Session ended"), nil
}

func (s *Server) handleMemSessionSummary(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]interface{})
	id, _ := args["session_id"].(string)
	content, _ := args["content"].(string)

	if id != "" {
		s.store.UpdateSessionSummary(id, content)
	}
	return mcp.NewToolResultText("Summary saved"), nil
}

func (s *Server) handleMemSuggestTopicKey(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]interface{})
	title, _ := args["title"].(string)

	key := strings.ToLower(strings.ReplaceAll(title, " ", "-"))
	resp := map[string]string{"topic_key": key}
	b, _ := json.Marshal(resp)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleMemUpdate(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]interface{})
	id, _ := args["id"].(string)
	title, _ := args["title"].(string)
	content, _ := args["content"].(string)
	obsType, _ := args["type"].(string)
	scope, _ := args["scope"].(string)

	if err := s.store.UpdateObservation(id, title, content, obsType, scope); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Updated observation %s", id)), nil
}
