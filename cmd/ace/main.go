package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"ace"
	"ace/internal/bus"
	"ace/internal/channels"
	"ace/internal/cognitive"
	"ace/internal/config"
	"ace/internal/constitution"
	"ace/internal/core"
	"ace/internal/memory"
	"ace/internal/provider"
	"ace/internal/sandbox"
	"ace/internal/science"
	"ace/internal/server"
	"ace/internal/telemetry"
	"ace/internal/watchdog"
	_ "modernc.org/sqlite"
)

func main() {
	// Credentials before anything (v0.9 §9.0): the supervisor gets them and
	// the worker inherits via os.Environ.
	if err := config.LoadEnv(); err != nil {
		log.Fatal(err)
	}
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		runSubcommand(os.Args[1:])
		return
	}

	// Bare ./ace is the supervisor (plan § Watchdog): it makes no model calls,
	// spawns the agent as a child of the same binary (ACE_WORKER=1, Pdeathsig
	// set), restarts it on crash or snapshot-detected stall, and runs halt,
	// rollback and hourly VACUUM INTO backups. The worker is the agent.
	if os.Getenv("ACE_WORKER") != "1" {
		runSupervisor()
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

	provCfg, err := provider.Load(config.Dir())
	if err != nil {
		provCfg = provider.DefaultConfig()
	}
	if *modelID != "" {
		provCfg.Models.Main = *modelID // env/flag overrides provider.json
	}
	adapters, err := provider.BuildAdapters(ctx, provCfg, provCfg.Timeouts.Cloud, provCfg.Timeouts.Local)
	if err != nil {
		log.Fatalf("no LLM provider groups available: %v", err)
	}
	registry, err := provider.New(db, provCfg, adapters)
	if err != nil {
		log.Fatalf("failed to initialize provider registry: %v", err)
	}
	registry.OnChurn(func(ev provider.ChurnEvent) {
		_ = memBus.Publish(ctx, core.BusEnvelope{
			CycleID:   "system",
			AgentID:   "ace-primary",
			Topic:     bus.InputTopic("ace-primary", 5),
			Direction: core.DirectionNorthbound,
			FromLayer: 6, ToLayer: 5,
			Kind:    "telemetry",
			Summary: fmt.Sprintf("catalogue churn: +%d -%d ~%d", len(ev.Added), len(ev.Removed), len(ev.Changed)),
			Payload: provider.MarshalChurn(ev),
		})
	})
	var llmPort core.LLMProvider = registry

	engine := cognitive.NewEngine(llmPort, memBus, memStore, jail, telem)
	// Budget governor (spec 9.4): smallest configured 5h-window quota slows
	// non-L6 cadences when projected burn exceeds the remaining window.
	var quotaUnits float64
	for _, windows := range provCfg.Quota {
		if lim, ok := windows["5h"]; ok && lim > 0 && (quotaUnits == 0 || lim < quotaUnits) {
			quotaUnits = lim
		}
	}
	engine.SetBudgetQuota(quotaUnits, int(provider.QuotaWindowSecs("5h")))
	engine.Start(ctx)

	tgBridge, err := channels.NewTelegramBridge(db, engine, memStore, jail, telem, *tgToken, *tgChats)
	if err != nil {
		log.Fatalf("failed to initialize telegram bridge: %v", err)
	}
	_ = tgBridge.Start(ctx, memBus)

	webFS, err := ace.WebFS()
	if err != nil {
		log.Fatalf("failed to load embedded web UI: %v", err)
	}

	authToken, err := config.LoadOrCreateToken()
	if err != nil {
		log.Fatalf("failed to load bearer token: %v", err)
	}

	srv := server.NewServer(engine, llmPort, memBus, memStore, jail, telem, tgBridge, webFS, authToken)
	httpServer := &http.Server{
		Addr:              "127.0.0.1:" + *port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		fmt.Printf("\n================================================================\n")
		fmt.Printf("  ACE Glass-Box Cognitive Cockpit: http://localhost:%s\n", *port)
		fmt.Printf("  Active Model (main role):        %s\n", registry.ActiveModel())
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
	case "verify":
		runVerify()
	case "halt", "pause", "resume", "rollback":
		runControl(args[0])
	case "science":
		if len(args) > 1 && args[1] == "regrade" {
			runScienceRegrade(args[2:])
			return
		}
		if len(args) > 1 && args[1] == "baseline" {
			runScienceBaseline(args[2:])
			return
		}
		log.Fatalf("usage: ace science regrade [archive-dir] | ace science baseline freeze --name NAME | ace science baseline compare NAME")
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

// runSupervisor is the watchdog overlay (commit 29cd3922, plan § Watchdog):
// bare ./ace makes NO model calls — it spawns the agent as a child of the
// same binary (ACE_WORKER=1, Pdeathsig so a dead parent kills the child),
// restarts it on crash or snapshot-detected stall, and runs the hash-chained
// event log plus hourly VACUUM INTO backups itself.
func runSupervisor() {
	sandboxDir := envOr("ACE_SANDBOX_DIR", ".ace-sandbox")
	events := watchdog.EventPath(sandboxDir)
	// One-time export of the retired in-DB chain; the supervisor holds no
	// persistent db handle so a rollback never strands it on a dead inode.
	if db, err := sql.Open("sqlite", filepath.Join(sandboxDir, "ace.db")+"?_pragma=busy_timeout(2000)"); err == nil {
		watchdog.MigrateEvents(db, events)
		_ = db.Close()
	}
	rollbackReq := filepath.Join(sandboxDir, "rollback.request")

	exe, err := os.Executable()
	if err != nil {
		log.Fatalf("supervisor: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	lastBackup := time.Now().Add(-time.Hour) // back up as soon as the worker is up
	restarts := 0
	// Runaway-episode dedup lives outside the child loop: a restarted
	// worker inherits the same rolling cycle count, so re-arming here
	// would re-pause every respawn. Re-armed by runawayClean below.
	runawayPaused := false
	runawayClean := 0
	for {
		child := exec.Command(exe, os.Args[1:]...)
		child.Env = append(os.Environ(), "ACE_WORKER=1")
		child.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}
		child.Stdout, child.Stderr, child.Stdin = os.Stdout, os.Stderr, os.Stdin
		if err := child.Start(); err != nil {
			_ = watchdog.AppendEventFile(events, "worker_start_fail", err.Error())
			time.Sleep(5 * time.Second)
			continue
		}
		_ = watchdog.AppendEventFile(events, "worker_start", fmt.Sprintf("pid=%d restart=%d", child.Process.Pid, restarts))
		log.Printf("[supervisor] worker pid %d (restart %d)", child.Process.Pid, restarts)
		// A fresh worker's first heartbeat lands ~15s out; re-stamp the
		// snapshot heartbeat at spawn so the stale-heartbeat check can't
		// kill it for its predecessor's silence.
		if prev, err := os.ReadFile(watchdog.SnapshotPath(sandboxDir)); err == nil {
			var s watchdog.Snapshot
			if json.Unmarshal(prev, &s) == nil {
				s.Heartbeat = time.Now().UTC().Format(time.RFC3339)
				_ = watchdog.WriteSnapshot(sandboxDir, s)
			}
		}

		childStarted := time.Now()
		done := make(chan error, 1)
		go func() { done <- child.Wait() }()

		exited := false
		for !exited {
			select {
			case err := <-done:
				_ = watchdog.AppendEventFile(events, "worker_exit", fmt.Sprintf("pid=%d err=%v", child.Process.Pid, err))
				exited = true
			case <-ctx.Done():
				_ = child.Process.Kill() // Pdeathsig covers it too; be explicit
				<-done
				_ = watchdog.AppendEventFile(events, "supervisor_stop", "signal received")
				return
			case <-time.After(15 * time.Second):
				if _, err := os.Stat(rollbackReq); err == nil {
					_ = watchdog.AppendEventFile(events, "rollback", "request received — stopping worker")
					log.Printf("[supervisor] rollback requested — stopping worker pid %d", child.Process.Pid)
					_ = child.Process.Kill()
					<-done
					doSupervisorRollback(sandboxDir, events)
					_ = os.Remove(rollbackReq)
					exited = true
					continue
				}
				stalled := false
				runaway := false
				for _, c := range watchdog.RunChecks(sandboxDir, 20) {
					if !c.OK {
						_ = watchdog.AppendEventFile(events, "fault", c.Name+": "+c.Detail)
						if c.Name == "heartbeat" || c.Name == "stall_detection" {
							// Boot grace: hydration and engine startup can
							// outlast the heartbeat window on big state.
							if time.Since(childStarted) < 2*time.Minute {
								continue
							}
							stalled = true
						}
						if c.Name == "runaway_loop" {
							runaway = true
						}
					}
				}
				if runaway {
					runawayClean = 0
					// Act, not just log: pause the worker once per episode and
					// let the engine send the single Telegram report.
					if !runawayPaused {
						runawayPaused = true
						_ = watchdog.AppendEventFile(events, "runaway_pause", "cognitive-cycle bound exceeded — pausing worker")
						if perr := postEngineMode(envOr("ACE_PORT", "8080"), "paused", "watchdog_runaway"); perr != nil {
							_ = watchdog.AppendEventFile(events, "runaway_pause_fail", perr.Error())
							runawayPaused = false // pause never landed — don't dedup it away
						}
					}
				} else {
					// Re-arm only after a sustained clean run — a single
					// transient zero (DB busy during a worker restart) must
					// not end the episode and re-fire the pause.
					runawayClean++
					if runawayClean >= 4 {
						runawayPaused = false
					}
				}
				if time.Since(lastBackup) > time.Hour {
					if dst, berr := watchdog.Backup(context.Background(), sandboxDir); berr == nil {
						_ = watchdog.AppendEventFile(events, "backup", dst)
						log.Printf("[supervisor] backup -> %s", dst)
					}
					lastBackup = time.Now()
				}
				if stalled {
					_ = watchdog.AppendEventFile(events, "stall_recovery", fmt.Sprintf("killing pid %d", child.Process.Pid))
					log.Printf("[supervisor] stall detected — restarting worker pid %d", child.Process.Pid)
					_ = child.Process.Kill()
					<-done
					exited = true
				}
			}
		}
		restarts++
		time.Sleep(3 * time.Second) // brief backoff before respawn
	}
}

// doSupervisorRollback runs while the worker is dead: safety-backup the
// current db, restore the newest backup taken BEFORE that safety copy, log a
// chained event, and clear the stale status snapshot (the new worker writes
// its own — a pre-rollback heartbeat would otherwise read as a stall).
func doSupervisorRollback(sandboxDir, events string) {
	dbPath := filepath.Join(sandboxDir, "ace.db")
	pat := filepath.Join(sandboxDir, "backups", "ace-*.db")
	pre, _ := filepath.Glob(pat)
	sort.Strings(pre)
	if len(pre) == 0 {
		_ = watchdog.AppendEventFile(events, "rollback", "aborted: no backups")
		writeRollbackDone(sandboxDir, "aborted: no backups")
		return
	}
	src := pre[len(pre)-1]
	if dst, err := watchdog.Backup(context.Background(), sandboxDir); err == nil {
		_ = watchdog.AppendEventFile(events, "rollback_pre_backup", dst)
	}
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	in, err := os.ReadFile(src)
	if err != nil {
		_ = watchdog.AppendEventFile(events, "rollback", "read failed: "+err.Error())
		writeRollbackDone(sandboxDir, "failed: "+err.Error())
		return
	}
	if err := os.WriteFile(dbPath, in, 0o600); err != nil {
		_ = watchdog.AppendEventFile(events, "rollback", "write failed: "+err.Error())
		writeRollbackDone(sandboxDir, "failed: "+err.Error())
		return
	}
	_ = os.Remove(watchdog.SnapshotPath(sandboxDir))
	_ = watchdog.AppendEventFile(events, "rollback", "restored "+src)
	writeRollbackDone(sandboxDir, "restored "+src)
	log.Printf("[supervisor] rollback restored %s", src)
}

func writeRollbackDone(sandboxDir, msg string) {
	_ = os.WriteFile(filepath.Join(sandboxDir, "rollback.done"),
		[]byte(time.Now().UTC().Format(time.RFC3339)+" "+msg), 0o600)
}

// runVerify implements `ace verify` — checks the watchdog hash-chained event
// file (outside ace.db so a rollback cannot erase it).
func runVerify() {
	events := watchdog.EventPath(envOr("ACE_SANDBOX_DIR", ".ace-sandbox"))
	if err := watchdog.VerifyChainFile(events); err != nil {
		log.Fatalf("watchdog chain broken: %v", err)
	}
	fmt.Println("watchdog event chain verified")
}

// runControl implements `ace halt|pause|resume` (loopback API to the worker,
// logged as chained watchdog events) and `ace rollback` (a request file the
// supervisor executes: stop worker → safety backup → restore → restart).
func runControl(sub string) {
	sandboxDir := envOr("ACE_SANDBOX_DIR", ".ace-sandbox")
	port := envOr("ACE_PORT", "8080")
	events := watchdog.EventPath(sandboxDir)

	if sub == "rollback" {
		req := filepath.Join(sandboxDir, "rollback.request")
		done := filepath.Join(sandboxDir, "rollback.done")
		_ = os.Remove(done)
		if err := os.WriteFile(req, []byte(time.Now().UTC().Format(time.RFC3339)), 0o600); err != nil {
			log.Fatalf("rollback request: %v", err)
		}
		_ = watchdog.AppendEventFile(events, "rollback_requested", "owner request via ace rollback")
		deadline := time.Now().Add(90 * time.Second)
		for time.Now().Before(deadline) {
			if b, err := os.ReadFile(done); err == nil {
				fmt.Printf("rollback: %s\n", strings.TrimSpace(string(b)))
				return
			}
			time.Sleep(time.Second)
		}
		log.Fatal("rollback: no supervisor response in 90s — is ./ace running?")
	}

	mode := "paused"
	if sub == "resume" {
		mode = "autonomous"
	}
	if err := postEngineMode(port, mode, "owner"); err != nil {
		fmt.Fprintf(os.Stderr, "%s failed: %v\n", sub, err)
		os.Exit(1)
	}
	if sub == "halt" {
		// halt <=5s: cancel in-flight work too (stop endpoint), then pause.
		token, _ := os.ReadFile(filepath.Join(config.Dir(), "bearer_token"))
		req2, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://127.0.0.1:%s/api/engine/stop", port), nil)
		req2.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
		client := &http.Client{Timeout: 5 * time.Second}
		if resp2, err := client.Do(req2); err == nil {
			resp2.Body.Close()
		}
	}
	_ = watchdog.AppendEventFile(events, "control", sub)
	fmt.Printf("%s sent (mode=%s)\n", sub, mode)
}

// postEngineMode sets the worker's engine mode over the loopback API —
// used by `ace halt|pause|resume` and by the supervisor's runaway
// watchdog when it pauses a burning loop. reason tags the request so
// watchdog-originated pauses can emit their one Telegram report.
func postEngineMode(port, mode, reason string) error {
	token, _ := os.ReadFile(filepath.Join(config.Dir(), "bearer_token"))
	url := fmt.Sprintf("http://127.0.0.1:%s/api/engine/mode", port)
	body := strings.NewReader(fmt.Sprintf(`{"mode":%q,"reason":%q}`, mode, reason))
	req, _ := http.NewRequest(http.MethodPost, url, body)
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("POST %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
		return fmt.Errorf("POST %s: %s: %s", url, resp.Status, strings.TrimSpace(string(msg)))
	}
	return nil
}

// runScienceRegrade re-grades archived v0.7 result JSONs through the 8.6
// scientist gate (spec: report what it finds, never pre-suppose). Declared
// booleans (h1_accepted) are ignored; statistics are recomputed from counts.
func runScienceRegrade(args []string) {
	dir := "archive"
	if len(args) > 0 {
		dir = args[0]
	}
	var files []string
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err == nil && strings.HasSuffix(p, ".json") && strings.Contains(p, "results") {
			files = append(files, p)
		}
		return nil
	})
	if len(files) == 0 {
		log.Fatalf("no result jsons under %s", dir)
	}
	var b strings.Builder
	b.WriteString("# v0.7 result re-grade through the 8.6 scientist gate\n\n")
	b.WriteString("| file | declared | recomputed mcnemar_p | declared_p | discrepancy |\n|---|---|---|---|---|\n")
	var recomputed, mismatched, undeclared int
	// Every regrade also writes a science_verdicts row (8.6) when the sandbox
	// db is reachable — the verdict table is the audit trail, the markdown is
	// only the report.
	var db *sql.DB
	if d, derr := sql.Open("sqlite", filepath.Join(envOr("ACE_SANDBOX_DIR", ".ace-sandbox"), "ace.db")+"?_pragma=busy_timeout(2000)"); derr == nil {
		db = d
		defer db.Close()
		_ = science.Migrate(db)
	}
	ctx := context.Background()
	for _, f := range files {
		blob, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if db != nil {
			if _, _, rgErr := science.RegradeResultJSON(ctx, db, blob, filepath.Base(f)); rgErr != nil {
				fmt.Fprintf(os.Stderr, "regrade verdict insert failed for %s: %v\n", filepath.Base(f), rgErr)
			}
		}
		var r struct {
			AxisID      string   `json:"axis"`
			Decision    string   `json:"decision"`
			H1Accepted  *bool    `json:"h1_accepted"`
			McP         *float64 `json:"mcnemar_p"`
			McFixed     *float64 `json:"mcnemar_fixed"`
			McBroken    *float64 `json:"mcnemar_broken"`
			Alpha       *float64 `json:"alpha_level"`
			CISeparated *bool    `json:"ci_separated"`
		}
		if json.Unmarshal(blob, &r) != nil {
			continue
		}
		declared := "?"
		if r.H1Accepted != nil {
			declared = fmt.Sprintf("h1=%v", *r.H1Accepted)
		}
		var flags []string
		var recStr, declStr = "n/a", "n/a"
		if r.McFixed != nil && r.McBroken != nil {
			p := science.McNemarExact(int(*r.McFixed), int(*r.McBroken))
			recStr = fmt.Sprintf("%.5f", p)
			recomputed++
			if r.McP != nil {
				declStr = fmt.Sprintf("%.5f", *r.McP)
				if math.Abs(*r.McP-p) > 0.01 {
					flags = append(flags, "p-value mismatch")
					mismatched++
				}
			}
			if r.Alpha != nil && r.H1Accepted != nil && *r.H1Accepted && p > *r.Alpha {
				flags = append(flags, "declared ACCEPT but recomputed p>alpha")
				mismatched++
			}
		} else {
			undeclared++
			flags = append(flags, "no recomputable counts — PROVISIONAL at best")
		}
		name := filepath.Base(f)
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n", name, declared, recStr, declStr, strings.Join(flags, "; ")))
	}
	b.WriteString(fmt.Sprintf("\n%d files: %d with recomputable McNemar counts, %d discrepancies vs declared, %d without recomputable stats.\n",
		len(files), recomputed, mismatched, undeclared))
	out := filepath.Join(dir, "v07_regrade.md")
	if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil {
		log.Fatalf("write report: %v", err)
	}
	fmt.Printf("regraded %d files -> %s\n", len(files), out)
}

// runScienceBaseline implements `ace science baseline freeze --name NAME` and
// `ace science baseline compare NAME` (spec 9.0): freeze snapshots the era's
// standing metrics under the current definition hash; compare prints deltas
// and refuses when the live definition differs from the frozen one.
func runScienceBaseline(args []string) {
	if len(args) == 0 {
		log.Fatalf("usage: ace science baseline freeze --name NAME | compare NAME")
	}
	db, err := sql.Open("sqlite", filepath.Join(envOr("ACE_SANDBOX_DIR", ".ace-sandbox"), "ace.db")+"?_pragma=busy_timeout(2000)")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()
	_ = science.Migrate(db)
	ctx := context.Background()
	switch args[0] {
	case "freeze":
		name := ""
		for i := 1; i+1 < len(args); i++ {
			if args[i] == "--name" {
				name = args[i+1]
			}
		}
		if name == "" {
			log.Fatal("baseline freeze needs --name")
		}
		b, err := science.FreezeBaseline(ctx, db, name)
		if err != nil {
			log.Fatalf("freeze: %v", err)
		}
		fmt.Printf("baseline %s frozen: %d metrics, window %s..%s, definition %s\n",
			b.Name, len(b.Metrics), b.WindowStart, b.WindowEnd, b.DefinitionSHA)
	case "compare":
		if len(args) < 2 {
			log.Fatal("baseline compare needs a name")
		}
		deltas, err := science.Compare(ctx, db, args[1])
		if err != nil {
			log.Fatalf("compare: %v", err)
		}
		keys := make([]string, 0, len(deltas))
		for k := range deltas {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			d := deltas[k]
			fmt.Printf("%-44s baseline=%.4f current=%.4f delta=%+.4f\n", k, d.Baseline, d.Current, d.Delta)
		}
	default:
		log.Fatalf("unknown baseline subcommand %q", args[0])
	}
}
