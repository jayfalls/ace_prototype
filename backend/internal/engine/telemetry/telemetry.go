package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/ace/framework/backend/internal/engine/layers"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// InputType represents the type of input
type InputType int

const (
	InputChat InputType = iota // User message
	InputWebhook              // External webhook
	InputSensor               // Sensor data
	InputMetric               // System metric
)

// Input represents an incoming observation
type Input struct {
	ID        uuid.UUID
	Type      InputType
	Source    string
	Content   string
	Metadata  map[string]interface{}
	Timestamp time.Time
}

// MetricsCollector collects application metrics
type MetricsCollector struct {
	mu              sync.RWMutex
	requestCount    map[string]int
	requestDuration map[string]time.Duration
	errorCount      map[string]int
	layerCycles     map[string]int
	llmCalls        int
	llmErrors       int
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		requestCount:    make(map[string]int),
		requestDuration: make(map[string]time.Duration),
		errorCount:      make(map[string]int),
		layerCycles:    make(map[string]int),
	}
}

// IncrementRequest increments request count for an endpoint
func (m *MetricsCollector) IncrementRequest(endpoint string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestCount[endpoint]++
}

// RecordDuration records request duration
func (m *MetricsCollector) RecordDuration(endpoint string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestDuration[endpoint] += duration
}

// IncrementError increments error count for an endpoint
func (m *MetricsCollector) IncrementError(endpoint string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorCount[endpoint]++
}

// IncrementLayerCycle increments layer cycle count
func (m *MetricsCollector) IncrementLayerCycle(layer string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.layerCycles[layer]++
}

// IncrementLLMCall increments LLM call count
func (m *MetricsCollector) IncrementLLMCall() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.llmCalls++
}

// IncrementLLMError increments LLM error count
func (m *MetricsCollector) IncrementLLMError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.llmErrors++
}

// GetMetrics returns current metrics
func (m *MetricsCollector) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]interface{}{
		"requests":       m.requestCount,
		"duration":       m.requestDuration,
		"errors":         m.errorCount,
		"layer_cycles":  m.layerCycles,
		"llm_calls":     m.llmCalls,
		"llm_errors":    m.llmErrors,
	}
}

// Tracer provides distributed tracing
type Tracer struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
}

// NewTracer creates a new tracer
func NewTracer(serviceName string) (*Tracer, error) {
	// Create stdout exporter for demonstration
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		return nil, err
	}

	// Create resource
	res, err := resource.New(context.Background(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Register provider
	otel.SetTracerProvider(tp)

	return &Tracer{
		provider: tp,
		tracer:   tp.Tracer(serviceName),
	}, nil
}

// StartSpan starts a new span
func (t *Tracer) StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name)
}

// AddAttributes adds attributes to a span
func (t *Tracer) AddAttributes(span trace.Span, attrs ...attribute.KeyValue) {
	span.SetAttributes(attrs...)
}

// End ends a span
func (t *Tracer) End(span trace.Span) {
	span.End()
}

// Shutdown shuts down the tracer
func (t *Tracer) Shutdown(ctx context.Context) error {
	return t.provider.Shutdown(ctx)
}

// Logger provides structured logging
type Logger struct {
	logger *log.Logger
	mu     sync.Mutex
}

// NewLogger creates a new logger
func NewLogger(prefix string) *Logger {
	return &Logger{
		logger: log.New(log.Default().Writer(), prefix, log.LstdFlags),
	}
}

// Log logs a message with level
func (l *Logger) Log(level, msg string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	fieldsJSON, _ := json.Marshal(fields)
	l.logger.Printf("[%s] %s %s", level, msg, fieldsJSON)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields map[string]interface{}) {
	l.Log("INFO", msg, fields)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields map[string]interface{}) {
	l.Log("ERROR", msg, fields)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields map[string]interface{}) {
	l.Log("WARN", msg, fields)
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields map[string]interface{}) {
	l.Log("DEBUG", msg, fields)
}

// Observability combines metrics, tracing, and logging
type Observability struct {
	Metrics *MetricsCollector
	Tracer  *Tracer
	Logger  *Logger
	enabled bool
}

// NewObservability creates a new observability instance
func NewObservability(serviceName string, enabled bool) (*Observability, error) {
	obs := &Observability{
		Metrics: NewMetricsCollector(),
		Logger:  NewLogger("[ACE] "),
		enabled: enabled,
	}

	if enabled {
		tracer, err := NewTracer(serviceName)
		if err != nil {
			return nil, err
		}
		obs.Tracer = tracer
		obs.Logger.Info("Observability initialized", map[string]interface{}{
			"service": serviceName,
			"tracing": true,
		})
	}

	return obs, nil
}

// Shutdown shuts down observability
func (o *Observability) Shutdown(ctx context.Context) {
	if o.Tracer != nil {
		o.Tracer.Shutdown(ctx)
	}
	o.Logger.Info("Observability shutdown complete", map[string]interface{}{})
}

