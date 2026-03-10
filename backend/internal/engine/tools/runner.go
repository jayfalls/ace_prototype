package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Tool represents a tool that can be executed
type Tool struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Category    string                 `json:"category"`
	Schema      map[string]interface{} `json:"schema"`
	Handler     ToolHandler            `json:"-"`
	Enabled     bool                   `json:"enabled"`
}

// ToolHandler is a function that executes a tool
type ToolHandler func(ctx context.Context, args map[string]interface{}) (interface{}, error)

// ToolRunner executes tools
type ToolRunner struct {
	tools  map[string]*Tool
	config *RunnerConfig
}

// RunnerConfig holds tool runner configuration
type RunnerConfig struct {
	AllowedCommands []string
	Timeout        time.Duration
	HTTPTimeout    time.Duration
	MaxOutputSize  int64
}

// DefaultRunnerConfig returns the default configuration
func DefaultRunnerConfig() *RunnerConfig {
	return &RunnerConfig{
		AllowedCommands: []string{"curl", "wget", "python3", "node", "bash", "sh"},
		Timeout:        30 * time.Second,
		HTTPTimeout:    10 * time.Second,
		MaxOutputSize:  1024 * 1024, // 1MB
	}
}

// NewToolRunner creates a new tool runner
func NewToolRunner(config *RunnerConfig) *ToolRunner {
	if config == nil {
		config = DefaultRunnerConfig()
	}
	return &ToolRunner{
		tools:  make(map[string]*Tool),
		config: config,
	}
}

// RegisterTool registers a tool
func (r *ToolRunner) RegisterTool(tool *Tool) {
	tool.Enabled = true
	r.tools[tool.ID] = tool
}

// GetTool returns a tool by ID
func (r *ToolRunner) GetTool(id string) (*Tool, bool) {
	tool, ok := r.tools[id]
	return tool, ok
}

// ListTools returns all registered tools
func (r *ToolRunner) ListTools() []*Tool {
	tools := make([]*Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		if tool.Enabled {
			tools = append(tools, tool)
		}
	}
	return tools
}

// Execute runs a tool
func (r *ToolRunner) Execute(ctx context.Context, toolID string, args map[string]interface{}) (interface{}, error) {
	tool, ok := r.tools[toolID]
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", toolID)
	}

	if !tool.Enabled {
		return nil, fmt.Errorf("tool disabled: %s", toolID)
	}

	if tool.Handler == nil {
		return nil, fmt.Errorf("tool has no handler: %s", toolID)
	}

	return tool.Handler(ctx, args)
}

// RegisterDefaultTools registers the default toolset
func (r *ToolRunner) RegisterDefaultTools() {
	// Web Search tool
	r.RegisterTool(&Tool{
		ID:          "web_search",
		Name:        "Web Search",
		Description: "Search the web for information",
		Category:    "research",
		Handler:     r.handleWebSearch,
	})

	// HTTP Request tool
	r.RegisterTool(&Tool{
		ID:          "http_request",
		Name:        "HTTP Request",
		Description: "Make HTTP requests",
		Category:    "network",
		Schema: map[string]interface{}{
			"url":    "string",
			"method": "string",
			"body":   "string",
		},
		Handler: r.handleHTTPRequest,
	})

	// Execute Code tool
	r.RegisterTool(&Tool{
		ID:          "execute_code",
		Name:        "Execute Code",
		Description: "Run code snippets",
		Category:    "execution",
		Schema: map[string]interface{}{
			"language": "string",
			"code":     "string",
		},
		Handler: r.handleExecuteCode,
	})

	// Calculator tool
	r.RegisterTool(&Tool{
		ID:          "calculator",
		Name:        "Calculator",
		Description: "Perform mathematical calculations",
		Category:    "utility",
		Schema: map[string]interface{}{
			"expression": "string",
		},
		Handler: r.handleCalculator,
	})

	// File Read tool
	r.RegisterTool(&Tool{
		ID:          "file_read",
		Name:        "File Read",
		Description: "Read files from disk",
		Category:    "filesystem",
		Schema: map[string]interface{}{
			"path": "string",
		},
		Handler: r.handleFileRead,
	})
}

// Tool handlers

func (r *ToolRunner) handleWebSearch(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	query, ok := args["query"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required argument: query")
	}

	// Simple search using DuckDuckGo HTML
	url := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", strings.ReplaceAll(query, " ", "+"))
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: r.config.HTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, r.config.MaxOutputSize))
	if err != nil {
		return nil, err
	}

	// Extract snippets (simple approach)
	result := map[string]interface{}{
		"query":   query,
		"url":     url,
		"results": string(body[:min(5000, len(body))]),
	}

	return result, nil
}

func (r *ToolRunner) handleHTTPRequest(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	url, ok := args["url"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required argument: url")
	}

	method := "GET"
	if m, ok := args["method"].(string); ok {
		method = m
	}

	var body io.Reader
	if b, ok := args["body"].(string); ok && b != "" {
		body = strings.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: r.config.HTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, r.config.MaxOutputSize))
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"status_code": resp.StatusCode,
		"headers":     resp.Header,
		"body":        string(respBody),
	}, nil
}

func (r *ToolRunner) handleExecuteCode(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	language, ok := args["language"].(string)
	if !ok {
		language = "python3"
	}

	code, ok := args["code"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required argument: code")
	}

	// Write code to temp file
	tmpFile, err := os.CreateTemp("", "ace-tool-*."+languageExt(language))
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(code); err != nil {
		return nil, err
	}
	tmpFile.Close()

	// Execute
	cmd := exec.CommandContext(ctx, language, tmpFile.Name())
	cmd.Env = os.Environ()
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return map[string]interface{}{
			"error":   err.Error(),
			"output":  string(output),
			"success": false,
		}, nil
	}

	return map[string]interface{}{
		"output":  string(output),
		"success": true,
	}, nil
}

func (r *ToolRunner) handleCalculator(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	expr, ok := args["expression"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required argument: expression")
	}

	// Very basic calculator using Python
	cmd := exec.CommandContext(ctx, "python3", "-c", fmt.Sprintf("print(%s)", expr))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("calculation error: %v - %s", err, string(output))
	}

	return map[string]interface{}{
		"expression": expr,
		"result":     strings.TrimSpace(string(output)),
	}, nil
}

func (r *ToolRunner) handleFileRead(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	path, ok := args["path"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required argument: path")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"path":    path,
		"content": string(data[:min(int64(len(data)), r.config.MaxOutputSize)]),
		"size":    len(data),
	}, nil
}

func languageExt(lang string) string {
	switch lang {
	case "python", "python3":
		return "py"
	case "javascript", "node":
		return "js"
	case "bash", "sh":
		return "sh"
	default:
		return "txt"
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
