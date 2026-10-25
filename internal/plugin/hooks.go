package plugin

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// Hook is one lifecycle extension point — at most 8 exist (spec 9.1).
type Hook string

const (
	HookPreTool    Hook = "pre_tool"
	HookPostTool   Hook = "post_tool"
	HookPreModel   Hook = "pre_model"
	HookPostModel  Hook = "post_model"
	HookOnBus      Hook = "on_bus"
	HookOnCompact  Hook = "on_compact"
	HookOnTaskDone Hook = "on_task_done"
	HookOnStart    Hook = "on_start"
)

// HookEvent is the payload handed to a hook handler.
type HookEvent struct {
	Hook    Hook              `json:"hook"`
	Layer   string            `json:"layer,omitempty"`
	CallID  string            `json:"call_id,omitempty"`
	Tool    string            `json:"tool,omitempty"`
	Payload string            `json:"payload,omitempty"`
	Meta    map[string]string `json:"meta,omitempty"`
}

// Handler is a plugin's hook callback; errors disable the plugin.
type Handler func(ctx context.Context, ev HookEvent) error

// Hooks dispatches lifecycle events to registered handlers in manifest order.
type Hooks struct {
	reg      *Registry
	handlers map[Hook][]struct {
		plugin string
		fn     Handler
	}
}

// NewHooks builds the dispatcher over a registry.
func NewHooks(reg *Registry) *Hooks {
	return &Hooks{reg: reg, handlers: map[Hook][]struct {
		plugin string
		fn     Handler
	}{}}
}

// On registers a plugin's handler for a hook.
func (h *Hooks) On(hook Hook, pluginName string, fn Handler) {
	h.handlers[hook] = append(h.handlers[hook], struct {
		plugin string
		fn     Handler
	}{pluginName, fn})
}

// Fire runs every handler for a hook; a handler error or panic disables its
// plugin and dispatch continues (spec 9.1).
func (h *Hooks) Fire(ctx context.Context, ev HookEvent) {
	for _, hh := range h.handlers[ev.Hook] {
		func() {
			defer func() {
				if r := recover(); r != nil {
					h.reg.Disable(hh.plugin, fmt.Sprintf("panic in %s hook: %v", ev.Hook, r))
				}
			}()
			if err := hh.fn(ctx, ev); err != nil {
				h.reg.Disable(hh.plugin, fmt.Sprintf("%s hook error: %v", ev.Hook, err))
			}
		}()
	}
}

// migrateModelRequests creates the pre_model invariant table (spec 9.1):
// every model request is reconstructable from the event log.
func migrateModelRequests(db *sql.DB) {
	if db == nil {
		return
	}
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS model_requests (
		call_id TEXT PRIMARY KEY,
		layer TEXT NOT NULL DEFAULT '',
		blocks_json_sha TEXT NOT NULL,
		injected_block_ids TEXT NOT NULL DEFAULT '[]',
		blocks_json TEXT NOT NULL DEFAULT '',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
}

// RecordModelRequest is the pre_model hook body: it writes the sha256 of the
// marshalled block list plus the ids actually injected. blocks_json keeps the
// payload so a later rebuild can diff bytes.
func RecordModelRequest(ctx context.Context, db *sql.DB, callID, layer string, blockIDs []string, blocks interface{}) error {
	if db == nil {
		return nil
	}
	raw, err := json.Marshal(blocks)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	ids, _ := json.Marshal(blockIDs)
	_, err = db.ExecContext(ctx, `INSERT OR REPLACE INTO model_requests (call_id, layer, blocks_json_sha, injected_block_ids, blocks_json)
		VALUES (?,?,?,?,?)`, callID, layer, hex.EncodeToString(sum[:]), string(ids), string(raw))
	return err
}

// RebuildModelRequest re-reads the logged blocks and verifies the sha —
// the test asserts byte-for-byte equality with what was sent.
func RebuildModelRequest(ctx context.Context, db *sql.DB, callID string) (json.RawMessage, []string, error) {
	var shaHex, idsJSON, blocksJSON string
	err := db.QueryRowContext(ctx, `SELECT blocks_json_sha, injected_block_ids, blocks_json FROM model_requests WHERE call_id=?`, callID).Scan(&shaHex, &idsJSON, &blocksJSON)
	if err != nil {
		return nil, nil, err
	}
	sum := sha256.Sum256([]byte(blocksJSON))
	if hex.EncodeToString(sum[:]) != shaHex {
		return nil, nil, fmt.Errorf("model_requests %s: sha mismatch — log corrupt", callID)
	}
	var ids []string
	_ = json.Unmarshal([]byte(idsJSON), &ids)
	return json.RawMessage(blocksJSON), ids, nil
}

// sortedBlockIDs gives a stable order for injected id logging.
func sortedBlockIDs(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
