package plugin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"ace/internal/core"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	_ "modernc.org/sqlite"
)

func writeManifest(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

type stubTool struct{ name string }

func (s *stubTool) Name() string            { return s.name }
func (s *stubTool) Schema() json.RawMessage { return json.RawMessage(`{}`) }
func (s *stubTool) ReadOnly() bool          { return true }
func (s *stubTool) Call(ctx context.Context, a json.RawMessage) (core.ToolResult, error) {
	return core.ToolResult{Content: "ok"}, nil
}

// 1. A manifest dependency cycle must fail Load naming the cycle.
func TestLoadRejectsDependencyCycle(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "plug-a", `name = "plug-a"
kind = "tool"
provides = ["cap.a"]
depends = ["cap.b"]
`)
	writeManifest(t, root, "plug-b", `name = "plug-b"
kind = "tool"
provides = ["cap.b"]
depends = ["cap.a"]
`)
	err := New(nil).Load(root)
	if err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected cycle error, got %v", err)
	}
}

// 2. Lookup of a capability not in depends[] must return ErrUndeclared.
func TestLookupUndeclaredDependency(t *testing.T) {
	root := t.TempDir()
	writeManifest(t, root, "plug-a", `name = "plug-a"
kind = "tool"
provides = ["cap.a"]
`)
	writeManifest(t, root, "plug-b", `name = "plug-b"
kind = "tool"
`)
	reg := New(nil)
	if err := reg.Load(root); err != nil {
		t.Fatal(err)
	}
	_, err := reg.Lookup("plug-b", "cap.a")
	if !errors.Is(err, ErrUndeclared) {
		t.Fatalf("expected ErrUndeclared, got %v", err)
	}
	// Declared lookup resolves to the providing plugin.
	writeManifest(t, root, "plug-c", `name = "plug-c"
kind = "tool"
depends = ["cap.a"]
`)
	reg2 := New(nil)
	if err := reg2.Load(root); err != nil {
		t.Fatal(err)
	}
	got, err := reg2.Lookup("plug-c", "cap.a")
	if err != nil {
		t.Fatal(err)
	}
	if got.(*Plugin).Manifest.Name != "plug-a" {
		t.Fatalf("lookup resolved to %v", got)
	}
}

// 3. A panicking tool disables its plugin; the next tool still runs.
func TestPanicIsolatesPlugin(t *testing.T) {
	reg := New(nil)
	_, _ = reg.Builtin(Manifest{Name: "bad", Kind: "tool"})
	_, _ = reg.Builtin(Manifest{Name: "good", Kind: "tool"})
	_ = reg.RegisterTool("bad", &panicTool{})
	_ = reg.RegisterTool("good", &stubTool{name: "fine"})
	reg.Isolate("bad")
	reg.Isolate("good")
	_, err := reg.Tool("boom").Call(context.Background(), nil)
	if err == nil {
		t.Fatal("expected panic to surface as error")
	}
	if reg.Tool("boom") != nil {
		t.Fatal("disabled plugin's tool should not resolve")
	}
	res, err := reg.Tool("fine").Call(context.Background(), nil)
	if err != nil || res.Content != "ok" {
		t.Fatalf("sibling plugin broken by panic: %v", err)
	}
}

type panicTool struct{ stubTool }

func (p *panicTool) Name() string { return "boom" }
func (p *panicTool) Call(ctx context.Context, a json.RawMessage) (core.ToolResult, error) {
	panic("detonate")
}

// 4. A duplicate tool name disables the second registrant.
func TestDuplicateToolNameDisablesSecond(t *testing.T) {
	reg := New(nil)
	_, _ = reg.Builtin(Manifest{Name: "one", Kind: "tool"})
	_, _ = reg.Builtin(Manifest{Name: "two", Kind: "tool"})
	_ = reg.RegisterTool("one", &stubTool{name: "dup"})
	_ = reg.RegisterTool("two", &stubTool{name: "dup"})
	for _, p := range reg.List() {
		if p.Manifest.Name == "two" && p.State != "disabled" {
			t.Fatal("second registrant should be disabled")
		}
	}
}

