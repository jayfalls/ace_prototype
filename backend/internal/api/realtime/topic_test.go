package realtime

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// testableTopicReg provides a testable TopicReg without NATS
type testableTopicReg struct {
	mu             sync.Mutex
	refs           map[string]int
	subs           map[string]bool // using bool instead of *nats.Subscription for testing
	topicToSubject map[string]string
	subjectToTopic map[string]string
	logger         *zap.Logger
}

func newTestableTopicReg(logger *zap.Logger) *testableTopicReg {
	topicToSubject := map[string]string{
		"agent:{id}:status": "ace.engine.{id}.layer.>",
		"agent:{id}:logs":   "ace.engine.{id}.loop.>",
		"agent:{id}:cycles": "ace.engine.{id}.layer.6.output",
		"system:health":     "ace.system.health.>",
		"usage:{id}":        "ace.usage.{id}.>",
	}

	subjectToTopic := make(map[string]string)
	for topic, subject := range topicToSubject {
		subjectToTopic[subject] = topic
	}

	return &testableTopicReg{
		refs:           make(map[string]int),
		subs:           make(map[string]bool),
		topicToSubject: topicToSubject,
		subjectToTopic: subjectToTopic,
		logger:         logger,
	}
}

// addRefCount simulates Add without actual NATS subscription
func (t *testableTopicReg) addRefCount(topic string) error {
	if err := ValidateTopic(topic); err != nil {
		return err
	}
	_, err := t.topicToSubjectFunc(topic)
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.refs[topic]++
	if t.refs[topic] == 1 {
		t.subs[topic] = true
	}
	return nil
}

// removeRefCount simulates Remove without actual NATS subscription
func (t *testableTopicReg) removeRefCount(topic string) error {
	if err := ValidateTopic(topic); err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.refs[topic] == 0 {
		return nil
	}
	t.refs[topic]--
	if t.refs[topic] == 0 {
		delete(t.refs, topic)
		delete(t.subs, topic)
	}
	return nil
}

// refCount returns the current reference count for a topic
func (t *testableTopicReg) refCount(topic string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.refs[topic]
}

// isSubscribed returns whether a topic has an active subscription
func (t *testableTopicReg) isSubscribed(topic string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.subs[topic]
}

// topicToSubjectFunc converts a public topic to a NATS subject pattern
func (t *testableTopicReg) topicToSubjectFunc(topic string) (string, error) {
	if subject, ok := t.topicToSubject[topic]; ok {
		return subject, nil
	}
	if len(topic) > 6 && topic[:6] == "agent:" {
		parts := splitTopic(topic)
		if len(parts) == 3 {
			id, subType := parts[1], parts[2]
			switch subType {
			case "status":
				return "ace.engine." + id + ".layer.>", nil
			case "logs":
				return "ace.engine." + id + ".loop.>", nil
			case "cycles":
				return "ace.engine." + id + ".layer.6.output", nil
			}
		}
	}
	if topic == "system:health" {
		return "ace.system.health.>", nil
	}
	if topic[:6] == "usage:" {
		parts := splitTopic(topic)
		if len(parts) == 2 {
			return "ace.usage." + parts[1] + ".>", nil
		}
	}
	return "", assert.AnError
}

func splitTopic(topic string) []string {
	var parts []string
	start := 0
	for i := 0; i <= len(topic); i++ {
		if i == len(topic) || topic[i] == ':' {
			parts = append(parts, topic[start:i])
			start = i + 1
		}
	}
	return parts
}

// natsToTopic converts a NATS subject back to a public topic
func (t *testableTopicReg) natsToTopic(subject string) string {
	_ = subject // unused but part of interface
	if topic, ok := t.subjectToTopic[subject]; ok {
		return topic
	}
	return ""
}

// TestTopicReg_Add_IncrementsRefCount tests that Add increments reference count
func TestTopicReg_Add_IncrementsRefCount(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	reg := newTestableTopicReg(logger)

	// First Add
	err := reg.addRefCount("agent:1:status")
	assert.NoError(t, err)
	assert.Equal(t, 1, reg.refCount("agent:1:status"))

	// Second Add should increment ref
	err = reg.addRefCount("agent:1:status")
	assert.NoError(t, err)
	assert.Equal(t, 2, reg.refCount("agent:1:status"))
}

