package services

import (
	"context"
	"fmt"

	"github.com/ace/framework/backend/internal/db"
	"github.com/google/uuid"
)

// MemoryTreeNode represents a node in the memory tree
type MemoryTreeNode struct {
	Memory   *db.Memory
	Children []*MemoryTreeNode
	Depth    int
}

// MemoryService handles memory operations including tree search
type MemoryService struct {
	db db.Database
}

// NewMemoryService creates a new memory service
func NewMemoryService(database db.Database) *MemoryService {
	return &MemoryService{db: database}
}

// SearchOptions for memory search
type SearchOptions struct {
	Query      string
	Tags       []string
	MemoryType string
	MinImportance float64
	MaxDepth   int
	Limit      int
}

// GetMemoryTree retrieves the full memory tree for an agent
func (s *MemoryService) GetMemoryTree(ctx context.Context, agentID string, maxDepth int) (*MemoryTreeNode, error) {
	// Get all memories for the agent
	memories, err := s.db.GetMemoriesByAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get memories: %w", err)
	}

	// Build tree structure
	memoryMap := make(map[string]*MemoryTreeNode)
	var rootNodes []*MemoryTreeNode

	// First pass: create all nodes
	for i := range memories {
		m := &memories[i]
		node := &MemoryTreeNode{
			Memory:   m,
			Children: []*MemoryTreeNode{},
			Depth:    0,
		}
		memoryMap[m.ID] = node
	}

	// Second pass: build tree
	for _, node := range memoryMap {
		parentID := node.Memory.ParentID
		if parentID == "" {
			// Root node
			rootNodes = append(rootNodes, node)
		} else if parent, ok := memoryMap[parentID]; ok {
			// Add as child
			parent.Children = append(parent.Children, node)
			node.Depth = parent.Depth + 1
		} else {
			// Orphan - treat as root
			rootNodes = append(rootNodes, node)
		}
	}

	// Filter by max depth
	if maxDepth > 0 {
		filterDepth(rootNodes, maxDepth)
	}

	// Return combined root
	if len(rootNodes) == 0 {
		return nil, nil
	}
	if len(rootNodes) == 1 {
		return rootNodes[0], nil
	}

	// Multiple roots - combine
	return &MemoryTreeNode{
		Memory: &db.Memory{
			ID:       uuid.Nil.String(),
			AgentID:  agentID,
			Content:  "root",
			Type:     "root",
		},
		Children: rootNodes,
		Depth:    0,
	}, nil
}

// filterDepth removes nodes beyond max depth
func filterDepth(nodes []*MemoryTreeNode, maxDepth int) {
	for _, node := range nodes {
		if node.Depth >= maxDepth {
			node.Children = nil
		} else if len(node.Children) > 0 {
			filterDepth(node.Children, maxDepth)
		}
	}
}

// SearchMemoriesTree searches memories using tree traversal
func (s *MemoryService) SearchMemoriesTree(ctx context.Context, agentID string, opts SearchOptions) ([]*db.Memory, error) {
	// Get all memories for the agent
	allMemories, err := s.db.GetMemoriesByAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get memories: %w", err)
	}

	// Get root nodes for tree traversal
	tree, err := s.GetMemoryTree(ctx, agentID, opts.MaxDepth)
	if err != nil {
		// Fall back to flat search
		return s.searchFlat(ctx, agentID, opts)
	}

	// Collect matching memories via BFS
	results := make([]*db.Memory, 0)
	queue := []*MemoryTreeNode{tree}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		// Check if this memory matches
		if matches(node.Memory, opts) {
			results = append(results, node.Memory)
			if opts.Limit > 0 && len(results) >= opts.Limit {
				return results, nil
			}
		}

		// Add children to queue
		for _, child := range node.Children {
			queue = append(queue, child)
		}
	}

	// If no tree results, fall back to flat search
	if len(results) == 0 {
		return s.searchFlat(ctx, agentID, opts)
	}

	return results, nil
}

// matches checks if a memory matches search options
func matches(memory *db.Memory, opts SearchOptions) bool {
	// Check memory type
	if opts.MemoryType != "" && memory.Type != opts.MemoryType {
		return false
	}

	// Check importance
	if opts.MinImportance > 0 && memory.Importance < opts.MinImportance {
		return false
	}

	// Check tags
	if len(opts.Tags) > 0 {
		matched := false
		for _, wanted := range opts.Tags {
			for _, have := range memory.Tags {
				if wanted == have {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check query (content search)
	if opts.Query != "" {
		// Simple contains check - could use more sophisticated matching
		found := false
		searchIn := []string{memory.Content}
		for _, s := range searchIn {
			if containsCI(s, opts.Query) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// searchFlat provides fallback flat search
func (s *MemoryService) searchFlat(ctx context.Context, agentID string, opts SearchOptions) ([]*db.Memory, error) {
	var results []*db.Memory

	if opts.Query != "" {
		memories, err := s.db.SearchMemories(ctx, agentID, opts.Query)
		if err != nil {
			return nil, err
		}
		results = memories
	} else {
		all, err := s.db.GetMemoriesByAgent(ctx, agentID)
		if err != nil {
			return nil, err
		}
		results = all
	}

	// Apply filters
	filtered := make([]*db.Memory, 0)
	for _, m := range results {
		if matches(m, opts) {
			filtered = append(filtered, m)
			if opts.Limit > 0 && len(filtered) >= opts.Limit {
				break
			}
		}
	}

	return filtered, nil
}

// GetMemoryPath returns the path from root to a specific memory
func (s *MemoryService) GetMemoryPath(ctx context.Context, memoryID string) ([]*db.Memory, error) {
	path := make([]*db.Memory, 0)
	currentID := memoryID

	for {
		memory, err := s.db.GetMemory(ctx, currentID)
		if err != nil {
			return nil, err
		}

		// Insert at beginning (root first)
		path = append([]*db.Memory{memory}, path...)

		if memory.ParentID == "" {
			break
		}
		currentID = memory.ParentID
	}

	return path, nil
}

// GetMemoryChildren returns direct children of a memory
func (s *MemoryService) GetMemoryChildren(ctx context.Context, memoryID string) ([]*db.Memory, error) {
	allMemories, err := s.db.GetMemoriesByAgent(ctx, "")
	if err != nil {
		return nil, err
	}

	children := make([]*db.Memory, 0)
	for _, m := range allMemories {
		if m.ParentID == memoryID {
			children = append(children, &m)
		}
	}

	return children, nil
}

// GetDescendantCount returns the number of descendants
func (s *MemoryService) GetDescendantCount(ctx context.Context, memoryID string) (int, error) {
	tree, err := s.GetMemoryTree(ctx, "", 0)
	if err != nil {
		return 0, err
	}

	// Find the node
	var count int
	var findAndCount func(node *MemoryTreeNode, targetID string)
	findAndCount = func(node *MemoryTreeNode, targetID string) {
		if node.Memory.ID == targetID {
			count = countChildren(node)
			return
		}
		for _, child := range node.Children {
			findAndCount(child, targetID)
		}
	}

	findAndCount(tree, memoryID)
	return count, nil
}

func countChildren(node *MemoryTreeNode) int {
	count := len(node.Children)
	for _, child := range node.Children {
		count += countChildren(child)
	}
	return count
}

func containsCI(s, substr string) bool {
	s = toLower(s)
	substr = toLower(substr)
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}
