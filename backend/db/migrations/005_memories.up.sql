-- Create memories table
CREATE TABLE memories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    agent_id UUID REFERENCES agents(id) ON DELETE SET NULL,
    content TEXT NOT NULL,
    memory_type VARCHAR(50) NOT NULL,
    parent_id UUID REFERENCES memories(id) ON DELETE SET NULL,
    tags TEXT[],
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_memories_owner_id ON memories(owner_id);
CREATE INDEX idx_memories_agent_id ON memories(agent_id);
CREATE INDEX idx_memories_parent_id ON memories(parent_id);
CREATE INDEX idx_memories_memory_type ON memories(memory_type);
CREATE INDEX idx_memories_tags ON memories USING GIN(tags);
