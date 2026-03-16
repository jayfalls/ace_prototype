package messaging

import (
	"fmt"
	"regexp"
)

// Pre-compiled regex patterns for subject validation
var (
	prefixRegex = regexp.MustCompile(`^ace\.[a-z]+\.`)

	enginePattern       = regexp.MustCompile(`^ace\.engine\.[^.]+\.(layer|loop)\.[^.]+\.(input|output|status)$`)
	memoryPattern       = regexp.MustCompile(`^ace\.memory\.[^.]+\.(store|query|result)$`)
	toolsPattern        = regexp.MustCompile(`^ace\.tools\.[^.]+\.[^.]+\.(invoke|result)$`)
	sensesPattern       = regexp.MustCompile(`^ace\.senses\.[^.]+\.[^.]+\.event$`)
	llmPattern          = regexp.MustCompile(`^ace\.llm\.[^.]+\.(request|response)$`)
	usagePattern        = regexp.MustCompile(`^ace\.usage\.[^.]+\.(token|cost)$`)
	systemAgentsPattern = regexp.MustCompile(`^ace\.system\.agents\.(spawn|shutdown)$`)
	systemHealthPattern = regexp.MustCompile(`^ace\.system\.health\.[^.]+$`)
)

// Subject represents a NATS subject name.
type Subject string

// Engine subjects
const (
	SubjectEngineLayerInput  Subject = "ace.engine.%s.layer.%s.input"
	SubjectEngineLayerOutput Subject = "ace.engine.%s.layer.%s.output"
	SubjectEngineLoopStatus  Subject = "ace.engine.%s.loop.%s.status"
)

// Memory subjects
const (
	SubjectMemoryStore  Subject = "ace.memory.%s.store"
	SubjectMemoryQuery  Subject = "ace.memory.%s.query"
	SubjectMemoryResult Subject = "ace.memory.%s.result"
)

// Tools subjects
const (
	SubjectToolsInvoke Subject = "ace.tools.%s.%s.invoke"
	SubjectToolsResult Subject = "ace.tools.%s.%s.result"
)

// Senses subjects
const (
	SubjectSensesEvent Subject = "ace.senses.%s.%s.event"
)

// LLM subjects (request/response)
const (
	SubjectLLMRequest  Subject = "ace.llm.%s.request"
	SubjectLLMResponse Subject = "ace.llm.%s.response"
)

// Usage subjects
const (
	SubjectUsageToken Subject = "ace.usage.%s.token"
	SubjectUsageCost  Subject = "ace.usage.%s.cost"
)

// System subjects
const (
	SubjectSystemAgentsSpawn    Subject = "ace.system.agents.spawn"
	SubjectSystemAgentsShutdown Subject = "ace.system.agents.shutdown"
	SubjectSystemHealth         Subject = "ace.system.health.%s"
)

// Format returns the subject with interpolated values.
func (s Subject) Format(args ...interface{}) string {
	return fmt.Sprintf(string(s), args...)
}

// Validate checks if the subject matches expected patterns.
func (s Subject) Validate() error {
	subject := string(s)

	// Check for empty subject
	if subject == "" {
		return &MessagingError{
			Code:    "INVALID_SUBJECT",
			Message: "subject cannot be empty",
		}
	}

	// Check if subject starts with "ace." prefix
	if !prefixRegex.MatchString(subject) {
		return &MessagingError{
			Code:    "INVALID_SUBJECT",
			Message: "subject must start with 'ace.<domain>.'",
		}
	}

	// Validate specific subject patterns using pre-compiled regex
	patterns := []struct {
		regex   *regexp.Regexp
		message string
	}{
		{enginePattern, "engine subject invalid format"},
		{memoryPattern, "memory subject invalid format"},
		{toolsPattern, "tools subject invalid format"},
		{sensesPattern, "senses subject invalid format"},
		{llmPattern, "llm subject invalid format"},
		{usagePattern, "usage subject invalid format"},
		{systemAgentsPattern, "system agents subject invalid format"},
		{systemHealthPattern, "system health subject invalid format"},
	}

	for _, p := range patterns {
		if p.regex.MatchString(subject) {
			return nil
		}
	}

	return &MessagingError{
		Code:    "INVALID_SUBJECT",
		Message: fmt.Sprintf("subject '%s' does not match any known pattern", subject),
	}
}
