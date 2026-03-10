package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewServer tests server creation
func TestNewServer(t *testing.T) {
	server := NewServer()
	require.NotNil(t, server)
}

// TestRegisterTool tests tool registration
func TestRegisterTool(t *testing.T) {
	server := NewServer()
	
	tool := &Tool{
		Name:        "test_tool",
		Description: "A test tool",
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return "result", nil
		},
	}
	
	err := server.RegisterTool(tool)
	require.NoError(t, err)
	
	// List tools
	tools := server.ListTools()
	assert.Len(t, tools, 1)
	assert.Equal(t, "test_tool", tools[0].Name)
}

// TestCallTool tests tool execution
func TestCallTool(t *testing.T) {
	server := NewServer()
	
	tool := &Tool{
		Name:        "echo",
		Description: "Echoes input",
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return args["input"], nil
		},
	}
	
	server.RegisterTool(tool)
	
	// Call tool
	result, err := server.CallTool(context.Background(), "echo", map[string]interface{}{"input": "hello"})
	require.NoError(t, err)
	assert.Equal(t, "hello", result)
}

// TestCallToolNotFound tests calling non-existent tool
func TestCallToolNotFound(t *testing.T) {
	server := NewServer()
	
	_, err := server.CallTool(context.Background(), "nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestResources tests resource management
func TestResources(t *testing.T) {
	server := NewServer()
	
	// List empty resources
	resources := server.ListResources()
	assert.NotNil(t, resources)
	
	// Add resource
	resource := &Resource{
		URI:         "test://resource",
		Name:        "Test Resource",
		Description: "A test resource",
		MimeType:    "text/plain",
	}
	
	server.RegisterResource(resource)
	
	// List again
	resources = server.ListResources()
	assert.Len(t, resources, 1)
	assert.Equal(t, "test://resource", resources[0].URI)
}

// TestPrompts tests prompt management
func TestPrompts(t *testing.T) {
	server := NewServer()
	
	// List empty prompts
	prompts := server.ListPrompts()
	assert.NotNil(t, prompts)
	
	// Add prompt
	prompt := &Prompt{
		Name:        "test_prompt",
		Description: "A test prompt",
		Template:    "Hello, {{name}}!",
		Arguments:  []string{"name"},
	}
	
	server.RegisterPrompt(prompt)
	
	// List again
	prompts = server.ListPrompts()
	assert.Len(t, prompts, 1)
	assert.Equal(t, "test_prompt", prompts[0].Name)
}

// TestDefaultTools tests default tool registration
func TestDefaultTools(t *testing.T) {
	server := NewServer()
	DefaultTools(server)
	
	tools := server.ListTools()
	// Should have default tools registered
	assert.GreaterOrEqual(t, len(tools), 1)
}

// TestToolHandlerError tests tool handler errors
func TestToolHandlerError(t *testing.T) {
	server := NewServer()
	
	tool := &Tool{
		Name: "error_tool",
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return nil, assert.AnError
		},
	}
	
	server.RegisterTool(tool)
	
	_, err := server.CallTool(context.Background(), "error_tool", nil)
	assert.Error(t, err)
}
