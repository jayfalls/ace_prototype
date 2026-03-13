-- Create health_check table for SQLC demo
-- This table is used to demonstrate SQLC query generation

CREATE TABLE IF NOT EXISTS health_check (
    id SERIAL PRIMARY KEY,
    status VARCHAR(50) NOT NULL DEFAULT 'healthy',
    message TEXT,
    checked_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Insert initial health check record
INSERT INTO health_check (status, message, checked_at)
VALUES ('healthy', 'System is operational', NOW())
ON CONFLICT DO NOTHING;

-- Create index for faster queries
CREATE INDEX IF NOT EXISTS idx_health_check_checked_at ON health_check(checked_at DESC);
