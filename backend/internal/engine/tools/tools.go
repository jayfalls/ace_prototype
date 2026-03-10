package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// ToolSource represents where a tool comes from
type ToolSource int

const (
	SourceBuiltIn ToolSource = iota // Home-grown tools
	SourceMCP                        // Model Context Protocol
	SourceSkill                      // Agentic skills
)

// ToolDefinition defines a tool
type ToolDefinition struct {
	ID          uuid.UUID
	Name        string
	Description string
	Source      ToolSource
	Schema      ToolSchema
	Enabled     bool
	AgentID     *uuid.UUID // nil for global
}

// ToolSchema defines tool input/output
type ToolSchema struct {
	Input  map[string]interface{}
	Output map[string]interface{}
}

// Tool represents an executable tool
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
	Schema() ToolSchema
}

// Executor runs tools
type Executor struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewExecutor creates a tool executor
func NewExecutor() *Executor {
	return &Executor{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool
func (e *Executor) Register(tool Tool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.tools[tool.Name()] = tool
}

// Get returns a tool by name
func (e *Executor) Get(name string) (Tool, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	tool, ok := e.tools[name]
	return tool, ok
}

// List returns all registered tools
func (e *Executor) List() []Tool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]Tool, 0, len(e.tools))
	for _, tool := range e.tools {
		result = append(result, tool)
	}
	return result
}

// Execute runs a tool by name
func (e *Executor) Execute(ctx context.Context, name string, input map[string]interface{}) (map[string]interface{}, error) {
	tool, ok := e.Get(name)
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return tool.Execute(ctx, input)
}

// BaseTool provides common tool functionality
type BaseTool struct {
	name        string
	description string
	schema      ToolSchema
	executor    func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)
}

func (t *BaseTool) Name() string        { return t.name }
func (t *BaseTool) Description() string { return t.description }
func (t *BaseTool) Schema() ToolSchema  { return t.schema }

func (t *BaseTool) Execute(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	if t.executor != nil {
		return t.executor(ctx, input)
	}
	return nil, fmt.Errorf("no executor configured")
}

// NewBaseTool creates a configurable tool
func NewBaseTool(name, desc string, exec func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error)) *BaseTool {
	return &BaseTool{
		name:        name,
		description: desc,
		schema:      ToolSchema{Input: map[string]interface{}{}, Output: map[string]interface{}{}},
		executor:    exec,
	}
}

// BuiltInTools returns default tools
func BuiltInTools() []Tool {
	return []Tool{
		NewBaseTool("web_search", "Search the web", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			query, _ := input["query"].(string)
			return map[string]interface{}{"results": []string{"mock search result for: " + query}}, nil
		}),
		NewBaseTool("calculator", "Perform calculations", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			expr, _ := input["expression"].(string)
			return map[string]interface{}{"result": "mock: " + expr}, nil
		}),
		NewBaseTool("code_executor", "Execute code", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			_, _ = input["code"]
			lang, _ := input["language"].(string)
			return map[string]interface{}{"output": "mock execution of " + lang + " code"}, nil
		}),
		NewBaseTool("file_reader", "Read files", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			path, _ := input["path"].(string)
			return map[string]interface{}{"content": "mock file content: " + path}, nil
		}),
		NewBaseTool("webpage_fetch", "Fetch webpage content", func(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
			url, _ := input["url"].(string)
			return map[string]interface{}{"content": "mock webpage: " + url}, nil
		}),
	}
}

// MCPClient handles Model Context Protocol tools
type MCPClient struct {
	mu       sync.RWMutex
	servers  map[string]*MCPServer
	executor *Executor
}

// MCPServer represents an MCP server connection
type MCPServer struct {
	ID        uuid.UUID
	Name      string
	URL       string
	Tools     []ToolDefinition
	Connected bool
}

// NewMCPClient creates an MCP client
func NewMCPClient(executor *Executor) *MCPClient {
	return &MCPClient{
		servers:  make(map[string]*MCPServer),
		executor: executor,
	}
}

// Connect establishes connection to MCP server
func (c *MCPClient) Connect(ctx context.Context, name, url string) error {
	server := &MCPServer{
		ID:        uuid.New(),
		Name:      name,
		URL:       url,
		Connected: true,
		Tools:     []ToolDefinition{}, // Would connect and get tools
	}

	c.mu.Lock()
	c.servers[name] = server
	c.mu.Unlock()

	return nil
}

// Disconnect closes connection
func (c *MCPClient) Disconnect(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if server, ok := c.servers[name]; ok {
		server.Connected = false
	}
	return nil
}

// ListTools returns available MCP tools
func (c *MCPClient) ListTools() []ToolDefinition {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var tools []ToolDefinition
	for _, server := range c.servers {
		tools = append(tools, server.Tools...)
	}
	return tools
}

// Skill represents an agentic skill
type Skill struct {
	ID          uuid.UUID
	Name        string
	Description string
	Prompt      string
	Tools       []string
}

// SkillExecutor runs agentic skills
type SkillExecutor struct {
	mu     sync.RWMutex
	skills map[string]Skill
}

// NewSkillExecutor creates a skill executor
func NewSkillExecutor() *SkillExecutor {
	return &SkillExecutor{
		skills: make(map[string]Skill),
	}
}

// Register adds a skill
func (e *SkillExecutor) Register(skill Skill) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.skills[skill.Name] = skill
}

// Get returns a skill
func (e *SkillExecutor) Get(name string) (Skill, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	skill, ok := e.skills[name]
	return skill, ok
}

// Execute runs a skill (returns the skill prompt for LLM)
func (e *SkillExecutor) Execute(ctx context.Context, name string, input map[string]interface{}) (string, error) {
	skill, ok := e.Get(name)
	if !ok {
		return "", fmt.Errorf("skill not found: %s", name)
	}
	return skill.Prompt, nil
}

// SkillRegistry holds all skills
type SkillRegistry struct {
	mu     sync.RWMutex
	skills map[string]Skill
}

// NewSkillRegistry creates a global skill registry
func NewSkillRegistry() *SkillRegistry {
	return &SkillRegistry{
		skills: make(map[string]Skill),
	}
}

func (r *SkillRegistry) Register(skill Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[skill.Name] = skill
}

func (r *SkillRegistry) Get(name string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	skill, ok := r.skills[name]
	return skill, ok
}

func (r *SkillRegistry) List() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		result = append(result, s)
	}
	return result
}

// BuiltInSkills returns default skills
func BuiltInSkills() []Skill {
	return []Skill{
		{
			Name:        "research",
			Description: "Research a topic thoroughly",
			Prompt:      "You are a research assistant. Conduct thorough research on: {{topic}}. Use web search to find relevant information.",
			Tools:       []string{"web_search", "webpage_fetch"},
		},
		{
			Name:        "code_review",
			Description: "Review code for issues",
			Prompt:      "You are a code reviewer. Analyze the following code for bugs, security issues, and best practices: {{code}}",
			Tools:       []string{"code_executor"},
		},
	}
}
