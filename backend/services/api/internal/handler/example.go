// Package handler contains HTTP request handlers.
package handler

import (
	"encoding/json"
	"net/http"

	"ace/api/internal/response"
	"ace/api/internal/validator"
)

// ExampleHandler handles example-related HTTP requests.
type ExampleHandler struct{}

// NewExampleHandler creates a new ExampleHandler.
func NewExampleHandler() *ExampleHandler {
	return &ExampleHandler{}
}

// CreateExampleRequest represents the request body for creating an example.
type CreateExampleRequest struct {
	Name  string `json:"name" validate:"required,min=1,max=100"`
	Email string `json:"email" validate:"required,email"`
}

// ExampleResponse represents an example response.
type ExampleResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Create handles POST /examples - demonstrates validation pattern
func (h *ExampleHandler) Create(w http.ResponseWriter, r *http.Request) {
	// 1. Decode request body
	var req CreateExampleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid_request", "Invalid request body")
		return
	}

	// 2. Validate request
	if err := validator.ValidateStruct(req); err != nil {
		response.ValidationError(w, err)
		return
	}

	// 3. Business logic (placeholder)
	example := ExampleResponse{
		ID:    "123",
		Name:  req.Name,
		Email: req.Email,
	}

	// 4. Return response
	response.Created(w, example)
}

// Get handles GET /examples/{id} - demonstrates URL parameter validation
type GetExampleRequest struct {
	ID string `validate:"required,uuid"`
}

// Get handles GET /examples/:id
func (h *ExampleHandler) Get(w http.ResponseWriter, r *http.Request) {
	// In a real handler, you would get the ID from URL params
	// and validate it

	// Example response
	example := ExampleResponse{
		ID:    "123",
		Name:  "Example",
		Email: "example@example.com",
	}

	response.Success(w, example)
}
