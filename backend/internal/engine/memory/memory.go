package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryType represents the type of memory
type MemoryType int

const (
	MemoryShortTerm MemoryType = iota // Working memory - always injected
	MemoryMediumTerm                  // Session memory - always injected
	MemoryLongTerm                    // Persistent tree memory
)

// MemoryItem represents a single memory entry
type MemoryItem struct {
	ID          uuid.UUID
	Content     string
	MemoryType  MemoryType
	Tags        []string
	Importance  float64   // 0.0 - 1.0
	ParentID    *uuid.UUID // For tree structure
	AgentID     uuid.UUID
	SessionID   *uuid.UUID
	CreatedAt   time.Time
	AccessedAt  time.Time
	AccessCount int
}

// SearchQuery represents a memory search request
type SearchQuery struct {
	Tags       []string
	Content    string // Partial match
	MemoryType *MemoryType
	MinImportance float64
	Limit      int
	Offset     int
}

// Store manages memory operations
type Store interface {
	// Create adds a new memory
	Create(ctx context.Context, item MemoryItem) error
	// Get retrieves a memory by ID
	Get(ctx context.Context, id uuid.UUID) (*MemoryItem, error)
	// Update modifies an existing memory
	Update(ctx context.Context, item MemoryItem) error
	// Delete removes a memory
	Delete(ctx context.Context, id uuid.UUID) error
	// Search finds memories matching query
	Search(ctx context.Context, agentID uuid.UUID, query SearchQuery) ([]MemoryItem, error)
	// GetByParent retrieves children of a memory node
	GetByParent(ctx context.Context, parentID uuid.UUID) ([]MemoryItem, error)
	// GetByTags retrieves memories with specific tags
	GetByTags(ctx context.Context, agentID uuid.UUID, tags []string) ([]MemoryItem, error)
}

// InMemoryStore provides in-memory implementation
type InMemoryStore struct {
	mu      sync.RWMutex
	items   map[uuid.UUID]*MemoryItem
	indexes map[string]map[uuid.UUID]bool // tag -> IDs
}

// NewInMemoryStore creates a memory store
func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		items:   make(map[uuid.UUID]*MemoryItem),
		indexes: make(map[string]map[uuid.UUID]bool),
	}
}

func (s *InMemoryStore) Create(ctx context.Context, item MemoryItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item.ID = uuid.New()
	item.CreatedAt = time.Now()
	item.AccessedAt = time.Now()

	s.items[item.ID] = &item

	// Index by tags
	for _, tag := range item.Tags {
		if s.indexes[tag] == nil {
			s.indexes[tag] = make(map[uuid.UUID]bool)
		}
		s.indexes[tag][item.ID] = true
	}

	return nil
}

func (s *InMemoryStore) Get(ctx context.Context, id uuid.UUID) (*MemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.items[id]
	if !ok {
		return nil, fmt.Errorf("memory not found")
	}

	item.AccessCount++
	item.AccessedAt = time.Now()

	return item, nil
}

func (s *InMemoryStore) Update(ctx context.Context, item MemoryItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.items[item.ID]; !ok {
		return fmt.Errorf("memory not found")
	}

	item.AccessedAt = time.Now()
	s.items[item.ID] = &item
	return nil
}

func (s *InMemoryStore) Delete(ctx context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[id]
	if !ok {
		return fmt.Errorf("memory not found")
	}

	// Remove from indexes
	for _, tag := range item.Tags {
		if s.indexes[tag] != nil {
			delete(s.indexes[tag], id)
		}
	}

	delete(s.items, id)
	return nil
}

func (s *InMemoryStore) Search(ctx context.Context, agentID uuid.UUID, query SearchQuery) ([]MemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]MemoryItem, 0)
	limit := query.Limit
	if limit == 0 {
		limit = 20
	}

	for _, item := range s.items {
		if item.AgentID != agentID {
			continue
		}

		// Filter by memory type
		if query.MemoryType != nil && item.MemoryType != *query.MemoryType {
			continue
		}

		// Filter by importance
		if item.Importance < query.MinImportance {
			continue
		}

		// Filter by content
		if query.Content != "" {
			// Simple substring match
			found := false
			for i := 0; i <= len(item.Content)-len(query.Content); i++ {
				if item.Content[i:i+len(query.Content)] == query.Content {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		results = append(results, *item)

		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

func (s *InMemoryStore) GetByParent(ctx context.Context, parentID uuid.UUID) ([]MemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]MemoryItem, 0)
	for _, item := range s.items {
		if item.ParentID != nil && *item.ParentID == parentID {
			results = append(results, *item)
		}
	}

	return results, nil
}

func (s *InMemoryStore) GetByTags(ctx context.Context, agentID uuid.UUID, tags []string) ([]MemoryItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Find intersection of tag sets
	candidateIDs := make(map[uuid.UUID]bool)
	for i, tag := range tags {
		if ids, ok := s.indexes[tag]; ok {
			if i == 0 {
				for id := range ids {
					candidateIDs[id] = true
				}
			} else {
				newCandidates := make(map[uuid.UUID]bool)
				for id := range candidateIDs {
					if ids[id] {
						newCandidates[id] = true
					}
				}
				candidateIDs = newCandidates
			}
		}
	}

	results := make([]MemoryItem, 0)
	for id := range candidateIDs {
		if item, ok := s.items[id]; ok && item.AgentID == agentID {
			results = append(results, *item)
		}
	}

	return results, nil
}

// Service provides memory management for layers
type Service struct {
	store Store
}

// NewService creates a memory service
func NewService(store Store) *Service {
	return &Service{store: store}
}

// CreateMemory adds a new memory
func (s *Service) CreateMemory(ctx context.Context, item MemoryItem) error {
	return s.store.Create(ctx, item)
}

// GetMemory retrieves a memory
func (s *Service) GetMemory(ctx context.Context, id uuid.UUID) (*MemoryItem, error) {
	return s.store.Get(ctx, id)
}

// SearchMemories finds memories
func (s *Service) SearchMemories(ctx context.Context, agentID uuid.UUID, query SearchQuery) ([]MemoryItem, error) {
	return s.store.Search(ctx, agentID, query)
}

// GetShortTerm returns always-injected memories
func (s *Service) GetShortTerm(ctx context.Context, agentID uuid.UUID) ([]MemoryItem, error) {
	memType := MemoryShortTerm
	return s.store.Search(ctx, agentID, SearchQuery{
		MemoryType: &memType,
		Limit:      10,
	})
}

// GetMediumTerm returns session-injected memories
func (s *Service) GetMediumTerm(ctx context.Context, sessionID uuid.UUID) ([]MemoryItem, error) {
	memType := MemoryMediumTerm
	return s.store.Search(ctx, uuid.Nil, SearchQuery{
		MemoryType: &memType,
		Limit:      20,
	})
}

// GetLongTerm retrieves from tree memory
func (s *Service) GetLongTerm(ctx context.Context, agentID uuid.UUID, tags []string) ([]MemoryItem, error) {
	if len(tags) > 0 {
		return s.store.GetByTags(ctx, agentID, tags)
	}
	// Return recent long-term memories
	memType := MemoryLongTerm
	return s.store.Search(ctx, agentID, SearchQuery{
		MemoryType:  &memType,
		MinImportance: 0.5,
		Limit:      10,
	})
}
