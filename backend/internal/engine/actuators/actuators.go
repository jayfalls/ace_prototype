package actuators

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ace/framework/backend/internal/engine/tools"
	"github.com/google/uuid"
)

// OutputType defines the type of output
type OutputType int

const (
	OutputChat OutputType = iota // Text response
	OutputTool                   // Execute tool
	OutputSignal                 // External signal
	OutputExport                 // Data export
)

// Output represents an actuator output
type Output struct {
	ID       uuid.UUID
	Type     OutputType
	Target   string
	Content  string
	Metadata map[string]interface{}
}

// Handler handles output processing
type Handler interface {
	Handle(ctx context.Context, output *Output) error
	Type() OutputType
}

// ChatHandler sends text responses
type ChatHandler struct {
	mu         sync.RWMutex
	callbacks  map[uuid.UUID]func(string)
	webhookURL string
}

// NewChatHandler creates a chat output handler
func NewChatHandler() *ChatHandler {
	return &ChatHandler{
		callbacks: make(map[uuid.UUID]func(string)),
	}
}

func (h *ChatHandler) Type() OutputType { return OutputChat }

func (h *ChatHandler) Handle(ctx context.Context, output *Output) error {
	if output.Content == "" {
		return nil
	}

	h.mu.RLock()
	for _, cb := range h.callbacks {
		cb(output.Content)
	}
	h.mu.RUnlock()

	// Send to webhook if configured
	if h.webhookURL != "" {
		// Would make HTTP POST here
	}

	return nil
}

// RegisterCallback adds a callback for chat messages
func (h *ChatHandler) RegisterCallback(sessionID uuid.UUID, cb func(string)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.callbacks[sessionID] = cb
}

// UnregisterCallback removes a callback
func (h *ChatHandler) UnregisterCallback(sessionID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.callbacks, sessionID)
}

// ToolHandler executes tools
type ToolHandler struct {
	executor *tools.Executor
}

// NewToolHandler creates a tool output handler
func NewToolHandler(executor *tools.Executor) *ToolHandler {
	return &ToolHandler{executor: executor}
}

func (h *ToolHandler) Type() OutputType { return OutputTool }

func (h *ToolHandler) Handle(ctx context.Context, output *Output) error {
	toolName, ok := output.Metadata["tool"].(string)
	if !ok {
		return fmt.Errorf("tool name not specified")
	}

	input, ok := output.Metadata["input"].(map[string]interface{})
	if !ok {
		input = make(map[string]interface{})
	}

	result, err := h.executor.Execute(ctx, toolName, input)
	if err != nil {
		return err
	}

	// Store result in metadata for downstream processing
	output.Metadata["result"] = result
	return nil
}

// SignalHandler sends external signals
type SignalHandler struct {
	mu       sync.RWMutex
	handlers map[string]func(interface{}) error
}

// NewSignalHandler creates a signal output handler
func NewSignalHandler() *SignalHandler {
	return &SignalHandler{
		handlers: make(map[string]func(interface{}) error),
	}
}

func (h *SignalHandler) Type() OutputType { return OutputSignal }

func (h *SignalHandler) Handle(ctx context.Context, output *Output) error {
	signalType, ok := output.Metadata["signal_type"].(string)
	if !ok {
		return fmt.Errorf("signal type not specified")
	}

	h.mu.RLock()
	handler, ok := h.handlers[signalType]
	h.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no handler for signal type: %s", signalType)
	}

	return handler(output.Metadata["payload"])
}

// RegisterSignalHandler adds a signal handler
func (h *SignalHandler) RegisterSignalHandler(signalType string, handler func(interface{}) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handlers[signalType] = handler
}

// ExportHandler exports data
type ExportHandler struct {
	mu          sync.RWMutex
	exporters   map[string]Exporter
	defaultFmt  string
}

// Exporter defines export functionality
type Exporter interface {
	Export(ctx context.Context, data interface{}) ([]byte, error)
	Format() string
}

// NewExportHandler creates an export handler
func NewExportHandler() *ExportHandler {
	return &ExportHandler{
		exporters:  make(map[string]Exporter),
		defaultFmt: "json",
	}
}

func (h *ExportHandler) Type() OutputType { return OutputExport }

func (h *ExportHandler) Handle(ctx context.Context, output *Output) error {
	format := output.Metadata["format"].(string)
	if format == "" {
		format = h.defaultFmt
	}

	h.mu.RLock()
	exporter, ok := h.exporters[format]
	h.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no exporter for format: %s", format)
	}

	data := output.Metadata["data"]
	bytes, err := exporter.Export(ctx, data)
	if err != nil {
		return err
	}

	output.Content = string(bytes)
	return nil
}

// RegisterExporter adds an exporter
func (h *ExportHandler) RegisterExporter(exporter Exporter) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.exporters[exporter.Format()] = exporter
}

// JSONExporter exports as JSON
type JSONExporter struct{}

func (e *JSONExporter) Export(ctx context.Context, data interface{}) ([]byte, error) {
	return json.MarshalIndent(data, "", "  ")
}

func (e *JSONExporter) Format() string { return "json" }

// CSVExporter exports as CSV
type CSVExporter struct{}

func (e *CSVExporter) Export(ctx context.Context, data interface{}) ([]byte, error) {
	// Would convert to CSV format
	return []byte("col1,col2\nval1,val2\n"), nil
}

func (e *CSVExporter) Format() string { return "csv" }

// OutputDispatcher routes outputs to appropriate handlers
type OutputDispatcher struct {
	mu       sync.RWMutex
	handlers map[OutputType]Handler
}

// NewOutputDispatcher creates a dispatcher
func NewOutputDispatcher() *OutputDispatcher {
	d := &OutputDispatcher{
		handlers: make(map[OutputType]Handler),
	}

	// Register default handlers
	d.handlers[OutputChat] = NewChatHandler()
	d.handlers[OutputTool] = nil // Will be set when executor available
	d.handlers[OutputSignal] = NewSignalHandler()
	d.handlers[OutputExport] = NewExportHandler()

	return d
}

// Register adds a handler for output type
func (d *OutputDispatcher) Register(outputType OutputType, handler Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[outputType] = handler
}

// Dispatch routes an output to its handler
func (d *OutputDispatcher) Dispatch(ctx context.Context, output *Output) error {
	d.mu.RLock()
	handler, ok := d.handlers[output.Type]
	d.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no handler for output type: %d", output.Type)
	}

	return handler.Handle(ctx, output)
}

// DispatchBatch processes multiple outputs
func (d *OutputDispatcher) DispatchBatch(ctx context.Context, outputs []*Output) []error {
	errors := make([]error, 0, len(outputs))

	for _, output := range outputs {
		if err := d.Dispatch(ctx, output); err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}
