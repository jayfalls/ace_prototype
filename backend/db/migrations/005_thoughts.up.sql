-- Create thoughts table
CREATE TABLE IF NOT EXISTS thoughts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID REFERENCES sessions(id) ON DELETE CASCADE,
    layer VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_thoughts_session_id ON thoughts(session_id);
CREATE INDEX IF NOT EXISTS idx_thoughts_layer ON thoughts(layer);
