package channels

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"ace/internal/cognitive"
	"ace/internal/core"
	"ace/internal/sandbox"
)

// TelegramStatus reports the live state of the Telegram Bot channel adapter.
type TelegramStatus struct {
	Enabled          bool      `json:"enabled"`
	Connected        bool      `json:"connected"`
	BotUsername      string    `json:"bot_username"`
	BotFirstName     string    `json:"bot_first_name"`
	MaskedToken      string    `json:"masked_token"`
	AllowedChatIDs   string    `json:"allowed_chat_ids"`
	LastChatID       int64     `json:"last_chat_id"`
	ForwardProactive bool      `json:"forward_proactive"`
	MessagesIn       int64     `json:"messages_in"`
	MessagesOut      int64     `json:"messages_out"`
	LastError        string    `json:"last_error,omitempty"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TelegramBridge implements an OpenClaw-style Channel Adapter for Telegram using long-polling (getUpdates).
type TelegramBridge struct {
	mu               sync.RWMutex
	db               *sql.DB
	engine           *cognitive.Engine
	mem              core.MemoryStore
	jail             *sandbox.Jail
	telem            core.TelemetrySink
	baseURL          string
	token            string
	allowedChatIDs   string
	lastChatID       int64
	forwardProactive bool
	enabled          bool
	connected        bool
	botUsername      string
	botFirstName     string
	messagesIn       int64
	messagesOut      int64
	lastError        string
	updatedAt        time.Time
	pollCancel       context.CancelFunc
	client           *http.Client
}

func NewTelegramBridge(
	db *sql.DB,
	engine *cognitive.Engine,
	mem core.MemoryStore,
	jail *sandbox.Jail,
	telem core.TelemetrySink,
	envToken string,
	envChatIDs string,
) (*TelegramBridge, error) {
	b := &TelegramBridge{
		db:               db,
		engine:           engine,
		mem:              mem,
		jail:             jail,
		telem:            telem,
		baseURL:          "https://api.telegram.org",
		forwardProactive: false, // Default false so background L6 tool steps never spam Telegram
		updatedAt:        time.Now().UTC(),
		client:           &http.Client{Timeout: 35 * time.Second},
	}
	if err := b.migrate(); err != nil {
		return nil, err
	}
	b.loadSettings(envToken, envChatIDs)
	return b, nil
}

func (b *TelegramBridge) migrate() error {
	_, err := b.db.Exec(`
		CREATE TABLE IF NOT EXISTS channel_settings (
			channel TEXT PRIMARY KEY,
			token TEXT NOT NULL,
			allowed_chat_ids TEXT NOT NULL,
			last_chat_id INTEGER NOT NULL DEFAULT 0,
			forward_proactive INTEGER NOT NULL DEFAULT 1,
			enabled INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME NOT NULL
		);
	`)
	return err
}

func (b *TelegramBridge) loadSettings(envToken string, envChatIDs string) {
	var tok, allowed string
	var lastChat int64
	var fwd, en int
	err := b.db.QueryRow(`
		SELECT token, allowed_chat_ids, last_chat_id, forward_proactive, enabled
		FROM channel_settings WHERE channel = 'telegram'
	`).Scan(&tok, &allowed, &lastChat, &fwd, &en)
	if err == nil {
		b.token = tok
		b.allowedChatIDs = allowed
		b.lastChatID = lastChat
		b.forwardProactive = fwd == 1
		b.enabled = en == 1
	}
	if strings.TrimSpace(envToken) != "" {
		b.token = strings.TrimSpace(envToken)
		b.enabled = true
	}
	if strings.TrimSpace(envChatIDs) != "" {
		b.allowedChatIDs = strings.TrimSpace(envChatIDs)
	}
}

func (b *TelegramBridge) saveSettingsLocked() {
	fwd := 0
	if b.forwardProactive {
		fwd = 1
	}
	en := 0
	if b.enabled {
		en = 1
	}
	_, _ = b.db.Exec(`
		INSERT INTO channel_settings (channel, token, allowed_chat_ids, last_chat_id, forward_proactive, enabled, updated_at)
		VALUES ('telegram', ?, ?, ?, ?, ?, ?)
		ON CONFLICT(channel) DO UPDATE SET
			token = excluded.token,
			allowed_chat_ids = excluded.allowed_chat_ids,
			last_chat_id = excluded.last_chat_id,
			forward_proactive = excluded.forward_proactive,
			enabled = excluded.enabled,
			updated_at = excluded.updated_at
	`, b.token, b.allowedChatIDs, b.lastChatID, fwd, en, time.Now().UTC())
}

func (b *TelegramBridge) Start(ctx context.Context) {
	// Turn off noisy proactive forwarding in DB if it was previously defaulted to 1
	b.mu.Lock()
	if b.forwardProactive {
		b.forwardProactive = false
		b.saveSettingsLocked()
	}
	b.mu.Unlock()

	// Subscribe to live events: ONLY forward explicit cycle completion summaries (Source == "L6_summary")
	// when forwardProactive is explicitly enabled by the user. Never forward raw L6 tool steps!
	b.telem.SubscribeLive(func(evt core.LiveEvent) {
		if evt.Type != "chat.message" {
			return
		}
		msg, ok := evt.Payload.(core.ChatMessage)
		if !ok || msg.Role != "proactive" || msg.Source != "L6_summary" {
			return
		}
		b.mu.RLock()
		shouldFwd := b.enabled && b.connected && b.forwardProactive && b.lastChatID != 0
		chatID := b.lastChatID
		b.mu.RUnlock()
		if shouldFwd {
			go func() {
				_ = b.SendMessage(context.Background(), chatID, fmt.Sprintf("✅ *ACE Cycle Complete*\n%s", msg.Content))
			}()
		}
	})

	b.mu.Lock()
	shouldStart := b.enabled && b.token != ""
	b.mu.Unlock()
	if shouldStart {
		b.restartPoller(ctx)
	}
}

// Configure updates the Telegram Bot token, allowed chat IDs, and proactive forwarding setting at runtime.
func (b *TelegramBridge) Configure(ctx context.Context, token string, allowedChatIDs string, forwardProactive bool, enabled bool) (TelegramStatus, error) {
	b.mu.Lock()
	if strings.TrimSpace(token) != "" && !strings.Contains(token, "***") {
		b.token = strings.TrimSpace(token)
	}
	b.allowedChatIDs = strings.TrimSpace(allowedChatIDs)
	b.forwardProactive = forwardProactive
	b.enabled = enabled && b.token != ""
	b.updatedAt = time.Now().UTC()
	b.saveSettingsLocked()
	tok := b.token
	en := b.enabled
	if b.pollCancel != nil {
		b.pollCancel()
		b.pollCancel = nil
	}
	b.connected = false
	b.lastError = ""
	b.mu.Unlock()

	if !en || tok == "" {
		st := b.Status()
		b.telem.EmitLive("telegram.status", st)
		return st, nil
	}

	// Verify bot token via getMe before starting poller
	username, firstName, err := b.getMe(ctx, tok)
	if err != nil {
		b.mu.Lock()
		b.lastError = err.Error()
		b.connected = false
		b.mu.Unlock()
		st := b.Status()
		b.telem.EmitLive("telegram.status", st)
		return st, err
	}

	b.mu.Lock()
	b.botUsername = username
	b.botFirstName = firstName
	b.connected = true
	b.lastError = ""
	b.mu.Unlock()

	b.restartPoller(context.Background())
	st := b.Status()
	b.telem.EmitLive("telegram.status", st)
	return st, nil
}

func (b *TelegramBridge) restartPoller(parentCtx context.Context) {
	b.mu.Lock()
	if b.pollCancel != nil {
		b.pollCancel()
	}
	pollCtx, cancel := context.WithCancel(parentCtx)
	b.pollCancel = cancel
	b.mu.Unlock()

	go b.pollLoop(pollCtx)
}

func (b *TelegramBridge) getMe(ctx context.Context, token string) (string, string, error) {
	url := fmt.Sprintf("%s/bot%s/getMe", b.baseURL, token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", err
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var res struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Result      struct {
			Username  string `json:"username"`
			FirstName string `json:"first_name"`
		} `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&res); err != nil {
		return "", "", err
	}
	if !res.OK {
		return "", "", fmt.Errorf("telegram getMe failed: %s", res.Description)
	}
	return res.Result.Username, res.Result.FirstName, nil
}

