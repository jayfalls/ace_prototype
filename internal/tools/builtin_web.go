package tools

import (
	"context"
	"fmt"
	"strings"

	"ace/internal/core"
	"ace/internal/plugin"
)

// WebDeps is what the built-in web tools need from the host: a sandbox for
// the publish/mail shells plus the layer gate on who may call out.
type WebDeps struct {
	Sandbox core.Sandbox
}

// RegisterWebTools adds the five outbound tools to a plugin (spec 9.1 move:
// web, search and launch tools become built-in plugins). Layer restrictions
// come from Meta.Layer — the engine tags it per call.
func RegisterWebTools(reg *plugin.Registry, pluginName string, d WebDeps) {
	add := func(t core.Tool) { _ = reg.RegisterTool(pluginName, t) }
	add(&FnTool{NameV: "web_search", ReadOnlyV: true, LayersV: []int{2, 3, 4, 6}, Fn: func(ctx context.Context, a Args) (string, error) {
		m := MetaFrom(ctx)
		if m.Layer == 1 || m.Layer == 5 {
			return "web_search restricted to L2, L3, L4, and L6", nil
		}
		query := strings.TrimSpace(a.Target)
		if query == "" {
			query = strings.TrimSpace(a.Content)
		}
		resText, err := SafeWebSearch(ctx, query)
		if err != nil {
			return "web_search error: " + err.Error(), nil
		}
		if m.Emit != nil {
			m.Emit(fmt.Sprintf("🌐 Live Web Search (`%s`):\n```\n%s\n```", query, truncate(resText, 600)))
		}
		return TagUntrusted(resText), nil
	}})
	add(&FnTool{NameV: "web_fetch", ReadOnlyV: true, LayersV: []int{2, 3, 4, 6}, Fn: func(ctx context.Context, a Args) (string, error) {
		m := MetaFrom(ctx)
		if m.Layer == 1 || m.Layer == 5 {
			return "web_fetch restricted to L2, L3, L4, and L6", nil
		}
		targetURL := strings.TrimSpace(a.Target)
		if targetURL == "" {
			targetURL = strings.TrimSpace(a.Content)
		}
		resText, err := SafeWebFetch(ctx, targetURL)
		if err != nil {
			return "web_fetch error: " + err.Error(), nil
		}
		if m.Emit != nil {
			m.Emit(fmt.Sprintf("🌐 Fetched URL (`%s`):\n```\n%s\n```", targetURL, truncate(resText, 500)))
		}
		return TagUntrusted(resText), nil
	}})
	add(&FnTool{NameV: "telegraph_publish", Fn: func(ctx context.Context, a Args) (string, error) {
		m := MetaFrom(ctx)
		if m.Layer != 6 {
			return "telegraph_publish restricted to L6", nil
		}
		resText, err := PublishToTelegraph(ctx, d.Sandbox, a.Target, a.Content)
		if err != nil {
			return "telegraph_publish error: " + err.Error(), nil
		}
		if m.Emit != nil {
			m.Emit("🚀 " + resText)
		}
		return resText, nil
	}})
	add(&FnTool{NameV: "mail_tm", Fn: func(ctx context.Context, a Args) (string, error) {
		m := MetaFrom(ctx)
		if m.Layer != 6 {
			return "mail_tm restricted to L6", nil
		}
		resText, err := ExecuteMailTm(ctx, d.Sandbox, a.Target)
		if err != nil {
			return "mail_tm error: " + err.Error(), nil
		}
		if m.Emit != nil {
			m.Emit("📬 " + resText)
		}
		return resText, nil
	}})
	add(&FnTool{NameV: "http_request", LayersV: []int{6}, Fn: func(ctx context.Context, a Args) (string, error) {
		m := MetaFrom(ctx)
		if m.Layer != 6 {
			return "http_request restricted to L6", nil
		}
		resText, err := SafeHTTPRequest(ctx, a.Target, a.Content)
		if err != nil {
			return "http_request error: " + err.Error(), nil
		}
		resText = TagUntrusted(resText)
		if m.Emit != nil {
			m.Emit(fmt.Sprintf("🌍 Outbound API Request (`%s`):\n```\n%s\n```", a.Target, truncate(resText, 400)))
		}
		return resText, nil
	}})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
