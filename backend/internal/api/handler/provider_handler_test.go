package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"ace/internal/api/model"
	"ace/internal/api/response"
	"ace/internal/api/service"
)

// mockProviderService implements ProviderServiceImpl for testing.
type mockProviderService struct {
	CreateProviderFunc func(ctx context.Context, req model.ProviderCreateRequest) (*model.ProviderResponse, error)
	GetProviderFunc    func(ctx context.Context, id string) (*model.ProviderResponse, error)
	ListProvidersFunc  func(ctx context.Context) ([]model.ProviderResponse, error)
	UpdateProviderFunc func(ctx context.Context, id string, req model.ProviderUpdateRequest) (*model.ProviderResponse, error)
	DeleteProviderFunc func(ctx context.Context, id string) error
}

func (m *mockProviderService) CreateProvider(ctx context.Context, req model.ProviderCreateRequest) (*model.ProviderResponse, error) {
	return m.CreateProviderFunc(ctx, req)
}

func (m *mockProviderService) GetProvider(ctx context.Context, id string) (*model.ProviderResponse, error) {
	return m.GetProviderFunc(ctx, id)
}

func (m *mockProviderService) ListProviders(ctx context.Context) ([]model.ProviderResponse, error) {
	return m.ListProvidersFunc(ctx)
}

func (m *mockProviderService) UpdateProvider(ctx context.Context, id string, req model.ProviderUpdateRequest) (*model.ProviderResponse, error) {
	return m.UpdateProviderFunc(ctx, id, req)
}

func (m *mockProviderService) DeleteProvider(ctx context.Context, id string) error {
	return m.DeleteProviderFunc(ctx, id)
}

func newMockProviderService() *mockProviderService {
	return &mockProviderService{}
}

// decodeResponse decodes an APIResponse from the response body.
func decodeResponse(t *testing.T, body *bytes.Buffer) response.APIResponse {
	t.Helper()
	var resp response.APIResponse
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	return resp
}

// TestProviderHandler_Create tests the Create handler.
func TestProviderHandler_Create(t *testing.T) {
	t.Run("successful create returns 201 with masked key", func(t *testing.T) {
		mock := newMockProviderService()
		mock.CreateProviderFunc = func(ctx context.Context, req model.ProviderCreateRequest) (*model.ProviderResponse, error) {
			return &model.ProviderResponse{
				ID:           "test-id",
				Name:         "Test Provider",
				ProviderType: model.ProviderOpenAI,
				BaseURL:      "https://api.openai.com/v1",
				APIKeyMasked: "****",
				ConfigJSON:   map[string]interface{}{},
				IsEnabled:    true,
				CreatedAt:    "2025-01-01T00:00:00Z",
				UpdatedAt:    "2025-01-01T00:00:00Z",
			}, nil
		}

		h, err := NewProviderHandler(mock)
		if err != nil {
			t.Fatalf("failed to create handler: %v", err)
		}

		body := bytes.NewBufferString(`{"name":"Test Provider","provider_type":"openai","base_url":"https://api.openai.com/v1","api_key":"sk-test-key"}`)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/providers", body)

		h.Create(w, r)

		if w.Code != http.StatusCreated {
			t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
		}

		resp := decodeResponse(t, w.Body)
		if !resp.Success {
			t.Errorf("expected success response, got error: %+v", resp.Error)
		}
	})

	t.Run("create with missing api_key returns 400", func(t *testing.T) {
		mock := newMockProviderService()
		mock.CreateProviderFunc = func(ctx context.Context, req model.ProviderCreateRequest) (*model.ProviderResponse, error) {
			return nil, errors.New("api_key is required for provider type openai")
		}

		h, err := NewProviderHandler(mock)
		if err != nil {
			t.Fatalf("failed to create handler: %v", err)
		}

		body := bytes.NewBufferString(`{"name":"Test","provider_type":"openai","base_url":"https://api.openai.com/v1"}`)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/providers", body)

		h.Create(w, r)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}
	})

	t.Run("create with duplicate name returns 400", func(t *testing.T) {
		mock := newMockProviderService()
		mock.CreateProviderFunc = func(ctx context.Context, req model.ProviderCreateRequest) (*model.ProviderResponse, error) {
			return nil, service.ErrDuplicateName
		}

		h, err := NewProviderHandler(mock)
		if err != nil {
			t.Fatalf("failed to create handler: %v", err)
		}

		body := bytes.NewBufferString(`{"name":"Existing","provider_type":"openai","base_url":"https://api.openai.com/v1","api_key":"sk-test"}`)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/providers", body)

		h.Create(w, r)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", w.Code)
		}

		resp := decodeResponse(t, w.Body)
		if resp.Error == nil || resp.Error.Code != "duplicate_name" {
			t.Errorf("expected duplicate_name error, got: %+v", resp.Error)
		}
	})
}

