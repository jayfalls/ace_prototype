package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ace/internal/channels"
	"ace/internal/cognitive"
	"ace/internal/core"
	"ace/internal/sandbox"
)

type Server struct {
	engine    *cognitive.Engine
	llm       core.LLMProvider
	bus       core.Bus
	memory    core.MemoryStore
	jail      *sandbox.Jail
	telem     core.TelemetrySink
	telegram  *channels.TelegramBridge
	webFS     fs.FS
	authToken string
}

func NewServer(
	engine *cognitive.Engine,
	llm core.LLMProvider,
	bus core.Bus,
	mem core.MemoryStore,
	jail *sandbox.Jail,
	telem core.TelemetrySink,
	telegram *channels.TelegramBridge,
	webFS fs.FS,
	authToken string,
) *Server {
	return &Server{
		engine:    engine,
		llm:       llm,
		bus:       bus,
		memory:    mem,
		jail:      jail,
		telem:     telem,
		telegram:  telegram,
		webFS:     webFS,
		authToken: authToken,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/state", s.handleGetState)
	mux.HandleFunc("GET /api/v07/cockpit", s.handleV07Cockpit)
	mux.HandleFunc("POST /api/chat", s.handleChat)
	mux.HandleFunc("POST /api/engine/mode", s.handleSetMode)
	mux.HandleFunc("POST /api/engine/cycle", s.handleTriggerCycle)
	mux.HandleFunc("POST /api/engine/step", s.handleStepLayer)
	mux.HandleFunc("POST /api/engine/stop", s.handleStopCycle)
	mux.HandleFunc("POST /api/models/select", s.handleSelectModel)

	mux.HandleFunc("GET /api/thoughts", s.handleListThoughts)
	mux.HandleFunc("GET /api/thoughts/{id}", s.handleGetThought)
	mux.HandleFunc("GET /api/telemetry/spans", s.handleListSpans)
	mux.HandleFunc("GET /api/telemetry/metrics", s.handleGetMetrics)
	mux.HandleFunc("GET /api/telemetry/prometheus", s.handlePrometheusMetrics)
	mux.HandleFunc("GET /api/telemetry/timeseries", s.handleTimeSeries)
	mux.HandleFunc("GET /api/harness/d", s.handleHarnessD)
	mux.HandleFunc("GET /metrics", s.handlePrometheusMetrics)
	mux.HandleFunc("POST /api/senses/webhook", s.handleWebhookSense)

	mux.HandleFunc("GET /api/wiki/read", s.handleReadWiki)
	mux.HandleFunc("POST /api/wiki/write", s.handleWriteWiki)

	mux.HandleFunc("GET /api/memory/search", s.handleSearchMemory)
	mux.HandleFunc("POST /api/memory/remember", s.handleRemember)

	mux.HandleFunc("POST /api/sandbox/verify", s.handleVerifySandbox)
	mux.HandleFunc("POST /api/sandbox/exec", s.handleSandboxExec)
	mux.HandleFunc("GET /api/sandbox/file", s.handleReadWorkspaceFile)

	mux.HandleFunc("GET /api/telegram/status", s.handleTelegramStatus)
	mux.HandleFunc("POST /api/telegram/configure", s.handleTelegramConfigure)
	mux.HandleFunc("POST /api/telegram/test", s.handleTelegramTest)

	mux.HandleFunc("GET /showcase/{path...}", s.handleShowcaseFile)

	mux.HandleFunc("GET /api/events", s.handleSSE)

	if s.webFS != nil {
		fileServer := http.FileServer(http.FS(s.webFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}
			if _, err := fs.Stat(s.webFS, path); err != nil {
				r.URL.Path = "/"
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	return corsMiddleware(s.mutatingAuth(mux))
}

func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	engStatus := s.engine.Status()
	chats, _ := s.memory.RecentChatMessages(ctx, 60)
	thoughts, _ := s.telem.RecentThoughts(ctx, 50, 0)
	busEvents, _ := s.bus.Recent(60)
	memories, _ := s.memory.RecentMemories(ctx, 30)
	wikiPages, _ := s.memory.ListWiki()
	wsFiles, _ := s.jail.ListFiles()
	recentExecs := s.jail.RecentExecs(20)
	spans, _ := s.telem.RecentSpans(ctx, 80, "")

	var tgStatus interface{}
	if s.telegram != nil {
		tgStatus = s.telegram.Status()
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"engine":          engStatus,
		"models":          s.llm.Models(),
		"telegram":        tgStatus,
		"chat_messages":   nonNilSlice(chats),
		"thoughts":        nonNilSlice(thoughts),
		"telemetry_spans": nonNilSlice(spans),
		"bus_events":      nonNilSlice(busEvents),
		"memories":        nonNilSlice(memories),
		"wiki_pages":      nonNilSlice(wikiPages),
		"workspace_files": nonNilSlice(wsFiles),
		"recent_execs":    nonNilSlice(recentExecs),
		"v07_cockpit":     s.engine.BuildV07CockpitSnapshot(ctx),
	})
}

func (s *Server) handleV07Cockpit(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.engine.BuildV07CockpitSnapshot(r.Context()))
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	msg, err := s.engine.Chat(r.Context(), body.Content)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, msg)
}

