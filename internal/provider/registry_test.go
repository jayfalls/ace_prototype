package provider

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"ace/internal/core"

	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	migrateUsage(db)
	t.Cleanup(func() { db.Close() })
	return db
}

// sseServer fakes an OpenAI-compatible streaming endpoint. status<200 means
// SSE content; non-200 emits the status + body once.
func sseServer(t *testing.T, hits *int64, status int, text string, usage bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			atomic.AddInt64(hits, 1)
		}
		if status != http.StatusOK {
			w.WriteHeader(status)
			fmt.Fprint(w, text)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", text)
		if usage {
			fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":7,\"prompt_cache_hit_tokens\":3}}\n\n")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}

func adapterFor(t *testing.T, g GroupCfg, srv *httptest.Server) Adapter {
	t.Helper()
	if srv != nil {
		g.BaseURL = srv.URL
	}
	a, err := NewOpenAICompatAdapter(g, "k", 5, "")
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func callMain(reg *Registry) error {
	_, err := reg.StreamComplete(context.Background(), core.CompletionRequest{
		Messages: []core.ChatTurn{{Role: "user", Content: "hi"}},
	}, nil)
	return err
}

// Spec 9.2 test 1: main on a subscription group hits 429-quota → error
// surfaces; the free group is never called.
func TestMainQuota429NeverFallsToFree(t *testing.T) {
	db := testDB(t)
	var paidHits, freeHits int64
	paid := sseServer(t, &paidHits, http.StatusTooManyRequests, "daily quota exceeded", false)
	defer paid.Close()
	free := sseServer(t, &freeHits, http.StatusOK, "ok", true)
	defer free.Close()

	cfg := Config{Groups: []GroupCfg{
		{Name: "paid", Adapter: "openai_compat", Priority: 1, Billing: "subscription", Models: []string{"m1"}},
		{Name: "free", Adapter: "openai_compat", Priority: 2, Billing: "free", Models: []string{"m1"}},
	}}
	cfg.Models.Main = "m1"
	reg, err := New(db, cfg, map[string]Adapter{
		"paid": adapterFor(t, cfg.Groups[0], paid),
		"free": adapterFor(t, cfg.Groups[1], free),
	})
	if err != nil {
		t.Fatal(err)
	}
	err = callMain(reg)
	var ce *Classified
	if err == nil || !errors.As(err, &ce) || ce.Class != ClassQuota {
		t.Fatalf("want quota-classified error, got %v", err)
	}
	if atomic.LoadInt64(&freeHits) != 0 {
		t.Fatalf("free group called %d times for main", freeHits)
	}
}

// Spec 9.2 test 2: a free model falls back only to free groups — the metered
// group serving the same id is never called.
func TestFreeFallsOnlyToFree(t *testing.T) {
	db := testDB(t)
	if _, err := db.Exec(`INSERT INTO models_catalogue (id, provider, price_in, price_out) VALUES ('fm','g',0,0)`); err != nil {
		t.Fatal(err)
	}
	var f1, f2, paid int64
	s1 := sseServer(t, &f1, http.StatusServiceUnavailable, "down", false)
	defer s1.Close()
	s2 := sseServer(t, &f2, http.StatusOK, "free-answer", true)
	defer s2.Close()
	sp := sseServer(t, &paid, http.StatusOK, "paid-answer", true)
	defer sp.Close()

	cfg := Config{Groups: []GroupCfg{
		{Name: "free1", Adapter: "openai_compat", Priority: 1, Billing: "free", Models: []string{"fm"}},
		{Name: "free2", Adapter: "openai_compat", Priority: 2, Billing: "free", Models: []string{"fm"}},
		{Name: "metered", Adapter: "openai_compat", Priority: 3, Billing: "metered", Models: []string{"fm"}},
	}}
	cfg.Models.Main = "fm"
	cfg.Retry.Base, cfg.Retry.Cap = 0.001, 0.002
	reg, err := New(db, cfg, map[string]Adapter{
		"free1":   adapterFor(t, cfg.Groups[0], s1),
		"free2":   adapterFor(t, cfg.Groups[1], s2),
		"metered": adapterFor(t, cfg.Groups[2], sp),
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := reg.Call(context.Background(), RoleTool, "fm", core.CompletionRequest{
		Messages: []core.ChatTurn{{Role: "user", Content: "hi"}},
	}, nil)
	if err != nil || resp.Content != "free-answer" {
		t.Fatalf("want free-answer, got %q err=%v", resp.Content, err)
	}
	if atomic.LoadInt64(&paid) != 0 {
		t.Fatal("metered group called for a free model")
	}
}

// Spec 9.2 test 3: three 503s (1 try + 2 retries) trip the breaker and the
// next group serves; a second trip doubles cooldown to 120 s.
func TestBreakerTripsAndNextGroupServes(t *testing.T) {
	db := testDB(t)
	var down, up int64
	s1 := sseServer(t, &down, http.StatusServiceUnavailable, "down", false)
	defer s1.Close()
	s2 := sseServer(t, &up, http.StatusOK, "backup", true)
	defer s2.Close()

	cfg := Config{Groups: []GroupCfg{
		{Name: "g1", Adapter: "openai_compat", Priority: 1, Billing: "metered", Models: []string{"m"}},
		{Name: "g2", Adapter: "openai_compat", Priority: 2, Billing: "metered", Models: []string{"m"}},
	}}
	cfg.Models.Main = "m"
	cfg.Retry.Max, cfg.Retry.Base, cfg.Retry.Cap = 2, 0.001, 0.002
	reg, err := New(db, cfg, map[string]Adapter{
		"g1": adapterFor(t, cfg.Groups[0], s1),
		"g2": adapterFor(t, cfg.Groups[1], s2),
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := reg.StreamComplete(context.Background(), core.CompletionRequest{
		Model:    "m",
		Messages: []core.ChatTurn{{Role: "user", Content: "hi"}},
	}, nil)
	if err != nil || resp.Content != "backup" {
		t.Fatalf("want backup via failover, got %q err=%v", resp.Content, err)
	}
	if got := atomic.LoadInt64(&down); got != 3 {
		t.Fatalf("want 3 attempts (1+2 retries), got %d", got)
	}
	if !reg.breakers.Open("g1", "m") {
		t.Fatal("breaker did not open after 3 transient failures")
	}
	if cd := reg.breakers.Trip("g1", "m"); cd != 120*time.Second {
		t.Fatalf("second trip must double 60s to 120s, got %v", cd)
	}
}

// Dead-port variant of test 3: a group whose port refuses the connection
// fails over to the next group inside one call (spec live-check shape).
func TestDeadPortFailsOverWithinOneCall(t *testing.T) {
	db := testDB(t)
	dead, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	deadAddr := dead.Addr().String()
	dead.Close() // port now refuses
	up := sseServer(t, nil, http.StatusOK, "backup", true)
	defer up.Close()

	cfg := Config{Groups: []GroupCfg{
		{Name: "dead", Adapter: "openai_compat", Priority: 1, Billing: "metered", Models: []string{"m"}, BaseURL: "http://" + deadAddr},
		{Name: "live", Adapter: "openai_compat", Priority: 2, Billing: "metered", Models: []string{"m"}},
	}}
	cfg.Models.Main = "m"
	cfg.Retry.Max, cfg.Retry.Base, cfg.Retry.Cap = 2, 0.001, 0.002
	reg, err := New(db, cfg, map[string]Adapter{
		"dead": adapterFor(t, cfg.Groups[0], nil),
		"live": adapterFor(t, cfg.Groups[1], up),
	})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := reg.StreamComplete(context.Background(), core.CompletionRequest{
		Messages: []core.ChatTurn{{Role: "user", Content: "hi"}},
	}, nil)
	if err != nil || resp.Content != "backup" {
		t.Fatalf("dead port must fail over, got %q err=%v", resp.Content, err)
	}
	if !reg.breakers.Open("dead", "m") {
		t.Fatal("dead port did not trip the breaker")
	}
}

// Spec 9.2 test 4: a 400 (invalid) is never retried.
func TestInvalid400NoRetry(t *testing.T) {
	db := testDB(t)
	var hits int64
	s := sseServer(t, &hits, http.StatusBadRequest, "bad request", false)
	defer s.Close()
	cfg := Config{Groups: []GroupCfg{
		{Name: "g", Adapter: "openai_compat", Priority: 1, Billing: "metered", Models: []string{"m"}},
	}}
	cfg.Models.Main = "m"
	cfg.Retry.Max = 2
	reg, err := New(db, cfg, map[string]Adapter{"g": adapterFor(t, cfg.Groups[0], s)})
	if err != nil {
		t.Fatal(err)
	}
	if err := callMain(reg); err == nil {
		t.Fatal("want error")
	}
	if got := atomic.LoadInt64(&hits); got != 1 {
		t.Fatalf("400 must not retry; got %d calls", got)
	}
}

// Spec 9.2 test 5: a response without usage stores NULL token columns.
func TestMissingUsageStoresNULL(t *testing.T) {
	db := testDB(t)
	s := sseServer(t, nil, http.StatusOK, "no-usage", false)
	defer s.Close()
	cfg := Config{Groups: []GroupCfg{
		{Name: "g", Adapter: "openai_compat", Priority: 1, Billing: "metered", Models: []string{"m"}},
	}}
	cfg.Models.Main = "m"
	reg, err := New(db, cfg, map[string]Adapter{"g": adapterFor(t, cfg.Groups[0], s)})
	if err != nil {
		t.Fatal(err)
	}
	if err := callMain(reg); err != nil {
		t.Fatal(err)
	}
	var p, c, o interface{}
	if err := db.QueryRow(`SELECT prompt_tokens, cached_tokens, completion_tokens FROM usage_events ORDER BY id DESC LIMIT 1`).Scan(&p, &c, &o); err != nil {
		t.Fatal(err)
	}
	if p != nil || c != nil || o != nil {
		t.Fatalf("want NULL tokens, got %v %v %v", p, c, o)
	}
}

// Spec 9.2 test 6: catalogue removal emits churn and reroutes the role to a
// remaining model.
func TestCatalogueRemovalChurnsAndReroutes(t *testing.T) {
	db := testDB(t)
	rows := []CatalogueRow{
		{ID: "m1", Provider: "g"}, {ID: "m2", Provider: "g"},
	}
	if _, err := SyncCatalogue(context.Background(), db, rows); err != nil {
		t.Fatal(err)
	}
	cfg := Config{Groups: []GroupCfg{
		{Name: "g", Adapter: "openai_compat", Priority: 1, Billing: "metered", Models: []string{"m1", "m2"}},
	}}
	cfg.Models.Main = "m1"
	var churned ChurnEvent
	reg, err := New(db, cfg, map[string]Adapter{"g": adapterFor(t, cfg.Groups[0], sseServer(t, nil, http.StatusOK, "x", true))})
	if err != nil {
		t.Fatal(err)
	}
	reg.OnChurn(func(ev ChurnEvent) { churned = ev })
	if _, err := reg.SyncCatalogue(context.Background(), []CatalogueRow{{ID: "m2", Provider: "g"}}); err != nil {
		t.Fatal(err)
	}
	if len(churned.Removed) == 0 || churned.Removed[0] != "g/m1" {
		t.Fatalf("want g/m1 churn, got %+v", churned)
	}
	if got := reg.ActiveModel(); got != "m2" {
		t.Fatalf("role must reroute to remaining m2, got %q", got)
	}
}

// Spec 9.2 test 7 lives in internal/crypto/field_test.go (round-trip +
// tamper failure against the real AES-256-GCM envelope).
