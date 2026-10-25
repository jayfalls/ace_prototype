package plugin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"ace/internal/core"
)

// ErrUndeclared is returned when a plugin looks up a capability it did not
// declare in depends[] (spec 9.1).
var ErrUndeclared = errors.New("undeclared dependency lookup")

// Plugin is one loaded unit: its manifest, its registered ports, and its
// runtime state.
type Plugin struct {
	Manifest Manifest
	State    string // loaded | disabled
	Disabled string // reason when disabled
	tools    map[string]core.Tool
	channel  core.Channel
	senses   []core.Sense
	ctx      core.ContextProvider
	lookup   func(key string) (interface{}, error)
}

// Tools returns the plugin's registered tools.
func (p *Plugin) Tools() []core.Tool {
	out := make([]core.Tool, 0, len(p.tools))
	for _, t := range p.tools {
		out = append(out, t)
	}
	return out
}

// Registry holds loaded plugins keyed by name plus the capability index
// (provides-key -> plugin name). Load rejects dependency cycles.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]*Plugin
	byKey   map[string]string // provides-key -> plugin name
	onEvent func(kind, summary string)
}

// New builds an empty registry; onEvent receives plugin_disabled reports.
func New(onEvent func(kind, summary string)) *Registry {
	return &Registry{plugins: map[string]*Plugin{}, byKey: map[string]string{}, onEvent: onEvent}
}

// Load scans dirs for plugin.toml files, parses them, then rejects
// dependency cycles before any plugin code runs.
func (r *Registry) Load(dirs ...string) error {
	var manifests []Manifest
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // missing dir is not an error
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			mPath := filepath.Join(dir, e.Name(), "plugin.toml")
			m, err := ParseManifest(mPath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return fmt.Errorf("plugin %s: %w", e.Name(), err)
			}
			manifests = append(manifests, m)
		}
	}
	if err := rejectCycles(manifests); err != nil {
		return err
	}
	for _, m := range manifests {
		if err := r.add(m); err != nil {
			return err
		}
	}
	return nil
}

// add registers a parsed manifest; port wiring happens separately via
// RegisterTool/RegisterChannel.
func (r *Registry) add(m Manifest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.plugins[m.Name]; dup {
		return fmt.Errorf("plugin %s: duplicate name", m.Name)
	}
	p := &Plugin{Manifest: m, State: "loaded", tools: map[string]core.Tool{}}
	for _, key := range m.Provides {
		if owner, clash := r.byKey[key]; clash {
			p.State, p.Disabled = "disabled", fmt.Sprintf("provides %q clashes with %s", key, owner)
			r.report(p, p.Disabled)
		} else {
			r.byKey[key] = m.Name
		}
	}
	r.plugins[m.Name] = p
	return nil
}

// RegisterTool attaches a tool implementation to a plugin — the second
// registration of a tool name disables the plugin (spec 9.1).
func (r *Registry) RegisterTool(pluginName string, t core.Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.plugins[pluginName]
	if !ok {
		return fmt.Errorf("plugin %s not loaded", pluginName)
	}
	for _, owner := range r.plugins {
		if _, clash := owner.tools[t.Name()]; clash {
			p.State, p.Disabled = "disabled", fmt.Sprintf("tool %q clashes with %s", t.Name(), owner.Manifest.Name)
			r.report(p, p.Disabled)
			return nil
		}
	}
	p.tools[t.Name()] = t
	return nil
}

// RegisterChannel attaches a channel port.
func (r *Registry) RegisterChannel(pluginName string, c core.Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.plugins[pluginName]; ok {
		p.channel = c
	}
}

// RegisterSenses attaches sense ports.
func (r *Registry) RegisterSenses(pluginName string, ss []core.Sense) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.plugins[pluginName]; ok {
		p.senses = ss
	}
}

// RegisterContextProvider attaches a context provider port.
func (r *Registry) RegisterContextProvider(pluginName string, cp core.ContextProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.plugins[pluginName]; ok {
		p.ctx = cp
	}
}

