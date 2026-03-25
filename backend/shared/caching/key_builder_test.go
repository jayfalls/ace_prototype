package caching

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

// TestNewKeyBuilder_ValidInputs verifies that NewKeyBuilder returns a non-nil
// builder when both namespace and agentID are provided.
func TestNewKeyBuilder_ValidInputs(t *testing.T) {
	kb := NewKeyBuilder("ns", "agent")
	if kb == nil {
		t.Fatal("expected non-nil builder, got nil")
	}
}

// TestNewKeyBuilder_EmptyNamespace verifies that Build returns ErrInvalidKey
// when namespace is empty.
func TestNewKeyBuilder_EmptyNamespace(t *testing.T) {
	kb := NewKeyBuilder("", "agent1")
	_, err := kb.Build()
	if err == nil {
		t.Fatal("expected error for empty namespace, got nil")
	}
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got: %v", err)
	}
}

// TestNewKeyBuilder_EmptyAgentID verifies that Build returns ErrAgentIDMissing
// when agentID is empty.
func TestNewKeyBuilder_EmptyAgentID(t *testing.T) {
	kb := NewKeyBuilder("ns", "")
	_, err := kb.Build()
	if err == nil {
		t.Fatal("expected error for empty agentID, got nil")
	}
	if !errors.Is(err, ErrAgentIDMissing) {
		t.Fatalf("expected ErrAgentIDMissing, got: %v", err)
	}
}

// TestBuild_ColonInAgentID verifies that Build returns ErrInvalidKey
// when agentID contains a colon.
func TestBuild_ColonInAgentID(t *testing.T) {
	kb := NewKeyBuilder("ns", "agent:1")
	_, err := kb.Build()
	if err == nil {
		t.Fatal("expected error for colon in agentID, got nil")
	}
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got: %v", err)
	}
}

// TestBuild_ColonInEntityType verifies that Build returns ErrInvalidKey
// when entityType contains a colon.
func TestBuild_ColonInEntityType(t *testing.T) {
	kb := NewKeyBuilder("ns", "agent1").EntityType("et:bad")
	_, err := kb.Build()
	if err == nil {
		t.Fatal("expected error for colon in entityType, got nil")
	}
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got: %v", err)
	}
}

// TestBuild_ColonInEntityID verifies that Build returns ErrInvalidKey
// when entityID contains a colon.
func TestBuild_ColonInEntityID(t *testing.T) {
	kb := NewKeyBuilder("ns", "agent1").EntityID("id:bad")
	_, err := kb.Build()
	if err == nil {
		t.Fatal("expected error for colon in entityID, got nil")
	}
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got: %v", err)
	}
}

// TestBuild_ColonInNamespace verifies that Build returns ErrInvalidKey
// when namespace contains a colon.
func TestBuild_ColonInNamespace(t *testing.T) {
	kb := NewKeyBuilder("ns:bad", "agent")
	_, err := kb.Build()
	if err == nil {
		t.Fatal("expected error for colon in namespace, got nil")
	}
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got: %v", err)
	}
}

// TestBuild_FullKey verifies that Build produces the correct key format
// when all components are set.
func TestBuild_FullKey(t *testing.T) {
	kb := NewKeyBuilder("ns", "agent1").
		EntityType("task").
		EntityID("123").
		Version("v1")

	key, err := kb.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "ns:agent1:task:123:v1"
	if key != expected {
		t.Fatalf("expected key %q, got %q", expected, key)
	}
}

// TestBuild_KeyTooLong verifies that Build returns ErrInvalidKey
// when the resulting key exceeds MaxKeyLength bytes.
func TestBuild_KeyTooLong(t *testing.T) {
	longComponent := strings.Repeat("a", 300)
	kb := NewKeyBuilder(longComponent, longComponent).
		EntityType(longComponent).
		EntityID(longComponent).
		Version(longComponent)

	_, err := kb.Build()
	if err == nil {
		t.Fatal("expected error for key too long, got nil")
	}
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got: %v", err)
	}
}

// TestPattern_NoComponents verifies that Pattern returns a wildcard pattern
// when no optional components are set.
func TestPattern_NoComponents(t *testing.T) {
	kb := NewKeyBuilder("ns", "agent1")

	pattern, err := kb.Pattern()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "ns:agent1:*:*:*"
	if pattern != expected {
		t.Fatalf("expected pattern %q, got %q", expected, pattern)
	}
}

// TestPattern_EntityTypeOnly verifies that Pattern returns a wildcard pattern
// with entityType set and other optional components as wildcards.
func TestPattern_EntityTypeOnly(t *testing.T) {
	kb := NewKeyBuilder("ns", "agent1").
		EntityType("task")

	pattern, err := kb.Pattern()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "ns:agent1:task:*:*"
	if pattern != expected {
		t.Fatalf("expected pattern %q, got %q", expected, pattern)
	}
}