func (s *Server) handleSetMode(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Mode            string `json:"mode"`
		IntervalSeconds int    `json:"interval_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	st := s.engine.SetMode(core.EngineMode(body.Mode), body.IntervalSeconds)
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleTriggerCycle(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Trigger    string `json:"trigger"`
		StartLayer int    `json:"start_layer"`
		MaxHops    int    `json:"max_hops"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if strings.TrimSpace(body.Trigger) == "" {
		body.Trigger = "Manual cognitive cascade triggered from Glass-Box Cockpit: evaluate mission, review user goals, update Wiki/memory, and prosecute top priority task."
	}
	if body.StartLayer <= 0 {
		body.StartLayer = int(core.LayerExecutive)
	}
	if body.MaxHops <= 0 {
		body.MaxHops = 3
	}
	cycleID := s.engine.EnqueueCycle(body.Trigger, core.LayerID(body.StartLayer), body.MaxHops)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"cycle_id": cycleID,
		"engine":   s.engine.Status(),
	})
}

func (s *Server) handleStepLayer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LayerID  int    `json:"layer_id"`
		Stimulus string `json:"stimulus"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.LayerID < 1 || body.LayerID > 6 {
		body.LayerID = int(core.LayerExecutive)
	}
	trace, err := s.engine.StepSingleLayer(r.Context(), core.LayerID(body.LayerID), body.Stimulus)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, trace)
}

func (s *Server) handleStopCycle(w http.ResponseWriter, r *http.Request) {
	st := s.engine.StopCurrentCycle()
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleSelectModel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ModelID string `json:"model_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.llm.SetActiveModel(body.ModelID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	st := s.engine.Status()
	s.telem.EmitLive("engine.status", st)
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleListThoughts(w http.ResponseWriter, r *http.Request) {
	layerFilter, _ := strconv.Atoi(r.URL.Query().Get("layer"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	thoughts, err := s.telem.RecentThoughts(r.Context(), limit, layerFilter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, nonNilSlice(thoughts))
}

func (s *Server) handleGetThought(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := s.telem.GetThought(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) handleListSpans(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	traceID := r.URL.Query().Get("trace_id")
	spans, err := s.telem.RecentSpans(r.Context(), limit, traceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, nonNilSlice(spans))
}

func (s *Server) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.telem.Metrics(r.Context()))
}

func (s *Server) handleWebhookSense(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Source  string `json:"source"`
		Event   string `json:"event"`
		Payload string `json:"payload"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Source == "" {
		body.Source = "external_webhook"
	}
	cycleID := s.engine.EnqueueCycle("Webhook Sense ["+body.Source+"/"+body.Event+"]: "+body.Payload, core.LayerAspirational, 6)
	writeJSON(w, http.StatusOK, map[string]string{
		"status":   "accepted",
		"cycle_id": cycleID,
	})
}

func (s *Server) handleReadWiki(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	content, err := s.memory.ReadWiki(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"path":    path,
		"content": content,
	})
}

func (s *Server) handleWriteWiki(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.memory.WriteWiki(body.Path, body.Content); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	pages, _ := s.memory.ListWiki()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"path":       body.Path,
		"wiki_pages": nonNilSlice(pages),
	})
}

func (s *Server) handleSearchMemory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	hits, err := s.memory.Search(r.Context(), q, 30)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, nonNilSlice(hits))
}

func (s *Server) handleRemember(w http.ResponseWriter, r *http.Request) {
	var body core.MemoryEntry
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.LayerCode == "" {
		body.LayerCode = "USER"
	}
	if body.Category == "" {
		body.Category = "note"
	}
	entry, err := s.memory.Remember(r.Context(), body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) handleVerifySandbox(w http.ResponseWriter, r *http.Request) {
	st := s.jail.Verify(r.Context())
	s.telem.EmitLive("engine.status", s.engine.Status())
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleSandboxExec(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	res, _ := s.jail.Exec(r.Context(), body.Command)
	s.telem.EmitLive("sandbox.exec", res)
	wsFiles, _ := s.jail.ListFiles()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"result":          res,
		"workspace_files": nonNilSlice(wsFiles),
	})
}

func (s *Server) handleReadWorkspaceFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	content, err := s.jail.ReadFile(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"path":    path,
		"content": content,
	})
}

func (s *Server) handleShowcaseFile(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if strings.TrimSpace(path) == "" {
		path = "model-viz.html"
	}
	content, err := s.jail.ReadFile(path)
	if err != nil {
		http.Error(w, "Showcase artifact not found: "+err.Error(), http.StatusNotFound)
		return
	}
	switch {
	case strings.HasSuffix(path, ".html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case strings.HasSuffix(path, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(path, ".js"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case strings.HasSuffix(path, ".json"):
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	_, _ = w.Write([]byte(content))
}

func (s *Server) handlePrometheusMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	if promSink, ok := s.telem.(interface {
		FormatPrometheusMetrics(context.Context, []core.ModelInfo, core.EngineStatus) string
	}); ok {
		_, _ = w.Write([]byte(promSink.FormatPrometheusMetrics(r.Context(), s.llm.Models(), s.engine.Status())))
		return
	}
	m := s.telem.Metrics(r.Context())
	_, _ = fmt.Fprintf(w, "ace_tokens_total{kind=\"prompt\"} %d\nace_tokens_total{kind=\"cached_prefix\"} %d\n", m.TotalPromptTokens, m.TotalCachedTokens)
}

func (s *Server) handleTimeSeries(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if tsSink, ok := s.telem.(interface {
		RecentTimeSeries(context.Context, int) ([]map[string]interface{}, error)
	}); ok {
		series, err := tsSink.RecentTimeSeries(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"series": series,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"series": []interface{}{}})
}

func (s *Server) handleHarnessD(w http.ResponseWriter, r *http.Request) {
	paretoRaw, _ := s.jail.ReadFile(".harness_d/pareto.json")
	scoreboardRaw, _ := s.jail.ReadFile(".harness_d/scoreboard.tsv")
	changeCtrlRaw, _ := s.jail.ReadFile(".harness_d/change_control.json")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"pareto":         paretoRaw,
		"scoreboard_tsv": scoreboardRaw,
		"change_control": changeCtrlRaw,
	})
}

func (s *Server) handleTelegramStatus(w http.ResponseWriter, r *http.Request) {
	if s.telegram == nil {
		writeError(w, http.StatusServiceUnavailable, "telegram bridge not initialized")
		return
	}
	writeJSON(w, http.StatusOK, s.telegram.Status())
}

func (s *Server) handleTelegramConfigure(w http.ResponseWriter, r *http.Request) {
	if s.telegram == nil {
		writeError(w, http.StatusServiceUnavailable, "telegram bridge not initialized")
		return
	}
	var body struct {
		Token            string `json:"token"`
		AllowedChatIDs   string `json:"allowed_chat_ids"`
		ForwardProactive bool   `json:"forward_proactive"`
		Enabled          bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	st, err := s.telegram.Configure(r.Context(), body.Token, body.AllowedChatIDs, body.ForwardProactive, body.Enabled)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":    err.Error(),
			"telegram": st,
		})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) handleTelegramTest(w http.ResponseWriter, r *http.Request) {
	if s.telegram == nil {
		writeError(w, http.StatusServiceUnavailable, "telegram bridge not initialized")
		return
	}
	var body struct {
		ChatID  int64  `json:"chat_id"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	st := s.telegram.Status()
	chatID := body.ChatID
	if chatID == 0 {
		chatID = st.LastChatID
	}
	if chatID == 0 && strings.TrimSpace(st.AllowedChatIDs) != "" {
		first := strings.TrimSpace(strings.Split(st.AllowedChatIDs, ",")[0])
		chatID, _ = strconv.ParseInt(first, 10, 64)
	}
	if chatID == 0 {
		writeError(w, http.StatusBadRequest, "No Chat ID known yet. Send /start to your Telegram bot first or enter your Chat ID in Allowed Chat IDs.")
		return
	}
	if strings.TrimSpace(body.Message) == "" {
		body.Message = "⚡ ACE Glass-Box Cockpit test ping! Bidirectional Telegram bridge is live."
	}
	if err := s.telegram.SendMessage(r.Context(), chatID, body.Message); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.telegram.Status())
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan core.LiveEvent, 64)
	unsub := s.telem.SubscribeLive(func(evt core.LiveEvent) {
		select {
		case ch <- evt:
		default:
		}
	})
	defer unsub()

	ch <- core.LiveEvent{
		Type:      "engine.status",
		Timestamp: time.Now().UTC(),
		Payload:   s.engine.Status(),
	}

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-ch:
			data, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
			flusher.Flush()
		}
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func nonNilSlice[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