// Lookup resolves a capability key for a calling plugin: the key must be in
// the caller's depends[], else ErrUndeclared (spec 9.1).
func (r *Registry) Lookup(caller, key string) (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[caller]
	if !ok {
		return nil, fmt.Errorf("plugin %s not loaded", caller)
	}
	declared := false
	for _, d := range p.Manifest.Depends {
		if d == key {
			declared = true
			break
		}
	}
	if !declared {
		return nil, fmt.Errorf("%w: %s -> %s", ErrUndeclared, caller, key)
	}
	owner, ok := r.byKey[key]
	if !ok {
		return nil, fmt.Errorf("no plugin provides %q", key)
	}
	return r.plugins[owner], nil
}

// Tool returns a registered tool by name ("" when absent or its plugin is
// disabled).
func (r *Registry) Tool(name string) core.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.plugins {
		if p.State != "loaded" {
			continue
		}
		if t, ok := p.tools[name]; ok {
			return t
		}
	}
	return nil
}

// ToolsForLayer lists live tool names whose Layers() metadata includes l —
// this generates the prompt action list from the registry (spec 9.1). Tools
// without Layers() metadata stay dispatchable but unadvertised.
func (r *Registry) ToolsForLayer(l int) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []string
	for _, p := range r.plugins {
		if p.State != "loaded" {
			continue
		}
		for n, t := range p.tools {
			if lt, ok := t.(interface{ Layers() []int }); ok {
				for _, tl := range lt.Layers() {
					if tl == l {
						out = append(out, n)
						break
					}
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

// ToolNames lists every live tool name, sorted.
func (r *Registry) ToolNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []string
	for _, p := range r.plugins {
		if p.State != "loaded" {
			continue
		}
		for n := range p.tools {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// List returns every plugin for the API (spec live check: GET /api/plugins).
func (r *Registry) List() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Manifest.Name < out[j].Manifest.Name })
	return out
}

// Disable marks a plugin down and reports it (spec 9.1: kind=plugin_disabled).
func (r *Registry) Disable(name, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.plugins[name]; ok && p.State != "disabled" {
		p.State, p.Disabled = "disabled", reason
		r.report(p, reason)
	}
}

func (r *Registry) report(p *Plugin, reason string) {
	if r.onEvent != nil {
		r.onEvent("plugin_disabled", fmt.Sprintf("plugin %s disabled: %s", p.Manifest.Name, reason))
	}
}

// Builtin registers an in-process plugin (no plugin.toml on disk): its
// manifest is supplied directly.
func (r *Registry) Builtin(m Manifest) (*Plugin, error) {
	if err := r.add(m); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.plugins[m.Name], nil
}

// rejectCycles topological-checks depends[] edges that point at provides[]
// keys owned by other plugins in this load set.
func rejectCycles(ms []Manifest) error {
	owner := map[string]string{}
	for _, m := range ms {
		for _, k := range m.Provides {
			owner[k] = m.Name
		}
	}
	edges := map[string][]string{}
	for _, m := range ms {
		for _, d := range m.Depends {
			if o, ok := owner[d]; ok && o != m.Name {
				edges[m.Name] = append(edges[m.Name], o)
			}
		}
	}
	const white, gray, black = 0, 1, 2
	color := map[string]int{}
	var stack []string
	var visit func(n string) error
	visit = func(n string) error {
		color[n] = gray
		stack = append(stack, n)
		for _, dep := range edges[n] {
			if color[dep] == gray {
				for i, s := range stack {
					if s == dep {
						return fmt.Errorf("plugin dependency cycle: %s", strings.Join(append(stack[i:], dep), " -> "))
					}
				}
				return fmt.Errorf("plugin dependency cycle through %s", dep)
			}
			if color[dep] == white {
				if err := visit(dep); err != nil {
					return err
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[n] = black
		return nil
	}
	for _, m := range ms {
		if color[m.Name] == white {
			if err := visit(m.Name); err != nil {
				return err
			}
		}
	}
	return nil
}
