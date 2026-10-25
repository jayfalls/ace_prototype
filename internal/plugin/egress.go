package plugin

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// ErrEgressDenied fires before any dial when the target host is off the
// mission allowlist (spec 9.1).
var ErrEgressDenied = errors.New("egress denied: host off allowlist")

// Egress is the one outbound policy point: every plugin HTTP client, and the
// provider adapters, go through it.
type Egress struct {
	allow map[string]bool // exact host or leading-dot suffix (.example.com)
}

// NewEgress builds an allowlist; "*" permits all (default missions without
// egress config).
func NewEgress(allow []string) *Egress {
	e := &Egress{allow: map[string]bool{}}
	for _, h := range allow {
		e.allow[strings.ToLower(strings.TrimSpace(h))] = true
	}
	return e
}

// Allowed reports whether the host (no port) is on the list.
func (e *Egress) Allowed(host string) bool {
	if e == nil || e.allow["*"] {
		return true
	}
	host = strings.ToLower(host)
	if e.allow[host] {
		return true
	}
	for h := range e.allow {
		if strings.HasPrefix(h, ".") && strings.HasSuffix(host, h) {
			return true
		}
	}
	return false
}

// Check returns ErrEgressDenied for an off-list URL — call before dialling.
func (e *Egress) Check(rawurl string) error {
	if e == nil {
		return nil
	}
	host := hostOf(rawurl)
	if e.Allowed(host) {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrEgressDenied, host)
}

func hostOf(rawurl string) string {
	h := rawurl
	if i := strings.Index(h, "://"); i >= 0 {
		h = h[i+3:]
	}
	if i := strings.IndexAny(h, "/?#"); i >= 0 {
		h = h[:i]
	}
	if host, _, err := net.SplitHostPort(h); err == nil {
		return host
	}
	return h
}

// HTTPClient returns an http.Client whose transport checks the allowlist on
// every request — off-list requests fail inside RoundTrip before any dial.
func (e *Egress) HTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: &egressTransport{e: e, base: http.DefaultTransport},
	}
}

type egressTransport struct {
	e    *Egress
	base http.RoundTripper
}

func (t *egressTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.e != nil && !t.e.Allowed(req.URL.Hostname()) {
		return nil, fmt.Errorf("%w: %s", ErrEgressDenied, req.URL.Hostname())
	}
	return t.base.RoundTrip(req)
}

// Broker injects declared secrets as headers at call time — the plugin never
// sees the raw value (spec 9.1).
type Broker struct {
	getenv func(string) string
}

// NewBroker wraps an env lookup (os.Getenv in production).
func NewBroker(getenv func(string) string) *Broker {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	return &Broker{getenv: getenv}
}

// InjectHeaders returns the headers for a plugin's declared secrets: the
// env var name maps to "x-secret-<lowername>" — never into args or a body.
func (b *Broker) InjectHeaders(secrets []string) map[string]string {
	out := map[string]string{}
	for _, s := range secrets {
		if v := b.getenv(s); v != "" {
			out["x-secret-"+strings.ToLower(strings.ReplaceAll(s, "_", "-"))] = v
		}
	}
	return out
}

// SecretVisible reports whether a raw secret value leaked into the argument
// payload — call sites assert this stays false.
func (b *Broker) SecretVisible(secrets []string, payload []byte) bool {
	for _, s := range secrets {
		if v := b.getenv(s); v != "" && strings.Contains(string(payload), v) {
			return true
		}
	}
	return false
}

// contextKey marks brokered headers travelling with a request context.
type brokerKey struct{}

// WithSecrets attaches brokered headers to a context for the egress client.
func WithSecrets(ctx context.Context, h map[string]string) context.Context {
	return context.WithValue(ctx, brokerKey{}, h)
}
