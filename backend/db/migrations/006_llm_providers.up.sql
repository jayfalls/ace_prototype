-- Create llm_providers table
CREATE TABLE llm_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    provider_type VARCHAR(50) NOT NULL,
    api_key_encrypted VARCHAR(512),
    base_url VARCHAR(512),
    model VARCHAR(100),
    config JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_llm_providers_owner_id ON llm_providers(owner_id);
CREATE INDEX idx_llm_providers_provider_type ON llm_providers(provider_type);

-- Create llm_attachments table
CREATE TABLE llm_attachments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    provider_id UUID NOT NULL REFERENCES llm_providers(id) ON DELETE CASCADE,
    layer VARCHAR(50) NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    config JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_llm_attachments_agent_id ON llm_attachments(agent_id);
CREATE INDEX idx_llm_attachments_provider_id ON llm_attachments(provider_id);
CREATE INDEX idx_llm_attachments_layer ON llm_attachments(layer);
