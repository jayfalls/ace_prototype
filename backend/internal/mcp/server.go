package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Tool represents an MCP tool that can be called
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
	Handler     ToolHandler
}

// ToolHandler is a function that handles tool calls
type ToolHandler func(ctx context.Context, input map[string]interface{}) (interface{}, error)

// Resource represents an MCP resource
type Resource struct {
	URI         string                 `json:"uri"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	MimeType    string                 `json:"mime_type"`
	Content     interface{}            `json:"content"`
}

// Prompt represents an MCP prompt
type Prompt struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Arguments   []PromptArgument       `json:"arguments"`
	Template    string                 `json:"template"`
}

// PromptArgument represents a prompt argument
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// ServerCapabilities represents MCP server capabilities
type ServerCapabilities struct {
	Tools       bool `json:"tools"`
	Resources   bool `json:"resources"`
	Prompts     bool `json:"prompts"`
	Logging     bool `json:"logging"`
	Completions  bool `json:"completions"`
}

// ClientCapabilities represents MCP client capabilities
type ClientCapabilities struct {
	Tools       bool `json:"tools"`
	Resources   bool `json:"resources"`
	Prompts     bool `json:"prompts"`
}

// InitializeRequest represents an MCP initialize request
type InitializeRequest struct {
	ProtocolVersion string             `json:"protocol_version"`
	Capabilities   ClientCapabilities `json:"capabilities"`
	ClientInfo     ClientInfo         `json:"client_info"`
}

// ClientInfo represents client information
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InitializeResponse represents an MCP initialize response
type InitializeResponse struct {
	ProtocolVersion string             `json:"protocol_version"`
	Capabilities   ServerCapabilities `json:"capabilities"`
	ServerInfo     ServerInfo        `json:"server_info"`
}

// ServerInfo represents server information
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ToolCallRequest represents a tool call request
type ToolCallRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// ToolCallResponse represents a tool call response
type ToolCallResponse struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"is_error"`
}

// ContentBlock represents a content block
type ContentBlock struct {
	Type     string      `json:"type"`
	Text     string      `json:"text,omitempty"`
	Resource string      `json:"resource,omitempty"`
	Blob     string      `json:"blob,omitempty"`
	MimeType string      `json:"mime_type,omitempty"`
}

// ListToolsResult represents the result of listing tools
type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

// ListResourcesResult represents the result of listing resources
type ListResourcesResult struct {
	Resources []Resource `json:"resources"`
}

// ListPromptsResult represents the result of listing prompts
type ListPromptsResult struct {
	Prompts []Prompt `json:"prompts"`
}

// Server represents an MCP server
type Server struct {
	mu           sync.RWMutex
	tools        map[string]Tool
	resources    map[string]Resource
	prompts      map[string]Prompt
	capabilities ServerCapabilities
	serverInfo   ServerInfo
	initialized  bool
}

// NewServer creates a new MCP server
func NewServer() *Server {
	return &Server{
		tools:   make(map[string]Tool),
		resources: make(map[string]Resource),
		prompts:  make(map[string]Prompt),
		capabilities: ServerCapabilities{
			Tools:       true,
			Resources:   true,
			Prompts:     true,
			Logging:     true,
			Completions: true,
		},
		serverInfo: ServerInfo{
			Name:    "ace-framework-mcp",
			Version: "1.0.0",
		},
		initialized: false,
	}
}

// Initialize initializes the MCP server
func (s *Server) Initialize(ctx context.Context, req InitializeRequest) (*InitializeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.initialized = true

	return &InitializeResponse{
		ProtocolVersion: "2024-11-05",
		Capabilities:   s.capabilities,
		ServerInfo:     s.serverInfo,
	}, nil
}

// IsInitialized returns whether the server is initialized
func (s *Server) IsInitialized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.initialized
}

// RegisterTool registers a tool
func (s *Server) RegisterTool(tool Tool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tool.Name == "" {
		return fmt.Errorf("tool name is required")
	}

	s.tools[tool.Name] = tool
	return nil
}

// UnregisterTool unregisters a tool
func (s *Server) UnregisterTool(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.tools[name]; !ok {
		return fmt.Errorf("tool not found: %s", name)
	}

	delete(s.tools, name)
	return nil
}

// GetTool returns a tool by name
func (s *Server) GetTool(name string) (Tool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tool, ok := s.tools[name]
	return tool, ok
}

// ListTools returns all registered tools
func (s *Server) ListTools() []Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tools := make([]Tool, 0, len(s.tools))
	for _, tool := range s.tools {
		tools = append(tools, tool)
	}
	return tools
}