func (b *TelegramBridge) pollLoop(ctx context.Context) {
	b.mu.RLock()
	tok := b.token
	b.mu.RUnlock()

	if u, fn, err := b.getMe(ctx, tok); err == nil {
		b.mu.Lock()
		b.botUsername = u
		b.botFirstName = fn
		b.connected = true
		b.lastError = ""
		b.mu.Unlock()
		b.telem.EmitLive("telegram.status", b.Status())
	} else {
		b.mu.Lock()
		b.connected = false
		b.lastError = err.Error()
		b.mu.Unlock()
		b.telem.EmitLive("telegram.status", b.Status())
		return
	}

	var offset int64
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		updates, err := b.fetchUpdates(ctx, tok, offset)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			b.mu.Lock()
			b.lastError = err.Error()
			b.mu.Unlock()
			time.Sleep(3 * time.Second)
			continue
		}

		for _, u := range updates {
			if u.UpdateID >= offset {
				offset = u.UpdateID + 1
			}
			if u.Message == nil || strings.TrimSpace(u.Message.Text) == "" {
				continue
			}
			b.handleInboundMessage(ctx, u.Message.Chat.ID, u.Message.From.Username, u.Message.Text)
		}
	}
}

type tgUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		MessageID int64 `json:"message_id"`
		From      struct {
			Username  string `json:"username"`
			FirstName string `json:"first_name"`
		} `json:"from"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

func (b *TelegramBridge) fetchUpdates(ctx context.Context, token string, offset int64) ([]tgUpdate, error) {
	url := fmt.Sprintf("%s/bot%s/getUpdates?timeout=20&offset=%d", b.baseURL, token, offset)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload struct {
		OK          bool       `json:"ok"`
		Description string     `json:"description"`
		Result      []tgUpdate `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256*1024)).Decode(&payload); err != nil {
		return nil, err
	}
	if !payload.OK {
		return nil, errors.New(payload.Description)
	}
	return payload.Result, nil
}