// TestTopicReg_Add_InvalidTopic tests that invalid topics are rejected
func TestTopicReg_Add_InvalidTopic(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	reg := newTestableTopicReg(logger)

	invalidTopics := []string{
		"INVALID",
		"agent:1:Status",
		"agent::status",
		"",
	}

	for _, topic := range invalidTopics {
		err := reg.addRefCount(topic)
		assert.Error(t, err, "expected error for topic: %s", topic)
	}
}

// TestTopicReg_Remove_DecrementsRef tests that Remove decrements reference count
func TestTopicReg_Remove_DecrementsRef(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	reg := newTestableTopicReg(logger)

	// Add twice
	err := reg.addRefCount("agent:1:status")
	assert.NoError(t, err)
	err = reg.addRefCount("agent:1:status")
	assert.NoError(t, err)
	assert.Equal(t, 2, reg.refCount("agent:1:status"))

	// First Remove
	err = reg.removeRefCount("agent:1:status")
	assert.NoError(t, err)
	assert.Equal(t, 1, reg.refCount("agent:1:status"))
}

// TestTopicReg_Remove_ToZero_CleansUp tests that Remove to zero cleans up
func TestTopicReg_Remove_ToZero_CleansUp(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	reg := newTestableTopicReg(logger)

	err := reg.addRefCount("agent:1:status")
	assert.NoError(t, err)
	assert.Equal(t, 1, reg.refCount("agent:1:status"))
	assert.True(t, reg.isSubscribed("agent:1:status"))

	err = reg.removeRefCount("agent:1:status")
	assert.NoError(t, err)
	assert.Equal(t, 0, reg.refCount("agent:1:status"))
	assert.False(t, reg.isSubscribed("agent:1:status"))
}

// TestTopicReg_Remove_NonSubscribed tests removing a topic that was never subscribed
func TestTopicReg_Remove_NonSubscribed(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	reg := newTestableTopicReg(logger)

	err := reg.removeRefCount("agent:1:status")
	assert.NoError(t, err) // remove on zero ref is now a no-op
}

// TestTopicReg_Concurrent_AddRemove tests concurrent Add/Remove operations
func TestTopicReg_Concurrent_AddRemove(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	reg := newTestableTopicReg(logger)

	var wg sync.WaitGroup

	// Add 10 times first
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = reg.addRefCount("agent:1:status")
		}()
	}
	wg.Wait()

	// Then remove 5 times concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = reg.removeRefCount("agent:1:status")
		}()
	}
	wg.Wait()

	assert.Equal(t, 5, reg.refCount("agent:1:status"))
}

// TestTopicReg_NATSToTopic tests the reverse mapping
func TestTopicReg_NATSToTopic(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	reg := newTestableTopicReg(logger)

	// Test that we can map subjects back to topics (template-to-template)
	subject := "ace.engine.{id}.layer.>"
	got := reg.natsToTopic(subject)
	assert.Equal(t, "agent:{id}:status", got, "direct subject mapping should work")
}

// TestTopicReg_RefCount_NonExistent tests ref count for non-existent topics
func TestTopicReg_RefCount_NonExistent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	reg := newTestableTopicReg(logger)

	assert.Equal(t, 0, reg.refCount("nonexistent"))
	assert.False(t, reg.isSubscribed("nonexistent"))
}

// TestTopicReg_ValidTopicMapping tests that all valid topic formats work
func TestTopicReg_ValidTopicMapping(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	reg := newTestableTopicReg(logger)

	validTopics := []string{
		"agent:1:status",
		"agent:abc-123:logs",
		"agent:123:cycles",
		"system:health",
		"usage:user123",
	}

	for _, topic := range validTopics {
		t.Run(topic, func(t *testing.T) {
			err := reg.addRefCount(topic)
			assert.NoError(t, err)
			assert.Greater(t, reg.refCount(topic), 0)
		})
	}
}

