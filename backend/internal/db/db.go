package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ace/framework/backend/internal/config"
	db "github.com/ace/framework/backend/internal/db/sqlc"
)

var Pool *pgxpool.Pool
var Queries *db.Queries

func Connect(cfg *config.DatabaseConfig) error {
	connStr := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
		cfg.SSLMode,
	)

	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	Pool = pool
	Queries = db.New(pool)
	return nil
}

func GetQueries() *db.Queries {
	return Queries
}

func Disconnect() {
	if Pool != nil {
		Pool.Close()
	}
}
