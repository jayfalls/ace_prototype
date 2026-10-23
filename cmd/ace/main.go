package main

import (
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
	"ace/internal/llm"
	"ace/internal/memory"
	"ace/internal/sandbox"
	"ace/internal/server"
	"ace/internal/telemetry"
	_ "modernc.org/sqlite"
)

func main() {
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

	srv := server.NewServer(engine, provider, memBus, memStore, jail, telem, tgBridge, webFS)
	httpServer := &http.Server{
		Addr:              ":" + *port,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		fmt.Printf("\n================================================================\n")
		fmt.Printf("  ACE Glass-Box Cognitive Cockpit: http://localhost:%s\n", *port)
		fmt.Printf("  Active OpenCode Free Model:      %s\n", provider.ActiveModel())
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