func (b *TelegramBridge) isChatAllowed(chatID int64) bool {
	b.mu.RLock()
	allowed := strings.TrimSpace(b.allowedChatIDs)
	b.mu.RUnlock()
	if allowed == "" {
		return true
	}
	idStr := strconv.FormatInt(chatID, 10)
	for _, part := range strings.Split(allowed, ",") {
		if strings.TrimSpace(part) == idStr {
			return true
		}
	}
	return false
}

func (b *TelegramBridge) handleInboundMessage(ctx context.Context, chatID int64, username string, text string) {
	if !b.isChatAllowed(chatID) {
		_ = b.SendMessage(ctx, chatID, fmt.Sprintf("⛔ Unauthorized Chat ID `%d`. Add this ID to Allowed Chat IDs in the ACE Web Cockpit.", chatID))
		return
	}

	b.mu.Lock()
	b.messagesIn++
	if b.lastChatID != chatID {
		b.lastChatID = chatID
		b.saveSettingsLocked()
	}
	b.updatedAt = time.Now().UTC()
	b.mu.Unlock()
	b.telem.EmitLive("telegram.status", b.Status())

	text = strings.TrimSpace(text)

	// Handle Telegram Slash Commands (/start, /status, /goals, /cycle, /mode, /exec, /wiki, /tickets)
	if strings.HasPrefix(text, "/") {
		reply := b.executeSlashCommand(ctx, chatID, text)
		_ = b.SendMessage(ctx, chatID, reply)
		return
	}

	// Standard conversational message -> route through ACE Fast Chat + 6-layer cascade!
	msg, err := b.engine.Chat(ctx, text)
	if err != nil {
		_ = b.SendMessage(ctx, chatID, "⚠️ ACE error: "+err.Error())
		return
	}
	_ = b.SendMessage(ctx, chatID, msg.Content)
}

