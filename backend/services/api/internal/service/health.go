// Package service provides business logic for the API service.
package service

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	queries "ace/api/internal/repository/generated"
)

// HealthService handles health check business logic.
type HealthService struct {
	queries *queries.Queries
	pool    *pgxpool.Pool
}

// NewHealthService creates a new health service.
func NewHealthService(queries *queries.Queries, pool *pgxpool.Pool) *HealthService {
	return &HealthService{
		queries: queries,
		pool:    pool,
	}
}

// HealthStatus represents the current health status.
type HealthStatus struct {
	DB      string
	Err     string
	Created time.Time
}

// DBHealthCheck verifies database connectivity.
func (s *HealthService) DBHealthCheck(ctx context.Context) error {
	if s.pool == nil {
		return nil // No pool, skip check
	}
	return s.pool.Ping(ctx)
}

// GetHealth returns the current health status.
func (s *HealthService) GetHealth(ctx context.Context) (*HealthStatus, error) {
	latestHealth, err := s.queries.GetLatestHealthCheck(ctx)
	if err != nil {
		return nil, err
	}

	return &HealthStatus{
		DB:      latestHealth.Db,
		Err:     latestHealth.Err.String,
		Created: latestHealth.Created.Time,
	}, nil
}

// EnsureHealthRecord ensures there's a health check record.
func (s *HealthService) EnsureHealthRecord(ctx context.Context) (*HealthStatus, error) {
	latestHealth, err := s.queries.GetLatestHealthCheck(ctx)
	if err == nil {
		return &HealthStatus{
			DB:      latestHealth.Db,
			Err:     latestHealth.Err.String,
			Created: latestHealth.Created.Time,
		}, nil
	}

	// Create new record if none exists
	newHealth, createErr := s.queries.CreateHealthCheck(ctx, queries.CreateHealthCheckParams{
		Db:  "healthy",
		Err: pgtype.Text{Valid: false},
	})
	if createErr != nil {
		// Record may already exist (race condition with migration)
		// Try to fetch again
		latestHealth, fetchErr := s.queries.GetLatestHealthCheck(ctx)
		if fetchErr != nil {
			return nil, createErr
		}
		return &HealthStatus{
			DB:      latestHealth.Db,
			Err:     latestHealth.Err.String,
			Created: latestHealth.Created.Time,
		}, nil
	}

	return &HealthStatus{
		DB:      newHealth.Db,
		Err:     newHealth.Err.String,
		Created: newHealth.Created.Time,
	}, nil
}

// CreateHealthCheck creates a new health check record with the given status.
func (s *HealthService) CreateHealthCheck(ctx context.Context, dbStatus string, errMsg *string) (*HealthStatus, error) {
	var errPgType pgtype.Text
	if errMsg != nil {
		errPgType = pgtype.Text{String: *errMsg, Valid: true}
	} else {
		errPgType = pgtype.Text{Valid: false}
	}

	newHealth, err := s.queries.CreateHealthCheck(ctx, queries.CreateHealthCheckParams{
		Db:  dbStatus,
		Err: errPgType,
	})
	if err != nil {
		return nil, err
	}

	return &HealthStatus{
		DB:      newHealth.Db,
		Err:     newHealth.Err.String,
		Created: newHealth.Created.Time,
	}, nil
}

// ListHealthChecks returns the health check history.
func (s *HealthService) ListHealthChecks(ctx context.Context, limit int32) ([]HealthStatus, error) {
	healthChecks, err := s.queries.ListHealthChecks(ctx, limit)
	if err != nil {
		return nil, err
	}

	result := make([]HealthStatus, len(healthChecks))
	for i, hc := range healthChecks {
		result[i] = HealthStatus{
			DB:      hc.Db,
			Err:     hc.Err.String,
			Created: hc.Created.Time,
		}
	}

	return result, nil
}
