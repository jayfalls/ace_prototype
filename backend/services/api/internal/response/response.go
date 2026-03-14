package response

import (
	"encoding/json"
	"net/http"

	"github.com/go-playground/validator/v10"
)

// APIResponse is the standard response format for all API responses
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
}

// APIError represents an error response
type APIError struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	Details   []FieldError   `json:"details,omitempty"`
}

// FieldError represents a validation error for a specific field
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// JSON sends a raw JSON response (for custom responses)
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Success sends a successful JSON response
func Success(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    data,
	})
}

// Created sends a 201 Created response
func Created(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    data,
	})
}

// Error sends an error response with the given code and message
func Error(w http.ResponseWriter, code, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	})
}

// ValidationError sends a validation error response with field-level details
func ValidationError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	var fieldErrors []FieldError
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, fe := range validationErrors {
			fieldErrors = append(fieldErrors, FieldError{
				Field:   fe.Field(),
				Message: formatValidationMessage(fe),
			})
		}
	}

	json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error: &APIError{
			Code:    "validation_error",
			Message: "Invalid request data",
			Details: fieldErrors,
		},
	})
}

// BadRequest sends a 400 Bad Request error
func BadRequest(w http.ResponseWriter, code, message string) {
	Error(w, code, message, http.StatusBadRequest)
}

// Unauthorized sends a 401 Unauthorized error
func Unauthorized(w http.ResponseWriter, message string) {
	Error(w, "unauthorized", message, http.StatusUnauthorized)
}

// Forbidden sends a 403 Forbidden error
func Forbidden(w http.ResponseWriter, message string) {
	Error(w, "forbidden", message, http.StatusForbidden)
}

// NotFound sends a 404 Not Found error
func NotFound(w http.ResponseWriter, message string) {
	Error(w, "not_found", message, http.StatusNotFound)
}

// InternalError sends a 500 Internal Server Error
func InternalError(w http.ResponseWriter, message string) {
	Error(w, "internal_error", message, http.StatusInternalServerError)
}

// formatValidationMessage creates a human-readable validation error message
func formatValidationMessage(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "This field is required"
	case "email":
		return "Invalid email format"
	case "min":
		return "Value is too short"
	case "max":
		return "Value is too long"
	case "url":
		return "Invalid URL format"
	default:
		return "Invalid value"
	}
}