func (b *TelegramBridge) executeSlashCommand(ctx context.Context, chatID int64, rawCmd string) string {
	parts := strings.Fields(rawCmd)
	cmd := strings.ToLower(strings.Split(parts[0], "@")[0])
	arg := strings.TrimSpace(strings.TrimPrefix(rawCmd, parts[0]))

	switch cmd {
	case "/start", "/help":
		return fmt.Sprintf("⚡ *ACE Cognitive Cockpit Connected*\nYour Chat ID: `%d`\n\n*Commands:*\n• `/status` — 6-layer state, mode & sandbox health\n• `/goals` — View active mission in `goals.md`\n• `/cycle <directive>` — Trigger full L1→L6 cascade\n• `/stop` — Stop any running cognitive cycle\n• `/mode <reactive|autonomous|manual|paused>` — Switch mode\n• `/exec <sh cmd>` — Run command inside `bwrap` sandbox\n• `/tickets` — List Pi task tickets\n• `/wiki [path]` — List or read Wiki pages\n\nOr send any normal message to chat with ACE!", chatID)

	case "/stop":
		st := b.engine.StopCurrentCycle()
		return fmt.Sprintf("🛑 Stopped active cognitive cycle. Mode: `%s` | Busy: `%v`", st.Mode, st.Busy)

	case "/status":
		st := b.engine.Status()
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("🧠 *ACE Engine Status*\n• Mode: `%s` | Busy: `%v` | Cycles: `%d`\n• Model: `%s`\n• Sandbox: `Healthy=%v` (`%s`)\n\n*6-Layer Stack:*\n",
			st.Mode, st.Busy, st.TotalCycles, st.ActiveModel, st.Sandbox.Healthy, st.Sandbox.BwrapVersion))
		for _, l := range st.Layers {
			sb.WriteString(fmt.Sprintf("• *%s (%s)* [%s]: %s\n", l.Code, l.Name, l.State, l.Summary))
		}
		return sb.String()

	case "/goals":
		goals, err := b.mem.ReadWiki("goals.md")
		if err != nil {
			return "Could not read goals.md: " + err.Error()
		}
		return "🎯 *Active Mission (`goals.md`):*\n\n" + goals

	case "/cycle":
		if arg == "" {
			arg = "Telegram-triggered tactical cascade: advance the top uncompleted mission goal in goals.md."
		}
		cycID := b.engine.EnqueueCycle(arg, core.LayerExecutive, 3)
		return fmt.Sprintf("🚀 Enqueued L4→L6 tactical cascade `%s`:\n_%s_", cycID, arg)

	case "/mode":
		mode := strings.ToLower(strings.TrimSpace(arg))
		if mode == "" {
			return "Usage: `/mode reactive | autonomous | manual | paused`"
		}
		st := b.engine.SetMode(core.EngineMode(mode), 0)
		return fmt.Sprintf("✅ ACE mode set to `%s`.", st.Mode)

	case "/exec":
		if arg == "" {
			return "Usage: `/exec <sandbox shell command>`"
		}
		res, _ := b.jail.Exec(ctx, arg)
		out := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
		if len(out) > 2500 {
			out = out[:2500] + "..."
		}
		return fmt.Sprintf("🛡️ *Sandbox Exec* (`exit=%d`, `%dms`):\n```\n%s\n```", res.ExitCode, res.DurationMs, out)

	case "/tickets":
		res, _ := b.jail.Exec(ctx, "ticket list")
		return "🎫 *Pi Tickets (`/workspace/.tickets`):*\n```json\n" + strings.TrimSpace(res.Stdout) + "\n```"

	case "/wiki":
		if arg == "" {
			pages, _ := b.mem.ListWiki()
			var sb strings.Builder
			sb.WriteString("📚 *ACE LLM Wiki Pages:*\n")
			for _, p := range pages {
				sb.WriteString(fmt.Sprintf("• `%s` (%d bytes)\n", p.Path, p.SizeBytes))
			}
			sb.WriteString("\nRead a page with `/wiki <path>`")
			return sb.String()
		}
		content, err := b.mem.ReadWiki(arg)
		if err != nil {
			return "Wiki read error: " + err.Error()
		}
		if len(content) > 3000 {
			content = content[:3000] + "..."
		}
		return fmt.Sprintf("📄 *Wiki: `%s`*\n\n%s", arg, content)
	}

	return "Unknown command. Send `/help` for available commands."
}

