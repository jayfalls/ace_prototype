// Package handler contains HTTP request handlers.
package handler

import (
	"net/http"

	"ace/internal/api/response"
)

// AgentHandler handles agent-related HTTP requests.
type AgentHandler struct{}

// NewAgentHandler creates a new AgentHandler.
func NewAgentHandler() *AgentHandler {
	return &AgentHandler{}
}

// List handles GET /agents - returns an empty agent list (stub).
func (h *AgentHandler) List(w http.ResponseWriter, r *http.Request) {
	response.Success(w, []interface{}{})
}
