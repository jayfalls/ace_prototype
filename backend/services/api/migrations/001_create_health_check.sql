-- Health check table for sqlc generation
CREATE TABLE IF NOT EXISTS health_check (
    id SERIAL PRIMARY KEY,
    db VARCHAR(50) NOT NULL DEFAULT 'healthy',
    err TEXT,
    created TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
