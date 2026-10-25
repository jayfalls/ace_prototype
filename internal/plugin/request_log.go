package plugin

import (
	"context"
	"database/sql"
	"strconv"

	"ace/internal/core"
	"ace/internal/provider"

	"github.com/google/uuid"
)

// requestLogger wraps an LLMProvider so every model request is logged to
// model_requests before dispatch (spec 9.1 pre_model invariant: the sent
// bytes are reconstructable from the event log — blocks_json is the full
// marshalled CompletionRequest, sha-verified on rebuild).
type requestLogger struct {
	inner core.LLMProvider
	db    *sql.DB
}

// NewRequestLogger returns inner wrapped with model-request logging. db nil
// disables logging without changing behavior.
func NewRequestLogger(inner core.LLMProvider, db *sql.DB) core.LLMProvider {
	migrateModelRequests(db)
	return &requestLogger{inner: inner, db: db}
}

func (l *requestLogger) Name() string                  { return l.inner.Name() }
func (l *requestLogger) Models() []core.ModelInfo      { return l.inner.Models() }
func (l *requestLogger) ActiveModel() string           { return l.inner.ActiveModel() }
func (l *requestLogger) SetActiveModel(m string) error { return l.inner.SetActiveModel(m) }

func (l *requestLogger) StreamComplete(ctx context.Context, req core.CompletionRequest, onDelta func(core.StreamDelta)) (core.CompletionResponse, error) {
	callID := "call_" + uuid.NewString()[:12]
	// injected_block_ids names each request part so the log shows what was
	// injected: the system prompt plus one id per message role.
	ids := make([]string, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		ids = append(ids, "system")
	}
	for i, m := range req.Messages {
		ids = append(ids, m.Role+":"+strconv.Itoa(i))
	}
	_ = RecordModelRequest(ctx, l.db, callID, provider.LayerFrom(ctx), ids, req)
	return l.inner.StreamComplete(ctx, req, onDelta)
}