// Processor handles incoming observations
type Processor interface {
	Process(ctx context.Context, input *Input) (*layers.LayerInput, error)
}

// Observer is the telemetry entry point
type Observer struct {
	mu          sync.RWMutex
	processors  map[InputType]Processor
	inputCh     chan *Input
	webhookPort string
	httpServer  *http.Server
}

// NewObserver creates a new observer
func NewObserver() *Observer {
	return &Observer{
		processors: make(map[InputType]Processor),
		inputCh:    make(chan *Input, 100),
	}
}

// RegisterProcessor adds a handler for input type
func (o *Observer) RegisterProcessor(inputType InputType, p Processor) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.processors[inputType] = p
}

// Start begins processing inputs
func (o *Observer) Start(ctx context.Context) error {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case input := <-o.inputCh:
				o.processInput(ctx, input)
			}
		}
	}()
	return nil
}

// Observe processes an incoming observation
func (o *Observer) Observe(ctx context.Context, input *Input) error {
	select {
	case o.inputCh <- input:
		return nil
	default:
		return fmt.Errorf("input channel full")
	}
}

func (o *Observer) processInput(ctx context.Context, input *Input) {
	o.mu.RLock()
	processor, ok := o.processors[input.Type]
	o.mu.RUnlock()

	if !ok {
		return
	}

	_, err := processor.Process(ctx, input)
	if err != nil {
		// Log error but don't crash
		fmt.Printf("Error processing input: %v\n", err)
	}
}

// ChatProcessor handles chat inputs
type ChatProcessor struct{}

func NewChatProcessor() *ChatProcessor {
	return &ChatProcessor{}
}

func (p *ChatProcessor) Process(ctx context.Context, input *Input) (*layers.LayerInput, error) {
	return &layers.LayerInput{
		AgentID:   uuid.Nil, // Will be set by router
		SessionID: uuid.New(),
		CycleID:   uuid.New(),
		LayerID:   uuid.New(),
		Data:      input.Content,
		Memory:    &layers.MemoryContext{},
	}, nil
}

// WebhookProcessor handles webhook inputs
type WebhookProcessor struct {
	parser func([]byte) (string, error)
}

func NewWebhookProcessor(parser func([]byte) (string, error)) *WebhookProcessor {
	return &WebhookProcessor{parser: parser}
}

func (p *WebhookProcessor) Process(ctx context.Context, input *Input) (*layers.LayerInput, error) {
	if p.parser == nil {
		return nil, fmt.Errorf("no parser configured")
	}

	content, err := p.parser([]byte(input.Content))
	if err != nil {
		return nil, err
	}

	return &layers.LayerInput{
		AgentID:   uuid.Nil,
		SessionID: uuid.New(),
		CycleID:   uuid.New(),
		LayerID:   uuid.New(),
		Data:      content,
		Memory:    &layers.MemoryContext{},
	}, nil
}

// SensorProcessor handles sensor data
type SensorProcessor struct {
	transform func(map[string]interface{}) (string, error)
}

func NewSensorProcessor(transform func(map[string]interface{}) (string, error)) *SensorProcessor {
	return &SensorProcessor{transform: transform}
}

func (p *SensorProcessor) Process(ctx context.Context, input *Input) (*layers.LayerInput, error) {
	metadata, ok := input.Metadata["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid sensor data")
	}

	if p.transform != nil {
		content, err := p.transform(metadata)
		if err != nil {
			return nil, err
		}
		return &layers.LayerInput{
			AgentID:   uuid.Nil,
			SessionID: uuid.New(),
			CycleID:   uuid.New(),
			LayerID:   uuid.New(),
			Data:      content,
			Memory:    &layers.MemoryContext{},
		}, nil
	}

	// Default: serialize as JSON
	data, _ := json.Marshal(metadata)
	return &layers.LayerInput{
		AgentID:   uuid.Nil,
		SessionID: uuid.New(),
		CycleID:   uuid.New(),
		LayerID:   uuid.New(),
		Data:      string(data),
		Memory:    &layers.MemoryContext{},
	}, nil
}

// StartWebhookServer starts HTTP server for webhooks
func (o *Observer) StartWebhookServer(port string) error {
	o.webhookPort = port

	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		input := &Input{
			ID:        uuid.New(),
			Type:      InputWebhook,
			Source:    r.RemoteAddr,
			Content:   r.URL.Query().Get("content"),
			Metadata:  payload,
			Timestamp: time.Now(),
		}

		if content, ok := payload["content"].(string); ok {
			input.Content = content
		}

		if err := o.Observe(r.Context(), input); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "received"})
	})

	o.httpServer = &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	go o.httpServer.ListenAndServe()
	return nil
}

// StopWebhookServer stops the webhook server
func (o *Observer) StopWebhookServer() error {
	if o.httpServer != nil {
		return o.httpServer.Shutdown(context.Background())
	}
	return nil
}