// SendMessage sends a message to a Telegram chat ID using clean HTML formatting (converting Markdown **bold** and `code`).
func (b *TelegramBridge) SendMessage(ctx context.Context, chatID int64, text string) error {
	b.mu.RLock()
	tok := b.token
	b.mu.RUnlock()
	if tok == "" || chatID == 0 {
		return errors.New("telegram token or chat_id not configured")
	}
	if len(text) > 3800 {
		text = text[:3800] + "..."
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", b.baseURL, tok)
	htmlText := formatTelegramHTML(text)
	bodyBytes, _ := json.Marshal(map[string]interface{}{
		"chat_id":    chatID,
		"text":       htmlText,
		"parse_mode": "HTML",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Fallback to plain text if Telegram HTML parser ever rejects an entity
	if resp.StatusCode >= 400 {
		fallbackBytes, _ := json.Marshal(map[string]interface{}{
			"chat_id": chatID,
			"text":    strings.ReplaceAll(text, "**", ""),
		})
		if req2, err2 := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(fallbackBytes)); err2 == nil {
			req2.Header.Set("Content-Type", "application/json")
			if resp2, err3 := b.client.Do(req2); err3 == nil {
				_ = resp2.Body.Close()
			}
		}
	}

	b.mu.Lock()
	b.messagesOut++
	b.updatedAt = time.Now().UTC()
	b.mu.Unlock()
	b.telem.EmitLive("telegram.status", b.Status())
	return nil
}

var (
	tgCodeBlockRe  = regexp.MustCompile("(?s)```(?:[a-zA-Z0-9_-]+)?\\n?(.*?)```")
	tgInlineCodeRe = regexp.MustCompile("`([^`\n]+)`")
	tgBoldDoubleRe = regexp.MustCompile(`\*\*([^\*\n]+)\*\*`)
	tgBoldSingleRe = regexp.MustCompile(`(?:^|[\s(])\*([^\*\n]+)\*`)
	tgHeaderRe     = regexp.MustCompile(`(?m)^#{1,4}\s+(.+)$`)
)

func formatTelegramHTML(md string) string {
	escaped := html.EscapeString(md)
	var blocks []string
	escaped = tgCodeBlockRe.ReplaceAllStringFunc(escaped, func(m string) string {
		sub := tgCodeBlockRe.FindStringSubmatch(m)
		idx := len(blocks)
		blocks = append(blocks, "<pre>"+strings.TrimSpace(sub[1])+"</pre>")
		return fmt.Sprintf("\x00BLK%d\x00", idx)
	})
	escaped = tgInlineCodeRe.ReplaceAllString(escaped, "<code>$1</code>")
	escaped = tgHeaderRe.ReplaceAllString(escaped, "<b>$1</b>")
	escaped = tgBoldDoubleRe.ReplaceAllString(escaped, "<b>$1</b>")
	escaped = tgBoldSingleRe.ReplaceAllStringFunc(escaped, func(m string) string {
		idx := strings.Index(m, "*")
		if idx < 0 {
			return m
		}
		prefix := m[:idx]
		inner := strings.TrimSuffix(m[idx+1:], "*")
		return prefix + "<b>" + inner + "</b>"
	})
	for i, blk := range blocks {
		escaped = strings.ReplaceAll(escaped, fmt.Sprintf("\x00BLK%d\x00", i), blk)
	}
	return escaped
}

func (b *TelegramBridge) Status() TelegramStatus {
	b.mu.RLock()
	defer b.mu.RUnlock()
	masked := ""
	if len(b.token) > 10 {
		masked = b.token[:6] + "***" + b.token[len(b.token)-4:]
	} else if b.token != "" {
		masked = "***"
	}
	return TelegramStatus{
		Enabled:          b.enabled,
		Connected:        b.connected,
		BotUsername:      b.botUsername,
		BotFirstName:     b.botFirstName,
		MaskedToken:      masked,
		AllowedChatIDs:   b.allowedChatIDs,
		LastChatID:       b.lastChatID,
		ForwardProactive: b.forwardProactive,
		MessagesIn:       b.messagesIn,
		MessagesOut:      b.messagesOut,
		LastError:        b.lastError,
		UpdatedAt:        b.updatedAt,
	}
}
