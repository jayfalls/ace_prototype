package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// wired builds the same middleware stack Server.Handler() uses, but around a
// stub inner handler so tests exercise auth/CORS without needing live deps.
func wired(token string) http.Handler {
	s := &Server{authToken: token}
	stub := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return corsMiddleware(s.mutatingAuth(stub))
}

func TestMutatingRoutesRequireBearer(t *testing.T) {
	h := wired("secret-token-123")

	// 1. Mutating POST without a token is rejected before reaching the handler.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/api/engine/mode", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// 2. Wrong token is rejected.
	rec = httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/engine/mode", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// 3. Correct token passes.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/engine/mode", nil)
	req.Header.Set("Authorization", "Bearer secret-token-123")
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// 4. GET routes stay readable without a token.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/thoughts", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	// 5. Non-API paths are not gated (static WebUI assets).
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/showcase/img.png", nil))
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestSameOriginWebUIPassesWithoutToken(t *testing.T) {
	h := wired("secret-token-123")

	// Browser same-origin POST (Origin host == request Host) passes auth.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/engine/mode", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Origin", "http://localhost:8080")
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// Same-origin via Referer when Origin is absent (older browsers).
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/engine/mode", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Referer", "http://localhost:8080/")
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	// Cross-origin browser POST without token is rejected.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/api/engine/mode", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Origin", "https://evil.example.com")
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCORSPreflightDoesNotBypassAndWildcardGone(t *testing.T) {
	h := wired("secret-token-123")

	// OPTIONS preflight on a foreign origin: no wildcard, no reflected origin,
	// and the subsequent real request is still authed.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/api/engine/mode", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))

	// Same-origin preflight reflects the origin explicitly.
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("OPTIONS", "/api/engine/mode", nil)
	req.Host = "localhost:8080"
	req.Header.Set("Origin", "http://localhost:8080")
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "http://localhost:8080", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestEmptyTokenRejectsEverything(t *testing.T) {
	// Fails closed: a server constructed without a token accepts no bearer at all.
	h := wired("")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/engine/mode", nil)
	req.Header.Set("Authorization", "Bearer anything")
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
