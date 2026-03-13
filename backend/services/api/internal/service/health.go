// Package service provides business logic for the API service.
package service

import (
	"context"
	"time"

	"ace/api/internal/repository/generated"
)

// HealthService handles health check business logic.
type HealthService struct {
	queries *generated.Queries
}

// NewHealthService creates a new health service.
func NewHealthService(queries *generated.Queries) *HealthService {
	return &HealthService{
		queries: queries,
	}
}

// HealthStatus represents the current health status.
type HealthStatus struct {
	Status    string
	Message   string
	CheckedAt time.Time
}

// GetHealth returns the current health status.
func (s *HealthService) GetHealth(ctx context.Context) (*HealthStatus, error) {
	latestHealth, err := s.queries.GetLatestHealthCheck(ctx)
	if err != nil {
		return nil, err
	}

	return &HealthStatus{
		Status:    latestHealth.Status,
		Message:   latestHealth.Message,
		CheckedAt: latestHealth.CheckedAt,
	}, nil
}

// EnsureHealthRecord ensures there's a health check record.
func (s *HealthService) EnsureHealthRecord(ctx context.Context) (*HealthStatus, error) {
	latestHealth, err := s.queries.GetLatestHealthCheck(ctx)
	if err == nil {
		return &HealthStatus{
			Status:    latestHealth.Status,
			Message:   latestHealth.Message,
			CheckedAt: latestHealth.CheckedAt,
		}, nil
	}

	// Create new record if none exists
	newHealth, err := s.queries.CreateHealthCheck(ctx, generated.CreateHealthCheckParams{
		Status:  "healthy",
		Message: "System is operational",
	})
	if err != nil {
		return nil, err
	}

	return &HealthStatus{
		Status:    newHealth.Status,
		Message:   newHealth.Message,
		CheckedAt: newHealth.CheckedAt,
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
			Status:    hc.Status,
			Message:   hc.Message,
			CheckedAt: hc.CheckedAt,
		}
	}

	return result, nil
}
