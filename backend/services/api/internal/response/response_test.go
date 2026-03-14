package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-playground/validator/v10"
)

type TestStruct struct {
	Name  string `json:"name" validate:"required,min=2,max=10"`
	Email string `json:"email" validate:"required,email"`
}

func TestSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"message": "hello"}

	Success(w, data)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}
	if resp.Data.(map[string]interface{})["message"] != "hello" {
		t.Error("expected data to match")
	}
}

func TestCreated(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"id": "123"}

	Created(w, data)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Error("expected success to be true")
	}
}

func TestError(t *testing.T) {
	w := httptest.NewRecorder()

	Error(w, "test_code", "test message", http.StatusBadRequest)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Success {
		t.Error("expected success to be false")
	}
	if resp.Error.Code != "test_code" {
		t.Errorf("expected code 'test_code', got '%s'", resp.Error.Code)
	}
	if resp.Error.Message != "test message" {
		t.Errorf("expected message 'test message', got '%s'", resp.Error.Message)
	}
}

func TestValidationError(t *testing.T) {
	w := httptest.NewRecorder()

	// Create a test struct with validation errors
	testStruct := TestStruct{
		Name:  "A", // too short
		Email: "invalid", // not an email
	}
	
	validate := validator.New()
	err := validate.Struct(testStruct)
	
	ValidationError(w, err)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp APIResponse
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

func TestBadRequest(t *testing.T) {
	w := httptest.NewRecorder()

	BadRequest(w, "invalid_input", "Invalid input data")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Error.Code != "invalid_input" {
		t.Errorf("expected code 'invalid_input', got '%s'", resp.Error.Code)
	}
}

func TestUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()

	Unauthorized(w, "Invalid credentials")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestForbidden(t *testing.T) {
	w := httptest.NewRecorder()

	Forbidden(w, "Access denied")

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestNotFound(t *testing.T) {
	w := httptest.NewRecorder()

	NotFound(w, "Resource not found")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestInternalError(t *testing.T) {
	w := httptest.NewRecorder()

	InternalError(w, "Something went wrong")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestJSON(t *testing.T) {
	w := httptest.NewRecorder()

	JSON(w, http.StatusOK, map[string]string{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["key"] != "value" {
		t.Errorf("expected 'value', got '%s'", resp["key"])
	}
}