// 5. MCP fixture server exposes echo -> callable with no Go change to ACE.
// The fixture is this test binary re-executed with MCP_FIXTURE=1, launched
// through a wrapper script (the realistic path for a manifest-declared
// mcp:<command> entry).
func TestMCPFixtureEcho(t *testing.T) {
	if os.Getenv("MCP_FIXTURE") == "1" {
		runEchoFixture()
		return
	}
	self, _ := os.Executable()
	dir := t.TempDir()
	wrap := filepath.Join(dir, "echo-server.sh")
	script := "#!/bin/sh\nexport MCP_FIXTURE=1\nexec " + self + " -test.run=TestMCPFixtureEcho\n"
	if err := os.WriteFile(wrap, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tools, cleanup, err := LoadMCP(ctx, "mcp:"+wrap)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	var echo core.Tool
	for _, tl := range tools {
		if tl.Name() == "echo" {
			echo = tl
		}
	}
	if echo == nil {
		t.Fatal("echo tool not listed by fixture")
	}
	res, err := echo.Call(ctx, json.RawMessage(`{"text":"ping"}`))
	if err != nil {
		t.Fatal(err)
	}
	if res.Content != "ping" {
		t.Fatalf("echo returned %q", res.Content)
	}
}

// runEchoFixture serves one MCP stdio echo tool on stdin/stdout.
func runEchoFixture() {
	s := server.NewMCPServer("echo-fixture", "0.0.1")
	s.AddTool(mcp.NewTool("echo", mcp.WithString("text")),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			text, _ := req.Params.Arguments.(map[string]any)["text"].(string)
			return mcp.NewToolResultText(text), nil
		})
	_ = server.NewStdioServer(s).Listen(context.Background(), os.Stdin, os.Stdout)
	os.Exit(0)
}

// 6. A request off the allowlist fails with ErrEgressDenied and the server
// never sees a connection.
func TestEgressDeniesOfflistHost(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()
	e := NewEgress([]string{"allowed.example.com"})
	err := e.Check(srv.URL)
	if !errors.Is(err, ErrEgressDenied) {
		t.Fatalf("expected ErrEgressDenied, got %v", err)
	}
	_, err = e.HTTPClient(2 * time.Second).Get(srv.URL)
	if !errors.Is(err, ErrEgressDenied) {
		t.Fatalf("transport should deny, got %v", err)
	}
	if atomic.LoadInt32(&hits) != 0 {
		t.Fatal("server saw a connection — deny happened after dial")
	}
}

// 7. A brokered secret arrives as a header; args/env never carry the value.
func TestSecretBrokerKeepsValueOutOfArgs(t *testing.T) {
	b := NewBroker(func(k string) string {
		if k == "API_TOKEN" {
			return "s3cr3t-value"
		}
		return ""
	})
	h := b.InjectHeaders([]string{"API_TOKEN", "MISSING"})
	if h["x-secret-api-token"] != "s3cr3t-value" {
		t.Fatalf("secret header missing: %v", h)
	}
	if _, ok := h["x-secret-missing"]; ok {
		t.Fatal("unset secret should produce no header")
	}
	payload := json.RawMessage(`{"target":"q","content":"body"}`)
	if b.SecretVisible([]string{"API_TOKEN"}, payload) {
		t.Fatal("secret leaked into args payload")
	}
}

// 8. Rebuilding a model request from the event log equals the sent bytes.
func TestModelRequestRebuildMatchesSent(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrateModelRequests(db)
	req := map[string]interface{}{
		"system_prompt": "sys",
		"messages":      []map[string]string{{"role": "user", "content": "hello"}},
	}
	ctx := context.Background()
	if err := RecordModelRequest(ctx, db, "call_1", "L6", []string{"system", "user:0"}, req); err != nil {
		t.Fatal(err)
	}
	blocks, ids, err := RebuildModelRequest(ctx, db, "call_1")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := json.Marshal(req)
	if string(blocks) != string(want) {
		t.Fatalf("rebuilt bytes differ:\n got %s\nwant %s", blocks, want)
	}
	if len(ids) != 2 || ids[0] != "system" {
		t.Fatalf("injected ids wrong: %v", ids)
	}
	// Tamper -> sha mismatch must fail the rebuild.
	_, _ = db.Exec(`UPDATE model_requests SET blocks_json='{"tampered":true}' WHERE call_id='call_1'`)
	if _, _, err := RebuildModelRequest(ctx, db, "call_1"); err == nil {
		t.Fatal("tampered row should fail sha verification")
	}
}
