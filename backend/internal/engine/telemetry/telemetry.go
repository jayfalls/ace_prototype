package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ace/framework/backend/internal/engine/layers"
	"github.com/google/uuid"
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
