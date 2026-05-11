package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"ace/internal/api/model"
	"ace/internal/api/response"
	"ace/internal/api/service"
)

// ProviderServiceImpl is the interface implemented by service.ProviderService.
type ProviderServiceImpl interface {
	CreateProvider(ctx context.Context, req model.ProviderCreateRequest) (*model.ProviderResponse, error)
	GetProvider(ctx context.Context, id string) (*model.ProviderResponse, error)
	ListProviders(ctx context.Context) ([]model.ProviderResponse, error)
	UpdateProvider(ctx context.Context, id string, req model.ProviderUpdateRequest) (*model.ProviderResponse, error)
	DeleteProvider(ctx context.Context, id string) error
}

// ProviderHandler handles HTTP requests for provider management.
type ProviderHandler struct {
	svc ProviderServiceImpl
}

// NewProviderHandler creates a new ProviderHandler.
func NewProviderHandler(svc ProviderServiceImpl) (*ProviderHandler, error) {
	if svc == nil {
		return nil, errors.New("provider service is required")
	}
	return &ProviderHandler{svc: svc}, nil
}

// Create handles POST /api/providers
func (h *ProviderHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.ProviderCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid_request", "Invalid request body")
		return
	}

	provider, err := h.svc.CreateProvider(r.Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrDuplicateName) {
			response.BadRequest(w, "duplicate_name", "Provider name already exists")
			return
		}
		// Validation errors from the service
		response.BadRequest(w, "validation_error", err.Error())
		return
	}

	response.Created(w, provider)
}

// List handles GET /api/providers
func (h *ProviderHandler) List(w http.ResponseWriter, r *http.Request) {
	providers, err := h.svc.ListProviders(r.Context())
	if err != nil {
		response.InternalError(w, "Failed to list providers")
		return
	}

	if providers == nil {
		providers = []model.ProviderResponse{}
	}

	response.Success(w, providers)
}

// Get handles GET /api/providers/{id}
func (h *ProviderHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.BadRequest(w, "invalid_request", "Provider ID is required")
		return
	}

	provider, err := h.svc.GetProvider(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrProviderNotFound) {
			response.NotFound(w, "Provider not found")
			return
		}
		response.InternalError(w, "Failed to get provider")
		return
	}

	response.Success(w, provider)
}

// Update handles PUT /api/providers/{id}
func (h *ProviderHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.BadRequest(w, "invalid_request", "Provider ID is required")
		return
	}

	var req model.ProviderUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid_request", "Invalid request body")
		return
	}

	provider, err := h.svc.UpdateProvider(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, service.ErrProviderNotFound) {
			response.NotFound(w, "Provider not found")
			return
		}
		if errors.Is(err, service.ErrDuplicateName) {
			response.BadRequest(w, "duplicate_name", "Provider name already exists")
			return
		}
		response.BadRequest(w, "validation_error", err.Error())
		return
	}

	response.Success(w, provider)
}

// Delete handles DELETE /api/providers/{id}
func (h *ProviderHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		response.BadRequest(w, "invalid_request", "Provider ID is required")
		return
	}

	if err := h.svc.DeleteProvider(r.Context(), id); err != nil {
		if errors.Is(err, service.ErrProviderNotFound) {
			response.NotFound(w, "Provider not found")
			return
		}
		response.InternalError(w, "Failed to delete provider")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
