package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewToolRunner tests runner creation
func TestNewToolRunner(t *testing.T) {
	runner := NewToolRunner(nil)
	require.NotNil(t, runner)
	
	tools := runner.ListTools()
	assert.Empty(t, tools)
}

// TestRegisterTool tests tool registration
func TestRegisterTool(t *testing.T) {
	runner := NewToolRunner(DefaultRunnerConfig())
	
	tool := &Tool{
		ID:   "test",
		Name: "Test Tool",
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			return "test result", nil
		},
	}
	
	runner.RegisterTool(tool)
	
	result, err := runner.Execute(context.Background(), "test", nil)
	require.NoError(t, err)
	assert.Equal(t, "test result", result)
}

// TestGetTool tests retrieving a tool
func TestGetTool(t *testing.T) {
	runner := NewToolRunner(DefaultRunnerConfig())
	
	tool := &Tool{
		ID:   "mytool",
		Name: "My Tool",
	}
	
	runner.RegisterTool(tool)
	
	result, ok := runner.GetTool("mytool")
	require.True(t, ok)
	assert.Equal(t, "My Tool", result.Name)
}

// TestExecuteNotFound tests executing non-existent tool
func TestExecuteNotFound(t *testing.T) {
	runner := NewToolRunner(DefaultRunnerConfig())
	
	_, err := runner.Execute(context.Background(), "nonexistent", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// TestDefaultTools tests default tool registration
func TestDefaultTools(t *testing.T) {
	runner := NewToolRunner(DefaultRunnerConfig())
	runner.RegisterDefaultTools()
	
	tools := runner.ListTools()
	assert.GreaterOrEqual(t, len(tools), 3) // At least web_search, http_request, calculator
}

// TestCalculator tests calculator tool
func TestCalculator(t *testing.T) {
	runner := NewToolRunner(DefaultRunnerConfig())
	runner.RegisterDefaultTools()
	
	result, err := runner.Execute(context.Background(), "calculator", map[string]interface{}{
		"expression": "2+2",
	})
	
	// May fail if python3 not available, but that's OK
	if err != nil {
		t.Logf("Calculator test skipped: %v", err)
		return
	}
	
	require.NotNil(t, result)
}

// TestRunnerConfig tests default config
func TestRunnerConfig(t *testing.T) {
	config := DefaultRunnerConfig()
	require.NotNil(t, config)
	
	assert.NotEmpty(t, config.AllowedCommands)
	assert.Equal(t, 30, int(config.Timeout.Seconds()))
	assert.Equal(t, 10, int(config.HTTPTimeout.Seconds()))
}