// TestProviderHandler_List tests the List handler.
func TestProviderHandler_List(t *testing.T) {
	t.Run("list returns providers with masked keys", func(t *testing.T) {
		mock := newMockProviderService()
		mock.ListProvidersFunc = func(ctx context.Context) ([]model.ProviderResponse, error) {
			return []model.ProviderResponse{
				{
					ID:           "id-1",
					Name:         "Provider 1",
					ProviderType: model.ProviderOpenAI,
					BaseURL:      "https://api.openai.com/v1",
					APIKeyMasked: "****",
					ConfigJSON:   map[string]interface{}{},
					IsEnabled:    true,
				},
			}, nil
		}

		h, err := NewProviderHandler(mock)
		if err != nil {
			t.Fatalf("failed to create handler: %v", err)
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/providers", nil)

		h.List(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}

		resp := decodeResponse(t, w.Body)
		if !resp.Success {
			t.Errorf("expected success, got error: %+v", resp.Error)
		}
	})

	t.Run("list empty returns empty array", func(t *testing.T) {
		mock := newMockProviderService()
		mock.ListProvidersFunc = func(ctx context.Context) ([]model.ProviderResponse, error) {
			return nil, nil
		}

		h, err := NewProviderHandler(mock)
		if err != nil {
			t.Fatalf("failed to create handler: %v", err)
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/providers", nil)

		h.List(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})
}

// TestProviderHandler_Get tests the Get handler.
func TestProviderHandler_Get(t *testing.T) {
	t.Run("get existing provider returns 200", func(t *testing.T) {
		mock := newMockProviderService()
		mock.GetProviderFunc = func(ctx context.Context, id string) (*model.ProviderResponse, error) {
			return &model.ProviderResponse{
				ID:           id,
				Name:         "Test",
				ProviderType: model.ProviderOpenAI,
				APIKeyMasked: "****",
			}, nil
		}

		h, err := NewProviderHandler(mock)
		if err != nil {
			t.Fatalf("failed to create handler: %v", err)
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/providers/test-id", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "test-id")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		h.Get(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("get non-existent provider returns 404", func(t *testing.T) {
		mock := newMockProviderService()
		mock.GetProviderFunc = func(ctx context.Context, id string) (*model.ProviderResponse, error) {
			return nil, service.ErrProviderNotFound
		}

		h, err := NewProviderHandler(mock)
		if err != nil {
			t.Fatalf("failed to create handler: %v", err)
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/providers/nonexistent", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "nonexistent")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		h.Get(w, r)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// TestProviderHandler_Update tests the Update handler.
func TestProviderHandler_Update(t *testing.T) {
	t.Run("update provider returns 200", func(t *testing.T) {
		mock := newMockProviderService()
		mock.UpdateProviderFunc = func(ctx context.Context, id string, req model.ProviderUpdateRequest) (*model.ProviderResponse, error) {
			name := "Updated"
			return &model.ProviderResponse{
				ID:           id,
				Name:         name,
				ProviderType: model.ProviderOpenAI,
				APIKeyMasked: "****",
			}, nil
		}

		h, err := NewProviderHandler(mock)
		if err != nil {
			t.Fatalf("failed to create handler: %v", err)
		}

		body := bytes.NewBufferString(`{"name":"Updated"}`)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPut, "/api/providers/test-id", body)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "test-id")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		h.Update(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("update non-existent returns 404", func(t *testing.T) {
		mock := newMockProviderService()
		mock.UpdateProviderFunc = func(ctx context.Context, id string, req model.ProviderUpdateRequest) (*model.ProviderResponse, error) {
			return nil, service.ErrProviderNotFound
		}

		h, err := NewProviderHandler(mock)
		if err != nil {
			t.Fatalf("failed to create handler: %v", err)
		}

		body := bytes.NewBufferString(`{"name":"Updated"}`)
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPut, "/api/providers/nonexistent", body)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "nonexistent")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		h.Update(w, r)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}

// TestProviderHandler_Delete tests the Delete handler.
func TestProviderHandler_Delete(t *testing.T) {
	t.Run("delete provider returns 204", func(t *testing.T) {
		mock := newMockProviderService()
		mock.DeleteProviderFunc = func(ctx context.Context, id string) error {
			return nil
		}

		h, err := NewProviderHandler(mock)
		if err != nil {
			t.Fatalf("failed to create handler: %v", err)
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodDelete, "/api/providers/test-id", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "test-id")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		h.Delete(w, r)

		if w.Code != http.StatusNoContent {
			t.Errorf("expected 204, got %d", w.Code)
		}
	})

	t.Run("delete non-existent returns 404", func(t *testing.T) {
		mock := newMockProviderService()
		mock.DeleteProviderFunc = func(ctx context.Context, id string) error {
			return service.ErrProviderNotFound
		}

		h, err := NewProviderHandler(mock)
		if err != nil {
			t.Fatalf("failed to create handler: %v", err)
		}

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodDelete, "/api/providers/nonexistent", nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "nonexistent")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

		h.Delete(w, r)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}