// TestPattern_AllComponentsSet verifies that Pattern returns the full key
// when all components are set (no wildcards).
func TestPattern_AllComponentsSet(t *testing.T) {
	kb := NewKeyBuilder("ns", "agent").
		EntityType("et").
		EntityID("id").
		Version("v1")

	pattern, err := kb.Pattern()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "ns:agent:et:id:v1"
	if pattern != expected {
		t.Fatalf("expected pattern %q, got %q", expected, pattern)
	}
}

// TestPattern_EmptyNamespace verifies that Pattern returns ErrInvalidKey
// when namespace is empty.
func TestPattern_EmptyNamespace(t *testing.T) {
	kb := NewKeyBuilder("", "agent")
	_, err := kb.Pattern()
	if err == nil {
		t.Fatal("expected error for empty namespace, got nil")
	}
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got: %v", err)
	}
}

// TestPattern_EmptyAgentID verifies that Pattern returns ErrAgentIDMissing
// when agentID is empty.
func TestPattern_EmptyAgentID(t *testing.T) {
	kb := NewKeyBuilder("ns", "")
	_, err := kb.Pattern()
	if err == nil {
		t.Fatal("expected error for empty agentID, got nil")
	}
	if !errors.Is(err, ErrAgentIDMissing) {
		t.Fatalf("expected ErrAgentIDMissing, got: %v", err)
	}
}

// TestBuild_PartialKey_NoEntityType verifies that Build produces the correct key
// when entityType is omitted but entityID is set.
func TestBuild_PartialKey_NoEntityType(t *testing.T) {
	kb := NewKeyBuilder("ns", "agent1").EntityID("123")

	key, err := kb.Build()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "ns:agent1::123:"
	if key != expected {
		t.Fatalf("expected key %q, got %q", expected, key)
	}
}

// TestBuild_ChainingOrder verifies that different method call orders
// produce the same key result.
func TestBuild_ChainingOrder(t *testing.T) {
	// Order 1: EntityType → EntityID → Version
	kb1 := NewKeyBuilder("ns", "agent1").
		EntityType("task").
		EntityID("123").
		Version("v1")

	// Order 2: Version → EntityType → EntityID
	kb2 := NewKeyBuilder("ns", "agent1").
		Version("v1").
		EntityType("task").
		EntityID("123")

	// Order 3: EntityID → Version → EntityType
	kb3 := NewKeyBuilder("ns", "agent1").
		EntityID("123").
		Version("v1").
		EntityType("task")

	key1, err := kb1.Build()
	if err != nil {
		t.Fatalf("unexpected error from kb1: %v", err)
	}
	key2, err := kb2.Build()
	if err != nil {
		t.Fatalf("unexpected error from kb2: %v", err)
	}
	key3, err := kb3.Build()
	if err != nil {
		t.Fatalf("unexpected error from kb3: %v", err)
	}

	expected := "ns:agent1:task:123:v1"
	if key1 != expected {
		t.Fatalf("kb1: expected %q, got %q", expected, key1)
	}
	if key2 != expected {
		t.Fatalf("kb2: expected %q, got %q", expected, key2)
	}
	if key3 != expected {
		t.Fatalf("kb3: expected %q, got %q", expected, key3)
	}
}

// TestBuild_UTF8Validation verifies that Build returns ErrInvalidKey
// when any component contains non-UTF8 bytes.
func TestBuild_UTF8Validation(t *testing.T) {
	invalidUTF8 := "\x80\x81\x82"
	if utf8.ValidString(invalidUTF8) {
		t.Fatal("test setup error: expected invalid UTF-8 sequence")
	}

	tests := []struct {
		name string
		kb   *KeyBuilder
	}{
		{
			name: "invalid namespace",
			kb:   NewKeyBuilder(invalidUTF8, "agent1"),
		},
		{
			name: "invalid agentID",
			kb:   NewKeyBuilder("ns", invalidUTF8),
		},
		{
			name: "invalid entityType",
			kb:   NewKeyBuilder("ns", "agent1").EntityType(invalidUTF8),
		},
		{
			name: "invalid entityID",
			kb:   NewKeyBuilder("ns", "agent1").EntityID(invalidUTF8),
		},
		{
			name: "invalid version",
			kb:   NewKeyBuilder("ns", "agent1").Version(invalidUTF8),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.kb.Build()
			if err == nil {
				t.Fatal("expected error for non-UTF8 component, got nil")
			}
			if !errors.Is(err, ErrInvalidKey) {
				t.Fatalf("expected ErrInvalidKey, got: %v", err)
			}
		})
	}
}