// TestTopicReg_Add_UnsupportedTopic tests that unsupported topics error
func TestTopicReg_Add_UnsupportedTopic(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	reg := newTestableTopicReg(logger)

	// This topic is valid format but not in our pattern mapping
	err := reg.addRefCount("agent:123:messages")
	assert.Error(t, err)
}

// TestTopicReg_IsSubscribed tests subscription state
func TestTopicReg_IsSubscribed(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	reg := newTestableTopicReg(logger)

	// Initially not subscribed
	assert.False(t, reg.isSubscribed("agent:1:status"))

	// After Add, subscribed
	_ = reg.addRefCount("agent:1:status")
	assert.True(t, reg.isSubscribed("agent:1:status"))

	// After Remove to zero, not subscribed
	_ = reg.removeRefCount("agent:1:status")
	assert.False(t, reg.isSubscribed("agent:1:status"))
}

// TestTopicReg_Add_IncrementsOnMultipleAdds tests ref counting across multiple adds
func TestTopicReg_Add_IncrementsOnMultipleAdds(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	reg := newTestableTopicReg(logger)

	topics := []string{"agent:1:status", "agent:1:status", "agent:1:status"}

	for i, topic := range topics {
		err := reg.addRefCount(topic)
		assert.NoError(t, err)
		assert.Equal(t, i+1, reg.refCount(topic))
	}
}

// TestTopicReg_Remove_AfterMultipleAdds tests removing after multiple adds
func TestTopicReg_Remove_AfterMultipleAdds(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	reg := newTestableTopicReg(logger)

	// Add 5 times
	for i := 0; i < 5; i++ {
		_ = reg.addRefCount("agent:1:status")
	}
	assert.Equal(t, 5, reg.refCount("agent:1:status"))

	// Remove 3 times
	for i := 0; i < 3; i++ {
		_ = reg.removeRefCount("agent:1:status")
	}
	assert.Equal(t, 2, reg.refCount("agent:1:status"))
	assert.True(t, reg.isSubscribed("agent:1:status"))

	// Remove remaining 2 times
	_ = reg.removeRefCount("agent:1:status")
	_ = reg.removeRefCount("agent:1:status")
	assert.Equal(t, 0, reg.refCount("agent:1:status"))
	assert.False(t, reg.isSubscribed("agent:1:status"))
}

// TestTopicReg_DifferentTopics tests ref counting for different topics independently
func TestTopicReg_DifferentTopics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	reg := newTestableTopicReg(logger)

	_ = reg.addRefCount("agent:1:status")
	_ = reg.addRefCount("agent:1:status")
	_ = reg.addRefCount("agent:2:status")
	_ = reg.addRefCount("system:health")

	assert.Equal(t, 2, reg.refCount("agent:1:status"))
	assert.Equal(t, 1, reg.refCount("agent:2:status"))
	assert.Equal(t, 1, reg.refCount("system:health"))

	_ = reg.removeRefCount("agent:1:status")
	assert.Equal(t, 1, reg.refCount("agent:1:status"))
	assert.Equal(t, 1, reg.refCount("agent:2:status"))
	assert.Equal(t, 1, reg.refCount("system:health"))
}

// TestTopicReg_TopicToSubject tests the topic to NATS subject conversion
func TestTopicReg_TopicToSubject(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	reg := newTestableTopicReg(logger)

	tests := []struct {
		topic       string
		wantSubject string
		wantErr     bool
	}{
		{"agent:1:status", "ace.engine.1.layer.>", false},
		{"agent:abc:logs", "ace.engine.abc.loop.>", false},
		{"agent:123:cycles", "ace.engine.123.layer.6.output", false},
		{"system:health", "ace.system.health.>", false},
		{"usage:user1", "ace.usage.user1.>", false},
	}

	for _, tt := range tests {
		t.Run(tt.topic, func(t *testing.T) {
			subject, err := reg.topicToSubjectFunc(tt.topic)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantSubject, subject)
			}
		})
	}
}

