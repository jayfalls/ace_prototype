// Package handler provides tests for HTTP handlers.
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ace/api/internal/response"
)

func TestCreate_ValidRequest(t *testing.T) {
	handler := NewExampleHandler()

	body := CreateExampleRequest{
		Name:  "John Doe",
		Email: "john@example.com",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var resp response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}
}

func TestCreate_InvalidJSON(t *testing.T) {
	handler := NewExampleHandler()

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Success {
		t.Error("expected success to be false")
	}
}

func TestCreate_ValidationErrors(t *testing.T) {
	handler := NewExampleHandler()

	// Invalid request - missing name, invalid email
	body := map[string]string{
		"name":  "", // required but empty
		"email": "not-an-email",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Success {
		t.Error("expected success to be false")
	}

	if resp.Error.Code != "validation_error" {
		t.Errorf("expected code 'validation_error', got '%s'", resp.Error.Code)
	}

	if len(resp.Error.Details) == 0 {
		t.Error("expected field details to be present")
	}
}

func TestCreate_ValidationErrors_NameTooLong(t *testing.T) {
	handler := NewExampleHandler()

	// Invalid request - name too long (max=100)
	body := map[string]string{
		"name":  "ThisIsAVeryLongNameThatExceedsTheMaximumLengthAllowedWhichIsOneHundredCharactersButThisStringIsEvenLongerThanThatSoItShouldFailValidation1234567890",
		"email": "valid@example.com",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Success {
		t.Error("expected success to be false")
	}

	if resp.Error.Code != "validation_error" {
		t.Errorf("expected code 'validation_error', got '%s'", resp.Error.Code)
	}
}

func TestGet(t *testing.T) {
	handler := NewExampleHandler()

	req := httptest.NewRequest(http.MethodGet, "/123", nil)
	w := httptest.NewRecorder()

	handler.Get(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp response.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}
}
