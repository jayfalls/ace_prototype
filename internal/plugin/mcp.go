package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"ace/internal/core"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

// mcpTool adapts one remote MCP tool to core.Tool. Schema is the server's
// declared inputSchema verbatim; ReadOnly honours the readOnlyHint annotation.
type mcpTool struct {
	name     string
	schema   json.RawMessage
	readOnly bool
	client   *mcpclient.Client
}

func (t *mcpTool) Name() string            { return t.name }
func (t *mcpTool) Schema() json.RawMessage { return t.schema }
func (t *mcpTool) ReadOnly() bool          { return t.readOnly }

// Layers advertises remote tools at L6 (the tool-using layer) so a
// manifest-added tool is actually callable — without it the registry
// dispatch can reach it but the prompt never offers it.
func (t *mcpTool) Layers() []int { return []int{6} }

func (t *mcpTool) Call(ctx context.Context, args json.RawMessage) (core.ToolResult, error) {
	var a map[string]any
	if len(args) > 0 {
		if err := json.Unmarshal(args, &a); err != nil {
			return core.ToolResult{}, fmt.Errorf("mcp tool %s: bad args: %w", t.name, err)
		}
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = t.name
	req.Params.Arguments = a
	res, err := t.client.CallTool(ctx, req)
	if err != nil {
		return core.ToolResult{}, err
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := mcp.AsTextContent(c); ok {
			b.WriteString(tc.Text)
		}
	}
	out := core.ToolResult{Content: b.String()}
	if res.IsError {
		out.Error = out.Content
	}
	return out, nil
}

// LoadMCP starts an mcp:<command> entry, lists its tools and returns one
// mcpTool per remote tool — each registered by the caller as a plugin Tool
// (spec 9.1: manifest-only tool add, no Go change).
func LoadMCP(ctx context.Context, entry string) ([]core.Tool, func(), error) {
	cmdline := strings.TrimPrefix(entry, "mcp:")
	parts := strings.Fields(cmdline)
	if len(parts) == 0 {
		return nil, nil, fmt.Errorf("empty mcp entry")
	}
	cli, err := mcpclient.NewStdioMCPClient(parts[0], nil, parts[1:]...)
	if err != nil {
		return nil, nil, fmt.Errorf("mcp start %q: %w", parts[0], err)
	}
	cleanup := func() { _ = cli.Close() }
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{Name: "ace", Version: "0.9"}
	if _, err := cli.Initialize(ctx, initReq); err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("mcp initialize: %w", err)
	}
	tools, err := cli.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("mcp tools/list: %w", err)
	}
	out := make([]core.Tool, 0, len(tools.Tools))
	for _, t := range tools.Tools {
		schema, _ := json.Marshal(t.InputSchema)
		out = append(out, &mcpTool{
			name:     t.Name,
			schema:   schema,
			readOnly: t.Annotations.ReadOnlyHint != nil && *t.Annotations.ReadOnlyHint,
			client:   cli,
		})
	}
	return out, cleanup, nil
}