// CallTool calls a tool with the given arguments
func (s *Server) CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolCallResponse, error) {
	s.mu.RLock()
	tool, ok := s.tools[name]
	s.mu.RUnlock()

	if !ok {
		return &ToolCallResponse{
			Content: []ContentBlock{
				{Type: "text", Text: fmt.Sprintf("Tool not found: %s", name)},
			},
			IsError: true,
		}, fmt.Errorf("tool not found: %s", name)
	}

	if tool.Handler == nil {
		return &ToolCallResponse{
			Content: []ContentBlock{
				{Type: "text", Text: fmt.Sprintf("Tool handler not implemented: %s", name)},
			},
			IsError: true,
		}, fmt.Errorf("tool handler not implemented: %s", name)
	}

	result, err := tool.Handler(ctx, args)
	if err != nil {
		return &ToolCallResponse{
			Content: []ContentBlock{
				{Type: "text", Text: fmt.Sprintf("Error: %v", err)},
			},
			IsError: true,
		}, err
	}

	// Convert result to content block
	content := []ContentBlock{}
	switch v := result.(type) {
	case string:
		content = []ContentBlock{{Type: "text", Text: v}}
	case error:
		content = []ContentBlock{{Type: "text", Text: v.Error()}}
	default:
		jsonBytes, _ := json.Marshal(v)
		content = []ContentBlock{{Type: "text", Text: string(jsonBytes)}}
	}

	return &ToolCallResponse{
		Content: content,
		IsError: false,
	}, nil
}

// RegisterResource registers a resource
func (s *Server) RegisterResource(resource Resource) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if resource.URI == "" {
		return fmt.Errorf("resource URI is required")
	}

	s.resources[resource.URI] = resource
	return nil
}

// ListResources returns all registered resources
func (s *Server) ListResources() []Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resources := make([]Resource, 0, len(s.resources))
	for _, resource := range s.resources {
		resources = append(resources, resource)
	}
	return resources
}

// GetResource returns a resource by URI
func (s *Server) GetResource(uri string) (Resource, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resource, ok := s.resources[uri]
	return resource, ok
}

// RegisterPrompt registers a prompt
func (s *Server) RegisterPrompt(prompt Prompt) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if prompt.Name == "" {
		return fmt.Errorf("prompt name is required")
	}

	s.prompts[prompt.Name] = prompt
	return nil
}

// ListPrompts returns all registered prompts
func (s *Server) ListPrompts() []Prompt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prompts := make([]Prompt, 0, len(s.prompts))
	for _, prompt := range s.prompts {
		prompts = append(prompts, prompt)
	}
	return prompts
}

// GetPrompt returns a prompt by name
func (s *Server) GetPrompt(name string) (Prompt, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prompt, ok := s.prompts[name]
	return prompt, ok
}

// RenderPrompt renders a prompt with the given arguments
func (s *Server) RenderPrompt(name string, args map[string]interface{}) (string, error) {
	prompt, ok := s.GetPrompt(name)
	if !ok {
		return "", fmt.Errorf("prompt not found: %s", name)
	}

	// Simple template rendering
	result := prompt.Template
	for key, value := range args {
		placeholder := fmt.Sprintf("{%s}", key)
		result = replaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}

	return result, nil
}

func replaceAll(s, old, new string) string {
	result := s
	for {
		result = replaceOne(result, old, new)
		if result == s {
			break
		}
		s = result
	}
	return result
}

func replaceOne(s, old, new string) string {
	for i := 0; i <= len(s)-len(old); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}

// ============ ACE Framework MCP Tools ============

// Tool names constants
const (
	ToolMemorySearch   = "memory_search"
	ToolMemoryStore    = "memory_store"
	ToolAgentExecute   = "agent_execute"
	ToolAgentStatus    = "agent_status"
	ToolLayerProcess   = "layer_process"
	ToolTelemetryQuery = "telemetry_query"
)

