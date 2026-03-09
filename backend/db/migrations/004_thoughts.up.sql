-- Create thoughts table
CREATE TABLE thoughts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    layer VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_thoughts_session_id ON thoughts(session_id);
CREATE INDEX idx_thoughts_layer ON thoughts(layer);
CREATE INDEX idx_thoughts_created_at ON thoughts(created_at);
