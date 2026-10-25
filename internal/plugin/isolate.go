package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"ace/internal/core"
)

// IsolatedTool wraps a tool so a panic disables its plugin and returns an
// error instead of crashing the loop (spec 9.1).
type IsolatedTool struct {
	core.Tool
	reg    *Registry
	plugin string
}

// Isolate wraps every tool a plugin provides.
func (r *Registry) Isolate(pluginName string) {
	r.mu.RLock()
	p, ok := r.plugins[pluginName]
	r.mu.RUnlock()
	if !ok {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, t := range p.tools {
		p.tools[name] = &IsolatedTool{Tool: t, reg: r, plugin: pluginName}
	}
}

func (t *IsolatedTool) Call(ctx context.Context, args json.RawMessage) (out core.ToolResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			t.reg.Disable(t.plugin, fmt.Sprintf("panic in tool %s: %v", t.Tool.Name(), r))
			err = fmt.Errorf("tool %s panicked: %v", t.Tool.Name(), r)
		}
	}()
	return t.Tool.Call(ctx, args)
}