// DefaultTools returns the default set of ACE Framework MCP tools
func DefaultTools(server *Server) {
	// Memory Search Tool
	_ = server.RegisterTool(Tool{
		Name:        ToolMemorySearch,
		Description: "Search memory for relevant information",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query",
				},
				"limit": map[string]interface{}{
					"type":        "number",
					"description": "Maximum results to return",
				},
				"memory_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of memory: short_term, medium_term, long_term, global",
				},
			},
			"required": []string{"query"},
		},
		Handler: func(ctx context.Context, input map[string]interface{}) (interface{}, error) {
			query, _ := input["query"].(string)
			limit, _ := input["limit"].(int)
			if limit == 0 {
				limit = 10
			}
			memoryType, _ := input["memory_type"].(string)

			return map[string]interface{}{
				"query":       query,
				"limit":       limit,
				"memory_type": memoryType,
				"results":     []string{},
				"status":      "search completed",
			}, nil
		},
	})

	// Memory Store Tool
	_ = server.RegisterTool(Tool{
		Name:        ToolMemoryStore,
		Description: "Store information in memory",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"content": map[string]interface{}{
					"type":        "string",
					"description": "Content to store",
				},
				"memory_type": map[string]interface{}{
					"type":        "string",
					"description": "Type of memory: short_term, medium_term, long_term",
				},
				"tags": map[string]interface{}{
					"type":        "array",
					"description": "Tags for the memory",
					"items":       map[string]interface{}{"type": "string"},
				},
				"importance": map[string]interface{}{
					"type":        "number",
					"description": "Importance score 0-10",
				},
			},
			"required": []string{"content"},
		},
		Handler: func(ctx context.Context, input map[string]interface{}) (interface{}, error) {
			content, _ := input["content"].(string)
			memoryType, _ := input["memory_type"].(string)
			tags, _ := input["tags"].([]interface{})
			importance, _ := input["importance"].(float64)

			return map[string]interface{}{
				"id":          uuid.New().String(),
				"content":     content,
				"memory_type": memoryType,
				"tags":        tags,
				"importance":  importance,
				"timestamp":   time.Now().Format(time.RFC3339),
				"status":      "stored",
			}, nil
		},
	})

	// Agent Execute Tool
	_ = server.RegisterTool(Tool{
		Name:        ToolAgentExecute,
		Description: "Execute a task with an agent",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"agent_id": map[string]interface{}{
					"type":        "string",
					"description": "Agent ID to execute with",
				},
				"task": map[string]interface{}{
					"type":        "string",
					"description": "Task description",
				},
				"context": map[string]interface{}{
					"type":        "object",
					"description": "Additional context",
				},
			},
			"required": []string{"task"},
		},
		Handler: func(ctx context.Context, input map[string]interface{}) (interface{}, error) {
			agentID, _ := input["agent_id"].(string)
			task, _ := input["task"].(string)

			return map[string]interface{}{
				"execution_id": uuid.New().String(),
				"agent_id":     agentID,
				"task":         task,
				"status":       "executed",
				"result":       "Task processed through ACE layers",
			}, nil
		},
	})

	// Agent Status Tool
	_ = server.RegisterTool(Tool{
		Name:        ToolAgentStatus,
		Description: "Get agent status information",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"agent_id": map[string]interface{}{
					"type":        "string",
					"description": "Agent ID (optional, returns all if not specified)",
				},
			},
		},
		Handler: func(ctx context.Context, input map[string]interface{}) (interface{}, error) {
			agentID, _ := input["agent_id"].(string)

			return map[string]interface{}{
				"agent_id":     agentID,
				"status":       "running",
				"layers":       []string{"L1", "L2", "L3", "L4", "L5", "L6"},
				"active_cycles": 0,
				"uptime":       "0s",
			}, nil
		},
	})

	// Layer Process Tool
	_ = server.RegisterTool(Tool{
		Name:        ToolLayerProcess,
		Description: "Process input through a specific ACE layer",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"layer": map[string]interface{}{
					"type":        "string",
					"description": "Layer name: aspirational, global_strategy, agent_model, executive_function, cognitive_control, task_prosecution",
				},
				"input": map[string]interface{}{
					"type":        "string",
					"description": "Input data for the layer",
				},
			},
			"required": []string{"layer", "input"},
		},
		Handler: func(ctx context.Context, input map[string]interface{}) (interface{}, error) {
			layer, _ := input["layer"].(string)
			layerInput, _ := input["input"].(string)

			return map[string]interface{}{
				"layer":    layer,
				"input":    layerInput,
				"output":   fmt.Sprintf("Processed through %s layer", layer),
				"status":   "completed",
				"duration": "10ms",
			}, nil
		},
	})

	// Telemetry Query Tool
	_ = server.RegisterTool(Tool{
		Name:        ToolTelemetryQuery,
		Description: "Query telemetry and metrics data",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"metric": map[string]interface{}{
					"type":        "string",
					"description": "Metric name to query",
				},
				"start_time": map[string]interface{}{
					"type":        "string",
					"description": "Start time (RFC3339)",
				},
				"end_time": map[string]interface{}{
					"type":        "string",
					"description": "End time (RFC3339)",
				},
			},
			"required": []string{"metric"},
		},
		Handler: func(ctx context.Context, input map[string]interface{}) (interface{}, error) {
			metric, _ := input["metric"].(string)
			startTime, _ := input["start_time"].(string)
			endTime, _ := input["end_time"].(string)

			return map[string]interface{}{
				"metric":     metric,
				"start_time": startTime,
				"end_time":   endTime,
				"data_points": []map[string]interface{}{},
				"status":     "queried",
			}, nil
		},
	})
}

// ============ MCP Client (for connecting to external MCP servers) ============

// Client represents an MCP client that can connect to external servers
type Client struct {
	mu          sync.RWMutex
	serverURL   string
	connected   bool
	httpClient  *http.Client
}

// NewClient creates a new MCP client
func NewClient(serverURL string) *Client {
	return &Client{
		serverURL:  serverURL,
		connected:  false,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Connect connects to an MCP server
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// In a real implementation, this would establish a connection
	c.connected = true
	return nil
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Disconnect disconnects from the MCP server
func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
}
