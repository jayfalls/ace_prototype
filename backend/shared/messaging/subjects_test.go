package messaging

import (
	"testing"
)

func TestSubjectFormat(t *testing.T) {
	tests := []struct {
		name     Subject
		args     []interface{}
		expected string
	}{
		{
			name:     SubjectEngineLayerInput,
			args:     []interface{}{"agent-1", "2"},
			expected: "ace.engine.agent-1.layer.2.input",
		},
		{
			name:     SubjectEngineLayerOutput,
			args:     []interface{}{"agent-1", "3"},
			expected: "ace.engine.agent-1.layer.3.output",
		},
		{
			name:     SubjectEngineLoopStatus,
			args:     []interface{}{"agent-1", "main"},
			expected: "ace.engine.agent-1.loop.main.status",
		},
		{
			name:     SubjectMemoryStore,
			args:     []interface{}{"agent-1"},
			expected: "ace.memory.agent-1.store",
		},
		{
			name:     SubjectMemoryQuery,
			args:     []interface{}{"agent-1"},
			expected: "ace.memory.agent-1.query",
		},
		{
			name:     SubjectMemoryResult,
			args:     []interface{}{"agent-1"},
			expected: "ace.memory.agent-1.result",
		},
		{
			name:     SubjectToolsInvoke,
			args:     []interface{}{"agent-1", "browse"},
			expected: "ace.tools.agent-1.browse.invoke",
		},
		{
			name:     SubjectToolsResult,
			args:     []interface{}{"agent-1", "browse"},
			expected: "ace.tools.agent-1.browse.result",
		},
		{
			name:     SubjectSensesEvent,
			args:     []interface{}{"agent-1", "chat"},
			expected: "ace.senses.agent-1.chat.event",
		},
		{
			name:     SubjectLLMRequest,
			args:     []interface{}{"agent-1"},
			expected: "ace.llm.agent-1.request",
		},
		{
			name:     SubjectLLMResponse,
			args:     []interface{}{"agent-1"},
			expected: "ace.llm.agent-1.response",
		},
		{
			name:     SubjectUsageToken,
			args:     []interface{}{"agent-1"},
			expected: "ace.usage.agent-1.token",
		},
		{
			name:     SubjectUsageCost,
			args:     []interface{}{"agent-1"},
			expected: "ace.usage.agent-1.cost",
		},
		{
			name:     SubjectSystemAgentsSpawn,
			args:     []interface{}{},
			expected: "ace.system.agents.spawn",
		},
		{
			name:     SubjectSystemAgentsShutdown,
			args:     []interface{}{},
			expected: "ace.system.agents.shutdown",
		},
		{
			name:     SubjectSystemHealth,
			args:     []interface{}{"api"},
			expected: "ace.system.health.api",
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.name), func(t *testing.T) {
			result := tt.name.Format(tt.args...)
			if result != tt.expected {
				t.Errorf("Format(%v) = %q, want %q", tt.args, result, tt.expected)
			}
		})
	}
}

func TestSubjectValidate(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		wantErr bool
	}{
		// Engine subjects
		{"engine layer input", SubjectEngineLayerInput.Format("agent-1", "2"), false},
		{"engine layer output", SubjectEngineLayerOutput.Format("agent-1", "3"), false},
		{"engine loop status", SubjectEngineLoopStatus.Format("agent-1", "main"), false},
		
		// Memory subjects
		{"memory store", SubjectMemoryStore.Format("agent-1"), false},
		{"memory query", SubjectMemoryQuery.Format("agent-1"), false},
		{"memory result", SubjectMemoryResult.Format("agent-1"), false},
		
		// Tools subjects
		{"tools invoke", SubjectToolsInvoke.Format("agent-1", "browse"), false},
		{"tools result", SubjectToolsResult.Format("agent-1", "browse"), false},
		
		// Senses subjects
		{"senses event", SubjectSensesEvent.Format("agent-1", "chat"), false},
		
		// LLM subjects
		{"llm request", SubjectLLMRequest.Format("agent-1"), false},
		{"llm response", SubjectLLMResponse.Format("agent-1"), false},
		
		// Usage subjects
		{"usage token", SubjectUsageToken.Format("agent-1"), false},
		{"usage cost", SubjectUsageCost.Format("agent-1"), false},
		
		// System subjects
		{"system agents spawn", string(SubjectSystemAgentsSpawn), false},
		{"system agents shutdown", string(SubjectSystemAgentsShutdown), false},
		{"system health", SubjectSystemHealth.Format("api"), false},
		
		// Invalid subjects
		{"invalid", "invalid.subject", true},
		{"ace invalid", "ace.invalid", true},
		{"random", "random.subject.name", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Subject(tt.subject).Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
