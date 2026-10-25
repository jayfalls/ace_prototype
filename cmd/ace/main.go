package main

import (
	"bufio"
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"ace"
	"ace/internal/bus"
	"ace/internal/channels"
	"ace/internal/cognitive"
	"ace/internal/config"
	"ace/internal/constitution"
	"ace/internal/llm"
	"ace/internal/memory"
	"ace/internal/sandbox"
	"ace/internal/server"
	"ace/internal/telemetry"
	"ace/internal/watchdog"
	_ "modernc.org/sqlite"
)

func main() {
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		runSubcommand(os.Args[1:])
		return
	}

	if err := constitution.Check(); err != nil {
		log.Fatalf("constitution check failed: %v", err)
	}

	defaultPort := envOr("ACE_PORT", "8080")
	defaultSandbox := envOr("ACE_SANDBOX_DIR", ".ace-sandbox")
	defaultModel := envOr("ACE_MODEL", "deepseek-v4.1-flash")
	defaultTgToken := envOr("ACE_TELEGRAM_TOKEN", "")
	defaultTgChats := envOr("ACE_TELEGRAM_CHAT_IDS", "")

	port := flag.String("port", defaultPort, "HTTP server port")
	sandboxDir := flag.String("sandbox-dir", defaultSandbox, "Isolated sandbox directory (os.Root + bwrap)")
	modelID := flag.String("model", defaultModel, "Default OpenCode Free model ID")
	tgToken := flag.String("telegram-token", defaultTgToken, "Optional Telegram Bot token from @BotFather")
	tgChats := flag.String("telegram-chat-ids", defaultTgChats, "Optional comma-separated allowed Telegram Chat IDs")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	jail, err := sandbox.NewJail(*sandboxDir)
	if err != nil {
		log.Fatalf("failed to initialize sandbox jail: %v", err)
	}
	defer jail.Close()

	sbStatus := jail.Status()
	log.Printf("[SANDBOX] Healthy=%v | %s", sbStatus.Healthy, sbStatus.VerificationLog)

	ensureLocalGPUGate(ctx, *sandboxDir)

	dbPath := filepath.Join(*sandboxDir, "ace.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatalf("failed to open sqlite db: %v", err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()

	telem, err := telemetry.NewSink(db)
	if err != nil {
		log.Fatalf("failed to initialize telemetry sink: %v", err)
	}

	memBus, err := bus.NewMemoryBus(db, telem)
	if err != nil {
		log.Fatalf("failed to initialize cognitive bus: %v", err)
	}

	memStore, err := memory.NewStore(db, jail.WikiRoot(), jail.WikiDir(), telem)
	if err != nil {
		log.Fatalf("failed to initialize memory & wiki store: %v", err)
	}

	provider := llm.NewOpenCodeFreeProvider("")
	if err := provider.SetActiveModel(*modelID); err != nil {
		log.Printf("[LLM] warning: %v, using %s", err, provider.ActiveModel())
	}

	engine := cognitive.NewEngine(provider, memBus, memStore, jail, telem)
	engine.Start(ctx)

	tgBridge, err := channels.NewTelegramBridge(db, engine, memStore, jail, telem, *tgToken, *tgChats)
	if err != nil {
		log.Fatalf("failed to initialize telegram bridge: %v", err)
	}
	tgBridge.Start(ctx)

	webFS, err := ace.WebFS()
	if err != nil {
		log.Fatalf("failed to load embedded web UI: %v", err)
	}

	authToken, err := config.LoadOrCreateToken()
	if err != nil {
		log.Fatalf("failed to load bearer token: %v", err)
	}

	srv := server.NewServer(engine, provider, memBus, memStore, jail, telem, tgBridge, webFS, authToken)
	httpServer := &http.Server{
		Addr:              "127.0.0.1:" + *port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		fmt.Printf("\n================================================================\n")
		fmt.Printf("  ACE Glass-Box Cognitive Cockpit: http://localhost:%s\n", *port)
		fmt.Printf("  Active OpenCode Free Model:      %s\n", provider.ActiveModel())
		fmt.Printf("  Bearer Token (mutating /api):    %s\n", filepath.Join(config.Dir(), "bearer_token"))
		fmt.Printf("  Host-Proof Sandbox Root:         %s (Healthy=%v)\n", sbStatus.SandboxDir, sbStatus.Healthy)
		fmt.Printf("================================================================\n\n")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// runSubcommand handles `ace init` and `ace mission reload` — the owner-facing
// commands that write owner-set config (plan 8.1). The daemon never runs here.
func runSubcommand(args []string) {
	switch args[0] {
	case "init":
		if err := runInit(); err != nil {
			log.Fatalf("ace init: %v", err)
		}
	case "watchdog":
		runWatchdog(args[1:])
	case "mission":
		if len(args) > 1 && args[1] == "reload" {
			runMissionReload()
			return
		}
		log.Fatalf("usage: ace mission reload")
	default:
		log.Fatalf("unknown command %q — usage: ace [init|mission reload] or ace (daemon)", args[0])
	}
}

// runInit asks the owner for the life mission, optional personality (≤1KB) and
// capability grants, then writes the owner-set config files + bearer token.
func runInit() error {
	if m := config.LoadMission(); m != "" {
		fmt.Printf("Mission already set (version %s): %s\nEdit %s/mission.md or `ace mission reload` to re-steer.\n", config.MissionVersion(), m, config.Dir())
		return nil
	}
	scan := bufio.NewScanner(os.Stdin)
	ask := func(label string) string {
		fmt.Print(label)
		if !scan.Scan() {
			return ""
		}
		return strings.TrimSpace(scan.Text())
	}

	mission := ask("Life mission: ")
	if mission == "" {
		return fmt.Errorf("a life mission is required — ACE idles until one is anchored")
	}
	personality := ask("Personality (optional, ≤1000 bytes): ")
	if len(personality) > 1000 {
		personality = personality[:1000]
	}
	grantIn := ask("Capability grants (comma-separated, empty for none): ")
	var grants []string
	for _, g := range strings.Split(grantIn, ",") {
		if g = strings.TrimSpace(g); g != "" {
			grants = append(grants, g)
		}
	}
	if err := config.WriteInit(mission, personality, grants); err != nil {
		return err
	}
	if _, err := config.LoadOrCreateToken(); err != nil {
		return err
	}
	fmt.Printf("Initialized: mission v%s at %s — run `ace` to start.\n", config.MissionVersion(), config.Dir())
	return nil
}

// runMissionReload re-anchors the mission: the engine reads mission.md every
// cycle and derives a new mission version on any content change, marks every
// open plan node stale, and re-runs L1 genesis against the constitution.
func runMissionReload() {
	m := config.LoadMission()
	if m == "" {
		fmt.Printf("No mission set — run `ace init` first.\n")
		return
	}
	fmt.Printf("Mission v%s re-anchoring: %s\nThe running engine picks it up on its next cycle.\n", config.MissionVersion(), m)
}

func discoverGPUPython() string {
	if envPy := strings.TrimSpace(os.Getenv("ACE_GPU_PYTHON")); envPy != "" {
		if _, err := os.Stat(envPy); err == nil {
			return envPy
		}
	}
	if venv := strings.TrimSpace(os.Getenv("VIRTUAL_ENV")); venv != "" {
		cand := filepath.Join(venv, "bin", "python")
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		matches, _ := filepath.Glob(filepath.Join(home, "*", "*", "*", ".venv", "bin", "python"))
		for _, m := range matches {
			if out, err := exec.Command(m, "-c", "import torch").CombinedOutput(); err == nil && len(out) == 0 {
				return m
			}
		}
	}
	return "python3"
}

// ensureLocalGPUGate ensures the local 4-bit System-1 GPU Decision Server (scripts/gpu_gate_server.py)
// is running on 127.0.0.1:8091 as a managed child process of ./ace (and auto-restarts it if it exits).
func ensureLocalGPUGate(ctx context.Context, sandboxDir string) {
	go func() {
		client := &http.Client{Timeout: 1500 * time.Millisecond}
		pyBin := discoverGPUPython()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			resp, err := client.Get("http://127.0.0.1:8091/health")
			if err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					time.Sleep(10 * time.Second)
					continue
				}
			}

			scriptPath := "scripts/gpu_gate_server.py"
			if _, statErr := os.Stat(scriptPath); statErr != nil {
				time.Sleep(15 * time.Second)
				continue
			}

			logFile, fErr := os.OpenFile(filepath.Join(sandboxDir, "gpu-gate.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			cmd := exec.CommandContext(ctx, pyBin, scriptPath)
			cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1", "ACE_GPU_PORT=8091")
			if fErr == nil {
				cmd.Stdout = logFile
				cmd.Stderr = logFile
			}
			if startErr := cmd.Start(); startErr == nil {
				log.Printf("[GPU-GATE] Spawned managed local 4-bit GPU gate server (PID %d)", cmd.Process.Pid)
				_ = cmd.Wait()
			}
			if fErr == nil {
				_ = logFile.Close()
			}
			time.Sleep(3 * time.Second)
		}
	}()
}

// runWatchdog implements `ace watchdog` (spec 8.5): a separate process that
// reads the atomic status snapshot — never the bus — checks integrity
// invariants, appends hash-chained events, and can pause/halt the engine via
// the loopback API.
func runWatchdog(args []string) {
	sandboxDir := envOr("ACE_SANDBOX_DIR", ".ace-sandbox")
	port := envOr("ACE_PORT", "8080")
	sub := "run"
	if len(args) > 0 {
		sub = args[0]
	}
	dbPath := filepath.Join(sandboxDir, "ace.db")

	switch sub {
	case "verify":
		db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(2000)")
		if err != nil {
			log.Fatalf("watchdog verify: %v", err)
		}
		defer db.Close()
		watchdog.InitEvents(db)
		if err := watchdog.VerifyChain(db); err != nil {
			log.Fatalf("watchdog chain broken: %v", err)
		}
		fmt.Println("watchdog event chain verified")
		return
	case "halt", "pause", "resume":
		token, _ := os.ReadFile(filepath.Join(config.Dir(), "bearer_token"))
		mode := "paused"
		if sub == "resume" {
			mode = "autonomous"
		}
		url := fmt.Sprintf("http://127.0.0.1:%s/api/engine/mode", port)
		body := strings.NewReader(fmt.Sprintf(`{"mode":%q}`, mode))
		req, _ := http.NewRequest(http.MethodPost, url, body)
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 5 * time.Second}
		if resp, err := client.Do(req); err == nil {
			resp.Body.Close()
		}
		if sub == "halt" {
			// halt <=5s: cancel in-flight work too (stop endpoint), then pause.
			req2, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%s/api/engine/stop", port), nil)
			req2.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
			if resp2, err := client.Do(req2); err == nil {
				resp2.Body.Close()
			}
		}
		fmt.Printf("watchdog %s sent (mode=%s)\n", sub, mode)
		return
	case "once", "run":
		// fall through to loop; "once" exits after one pass
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(2000)")
	if err != nil {
		log.Fatalf("watchdog: %v", err)
	}
	defer db.Close()
	watchdog.InitEvents(db)

	lastBackup := time.Time{}
	for {
		checks := watchdog.RunChecks(sandboxDir, 20)
		allOK := true
		for _, c := range checks {
			if !c.OK {
				allOK = false
				fmt.Printf("[watchdog] FAULT %s: %s\n", c.Name, c.Detail)
				_ = watchdog.AppendEvent(db, "fault", c.Name+": "+c.Detail)
			}
		}
		if allOK {
			fmt.Printf("[watchdog] %s all checks ok\n", time.Now().Format("15:04:05"))
		}
		if time.Since(lastBackup) > time.Hour {
			if dst, err := watchdog.Backup(context.Background(), sandboxDir); err == nil {
				_ = watchdog.AppendEvent(db, "backup", dst)
				fmt.Printf("[watchdog] backup -> %s\n", dst)
			}
			lastBackup = time.Now()
		}
		if sub == "once" {
			return
		}
		time.Sleep(30 * time.Second)
	}
}
