package loops

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ace/framework/backend/internal/engine/layers"
	"github.com/google/uuid"
)

// LoopType defines the type of processing loop
type LoopType int

const (
	LoopTaskProsecution LoopType = iota // L6 - infinite, runs until goal
	LoopPlanning                        // L2/L4 - finite, fixed iterations
	LoopMonitoring                     // Safety - continuous monitoring
)

// LoopConfig holds loop configuration
type LoopConfig struct {
	Type         LoopType
	MaxCycles    int
	MaxTime      time.Duration
	Interval     time.Duration // For periodic loops
	StopOnError  bool
}

// Loop represents a processing loop
type Loop interface {
	Type() LoopType
	Config() LoopConfig
	Start(ctx context.Context) error
	Stop() error
	IsRunning() bool
}

// BaseLoop provides common loop functionality
type BaseLoop struct {
	loopType LoopType
	config   LoopConfig
	running  bool
	mu       sync.RWMutex
}

func (b *BaseLoop) Type() LoopType     { return b.loopType }
func (b *BaseLoop) Config() LoopConfig { return b.config }
func (b *BaseLoop) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.running
}

func (b *BaseLoop) setRunning(running bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.running = running
}

// LayerLoop processes input through all ACE layers
type LayerLoop struct {
	*BaseLoop
	engine *layers.Engine
}

// NewLayerLoop creates a loop that processes through layers
func NewLayerLoop(engine *layers.Engine, loopType LoopType) *LayerLoop {
	config := LoopConfig{
		Type:         loopType,
		MaxCycles:    10,
		MaxTime:      30 * time.Second,
		StopOnError:  false,
	}

	// Adjust config based on loop type
	switch loopType {
	case LoopTaskProsecution:
		config.MaxCycles = 0 // Infinite
		config.MaxTime = 0
	}

	return &LayerLoop{
		BaseLoop: &BaseLoop{
			loopType: loopType,
			config:   config,
		},
		engine: engine,
	}
}

func (l *LayerLoop) Start(ctx context.Context) error {
	if l.IsRunning() {
		return nil
	}

	l.setRunning(true)

	go func() {
		cycle := 0
		for {
			select {
			case <-ctx.Done():
				l.setRunning(false)
				return
			default:
				if !l.IsRunning() {
					return
				}

				cycle++

				// Check limits
				if l.config.MaxCycles > 0 && cycle > l.config.MaxCycles {
					l.setRunning(false)
					return
				}

				// Process one cycle
				_, err := l.engine.ProcessCycle(ctx, "cycle input")
				if err != nil && l.config.StopOnError {
					l.setRunning(false)
					return
				}

				// Small delay between cycles
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	return nil
}

func (l *LayerLoop) Stop() error {
	l.setRunning(false)
	return nil
}

// GlobalLoops represents the Human-Model Reference (HRM) loops
type GlobalLoops struct {
	mu      sync.RWMutex
	loops   map[string]Loop
	agentID uuid.UUID
}

// NewGlobalLoops creates the global loop manager
func NewGlobalLoops(agentID uuid.UUID) *GlobalLoops {
	return &GlobalLoops{
		loops:   make(map[string]Loop),
		agentID: agentID,
	}
}

// Register adds a named loop
func (g *GlobalLoops) Register(name string, loop Loop) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.loops[name] = loop
}

// Start starts all loops
func (g *GlobalLoops) Start(ctx context.Context) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for name, loop := range g.loops {
		if err := loop.Start(ctx); err != nil {
			return fmt.Errorf("failed to start loop %s: %w", name, err)
		}
	}
	return nil
}

// Stop stops all loops
func (g *GlobalLoops) Stop() error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, loop := range g.loops {
		if err := loop.Stop(); err != nil {
			return err
		}
	}
	return nil
}

// Get returns a loop by name
func (g *GlobalLoops) Get(name string) (Loop, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	loop, ok := g.loops[name]
	return loop, ok
}

// List returns all registered loops
func (g *GlobalLoops) List() []Loop {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]Loop, 0, len(g.loops))
	for _, loop := range g.loops {
		result = append(result, loop)
	}
	return result
}

// ChatLoop handles the fast human interaction loop
type ChatLoop struct {
	*BaseLoop
	inputCh chan string
}

// NewChatLoop creates the chat loop
func NewChatLoop() *ChatLoop {
	return &ChatLoop{
		BaseLoop: &BaseLoop{
			loopType: LoopMonitoring,
			config: LoopConfig{
				Type:        LoopMonitoring,
				MaxCycles:   0, // Infinite
				MaxTime:     0,
				Interval:    0,
				StopOnError: false,
			},
		},
		inputCh: make(chan string, 100),
	}
}

func (l *ChatLoop) Start(ctx context.Context) error {
	l.setRunning(true)
	go func() {
		for {
			select {
			case <-ctx.Done():
				l.setRunning(false)
				return
			case input := <-l.inputCh:
				// Process chat input
				fmt.Printf("Chat input: %s\n", input)
			}
		}
	}()
	return nil
}

func (l *ChatLoop) Stop() error {
	l.setRunning(false)
	return nil
}

// SendInput sends input to chat loop
func (l *ChatLoop) SendInput(input string) error {
	select {
	case l.inputCh <- input:
		return nil
	default:
		return fmt.Errorf("input channel full")
	}
}

// SafetyMonitorLoop monitors for threats
type SafetyMonitorLoop struct {
	*BaseLoop
	alertCh chan string
}

// NewSafetyMonitorLoop creates the safety loop
func NewSafetyMonitorLoop() *SafetyMonitorLoop {
	return &SafetyMonitorLoop{
		BaseLoop: &BaseLoop{
			loopType: LoopMonitoring,
			config: LoopConfig{
				Type:        LoopMonitoring,
				MaxCycles:   0,
				MaxTime:     0,
				Interval:    100 * time.Millisecond,
				StopOnError: true,
			},
		},
		alertCh: make(chan string, 10),
	}
}

func (l *SafetyMonitorLoop) Start(ctx context.Context) error {
	l.setRunning(true)
	go func() {
		for {
			select {
			case <-ctx.Done():
				l.setRunning(false)
				return
			case <-time.After(l.config.Interval):
				// Check safety conditions
				// In real implementation, would analyze layer outputs
			}
		}
	}()
	return nil
}

func (l *SafetyMonitorLoop) Stop() error {
	l.setRunning(false)
	return nil
}

// Alerts returns the alert channel
func (l *SafetyMonitorLoop) Alerts() <-chan string {
	return l.alertCh
}
