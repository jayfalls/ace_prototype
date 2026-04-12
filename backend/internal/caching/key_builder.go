package caching

import (
	"strings"
	"unicode/utf8"
)

// MaxKeyLength is the maximum allowed length for a cache key in bytes.
const MaxKeyLength = 1024

// EntityType sets the entityType component and returns the builder for chaining.
func (kb *KeyBuilder) EntityType(t string) *KeyBuilder {
	kb.entityType = t
	return kb
}

// EntityID sets the entityID component and returns the builder for chaining.
func (kb *KeyBuilder) EntityID(id string) *KeyBuilder {
	kb.entityID = id
	return kb
}

// Version sets the version component and returns the builder for chaining.
func (kb *KeyBuilder) Version(v string) *KeyBuilder {
	kb.version = v
	return kb
}

// Build constructs the final cache key string.
// Format: {namespace}:{agentId}:{entityType}:{entityId}:{version}
// Returns ErrInvalidKey if namespace is empty, any component contains a colon,
// or the resulting key exceeds MaxKeyLength bytes.
// Returns ErrAgentIDMissing if agentID is empty.
func (kb *KeyBuilder) Build() (string, error) {
	if kb.namespace == "" {
		return "", ErrInvalidKey
	}
	if kb.agentID == "" {
		return "", ErrAgentIDMissing
	}

	// Check that no component contains a colon and all are valid UTF-8
	components := []string{kb.namespace, kb.agentID, kb.entityType, kb.entityID, kb.version}
	for _, comp := range components {
		if strings.ContainsRune(comp, ':') {
			return "", ErrInvalidKey
		}
		if !utf8.ValidString(comp) {
			return "", ErrInvalidKey
		}
	}

	key := kb.namespace + ":" + kb.agentID + ":" + kb.entityType + ":" + kb.entityID + ":" + kb.version

	if len(key) > MaxKeyLength {
		return "", ErrInvalidKey
	}

	return key, nil
}

// Pattern constructs a pattern string for cache key matching.
// Unset optional components become '*'.
// Returns ErrInvalidKey if namespace is empty, any component contains a colon,
// or the resulting pattern exceeds MaxKeyLength bytes.
// Returns ErrAgentIDMissing if agentID is empty.
func (kb *KeyBuilder) Pattern() (string, error) {
	if kb.namespace == "" {
		return "", ErrInvalidKey
	}
	if kb.agentID == "" {
		return "", ErrAgentIDMissing
	}

	// Check that no component contains a colon and all are valid UTF-8
	components := []string{kb.namespace, kb.agentID, kb.entityType, kb.entityID, kb.version}
	for _, comp := range components {
		if strings.ContainsRune(comp, ':') {
			return "", ErrInvalidKey
		}
		if !utf8.ValidString(comp) {
			return "", ErrInvalidKey
		}
	}

	// Replace empty optional components with wildcard
	entityType := kb.entityType
	if entityType == "" {
		entityType = "*"
	}
	entityID := kb.entityID
	if entityID == "" {
		entityID = "*"
	}
	version := kb.version
	if version == "" {
		version = "*"
	}

	pattern := kb.namespace + ":" + kb.agentID + ":" + entityType + ":" + entityID + ":" + version

	if len(pattern) > MaxKeyLength {
		return "", ErrInvalidKey
	}

	return pattern, nil
}
