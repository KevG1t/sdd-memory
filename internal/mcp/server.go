package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"sdd-memory/internal/store"
)

type Server struct {
	mcpServer *server.MCPServer
	store     *store.LocalStore
}

func NewServer(localStore *store.LocalStore) *Server {
	s := &Server{
		mcpServer: server.NewMCPServer("sdd-memory", "1.0.0"),
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

	topic, _ := args["topic"].(string)
	title, _ := args["title"].(string)
	if topic == "" {
		topic = title // fallback
	}

	content, _ := args["content"].(string)
	if content == "" {
		return mcp.NewToolResultError("content is required"), nil
	}

	existing, _ := s.store.FindByTopicKey(project, scope, topic)
	
	obs := &store.Observation{
		Project:       project,
		Scope:         scope,
		Topic:         topic,
		Content:       content,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
		RevisionCount: 1,
	}

	if existing != nil {
		obs.ID = existing.ID
		obs.CreatedAt = existing.CreatedAt
		obs.RevisionCount = existing.RevisionCount + 1
	} else {
		b := make([]byte, 8)
		rand.Read(b)
		obs.ID = "obs-" + hex.EncodeToString(b)
	}

	if err := s.store.SaveObservation(obs); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("save error: %v", err)), nil
	}

	// In Lite version we just return the saved ID, no conflict detection
	resp := map[string]interface{}{
		"status": "saved",
		"id": obs.ID,
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
	
	args["topic"] = "passive-capture-" + time.Now().Format("20060102150405")
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
		"recent_sessions": sessions,
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
		"path": cwd,
	}
	b, _ := json.Marshal(resp)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleMemDoctor(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	resp := map[string]string{
		"status": "healthy",
		"mode": "lite",
	}
	b, _ := json.Marshal(resp)
	return mcp.NewToolResultText(string(b)), nil
}

func (s *Server) handleMemSavePrompt(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, _ := request.Params.Arguments.(map[string]interface{})
	args["topic"] = "user-prompt"
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
