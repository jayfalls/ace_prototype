package cognitive

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSafeWebFetchBlocksLocalhostAndPrivateIPs(t *testing.T) {
	_, err := SafeWebFetch(context.Background(), "http://localhost:8080/api/state")
	require.Error(t, err)
	_, err = SafeWebFetch(context.Background(), "http://127.0.0.1:8080/api/state")
	require.Error(t, err)
	_, err = SafeWebFetch(context.Background(), "file:///etc/passwd")
	require.Error(t, err)
}

func TestHTMLToStructuredMarkdownAndFragments(t *testing.T) {
	rawHTML := `<html><body><script>evil()</script><h1>Agent Architecture</h1><p>Read <a href="https://example.org/spec">Spec</a> with <code>bwrap</code>.</p><pre><code>def run():
    return 42</code></pre><ul><li>Memory tier 1</li><li>Memory tier 2</li></ul><table><tr><th>Name</th><th>Score</th></tr><tr><td>ACE</td><td>99</td></tr></table></body></html>`
	md := htmlToStructuredMarkdown(rawHTML)
	require.Contains(t, md, "# Agent Architecture")
	require.Contains(t, md, "[Spec](https://example.org/spec)")
	require.Contains(t, md, "`bwrap`")
	require.Contains(t, md, "```\ndef run():\n    return 42\n```")
	require.Contains(t, md, "- Memory tier 1")
	require.Contains(t, md, "| ACE | 99 |")
	require.NotContains(t, md, "evil()")

	sliced := applyFetchFragment("https://example.org/doc", 200, md, "grep=Memory")
	require.Contains(t, sliced, "Memory tier 1")
}
