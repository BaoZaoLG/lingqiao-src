package main

import (
	"context"
	"crypto/tls"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/lingqiao/server/internal/config"
)

//go:embed web-dist/*
var webDistFS embed.FS

var (
	serverStart  time.Time
	requestCount atomic.Int64
)

func main() {
	serverStart = time.Now()
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[CONFIG] %v", err)
	}
	ConfigureRuntime(RuntimeConfig{
		DataDir:    cfg.DataDir,
		SessionTTL: cfg.SessionTTL,
	})

	storage := NewJSONStorage(cfg.DataDir)
	cm := NewCardManager(storage)
	initAnnouncement()

	// CLI mode: generate card and exit
	if handleCLIGenerate(cm) {
		return
	}

	log.Println("[SERVER] Starting DLL Injector Auth Server...")

	api := NewAPIHandler(cm)
	admin := NewAdminHandler(cm)
	agent := NewAgentHandler(cm)
	payloadStore := NewPayloadStore(storage)
	payload := NewPayloadHandler(payloadStore)
	payload.SetAuditSink(cm.RecordAudit)

	printStartupInfo(cm)

	go api.SessionCleanupTask()

	adminServer := buildAdminServer(cfg.AdminAddr, api, admin, payload)
	agentServer := buildAgentServer(cfg.AgentAddr, agent)

	startServers(adminServer, agentServer)
}

// ── CLI Mode ─────────────────────────────────────────────────────────────────

func handleCLIGenerate(cm *CardManager) bool {
	genCard := flag.Bool("generate-card", false, "Generate a new card for testing")
	cardDuration := flag.Int("duration", 720, "Card validity duration in hours")
	cardSessions := flag.Int("max-sessions", 1, "Max concurrent sessions")
	cardNote := flag.String("note", "CLI-generated", "Note for the generated card")
	flag.Parse()

	if !*genCard {
		return false
	}

	card, err := cm.GenerateCard(
		time.Duration(*cardDuration)*time.Hour,
		*cardNote,
		*cardSessions,
		"",
	)
	if err != nil {
		log.Fatalf("[GEN] Failed to generate card: %v", err)
	}
	fmt.Printf("Card generated successfully:\n")
	fmt.Printf("  Code:         %s\n", card.Code)
	fmt.Printf("  Expires at:   %s\n", card.ExpiresAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Max sessions: %d\n", card.MaxSessions)
	fmt.Printf("  Note:         %s\n", card.Note)
	fmt.Printf("\nCopy the code above into the Injector client to activate.\n")
	return true
}

// ── Server Construction ──────────────────────────────────────────────────────

func buildAdminServer(addr string, api *APIHandler, admin *AdminHandler, payload *PayloadHandler) *http.Server {
	mux := http.NewServeMux()
	activateLimiter := newRateLimiter(1*time.Minute, 20)

	// Client API routes
	mux.HandleFunc("/api/v1/activate", func(w http.ResponseWriter, r *http.Request) {
		if !activateLimiter.allow(getClientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "激活请求过于频繁，请稍后再试")
			return
		}
		api.HandleActivate(w, r)
	})
	mux.HandleFunc("/api/v1/heartbeat", api.HandleHeartbeat)
	mux.HandleFunc("/api/v1/deactivate", api.HandleDeactivate)
	mux.HandleFunc("/api/v1/announcement", api.HandleAnnouncement)
	mux.HandleFunc("/api/v1/dll", api.HandleDllDownload)
	mux.HandleFunc("/api/v1/script", api.HandleScriptDownload)
	mux.HandleFunc("/api/v1/update/check", api.HandleUpdateCheck)
	mux.HandleFunc("/api/v1/update/events", api.HandleUpdateEvent)
	mux.HandleFunc("/api/v1/update/package/", api.HandleUpdatePackageDownload)

	// Payload routes
	registerPayloadRoutes(mux, api, payload)

	// Health check
	mux.HandleFunc("/api/health", healthHandler)

	// Admin routes (authenticated)
	registerAdminRoutes(mux, admin, payload)

	// Serve admin panel HTML
	adminSubFS, _ := fs.Sub(webDistFS, "web-dist/admin")
	assetsSubFS, _ := fs.Sub(webDistFS, "web-dist/assets")
	mux.Handle("/admin/", http.StripPrefix("/admin/", http.FileServer(http.FS(adminSubFS))))
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsSubFS))))

	return newServer(addr, corsMiddleware(mux))
}

func buildAgentServer(addr string, agent *AgentHandler) *http.Server {
	mux := http.NewServeMux()

	// Agent API routes (no auth required)
	mux.HandleFunc("/api/register", agent.HandleRegister)
	mux.HandleFunc("/api/login", agent.HandleLogin)

	// Agent API routes (authenticated)
	mux.HandleFunc("/api/dashboard", agent.agentAuth(agent.HandleDashboard))
	mux.HandleFunc("/api/cards", agent.agentAuth(agent.HandleListCards))
	mux.HandleFunc("/api/card/generate", agent.agentAuth(agent.HandleGenerateCard))
	mux.HandleFunc("/api/card/batch-generate", agent.agentAuth(agent.HandleBatchGenerate))
	mux.HandleFunc("/api/info", agent.agentAuth(agent.HandleAgentInfo))
	mux.HandleFunc("/api/password", agent.agentAuth(agent.HandlePasswordChange))

	// Health check
	mux.HandleFunc("/api/health", healthHandler)

	// Serve agent panel HTML
	agentSubFS, _ := fs.Sub(webDistFS, "web-dist/agent")
	assetsSubFS, _ := fs.Sub(webDistFS, "web-dist/assets")
	mux.Handle("/agent/", http.StripPrefix("/agent/", http.FileServer(http.FS(agentSubFS))))
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsSubFS))))
	mux.Handle("/", http.FileServer(http.FS(agentSubFS)))

	return newServer(addr, corsMiddlewareAgent(mux))
}

// ── Route Registration ───────────────────────────────────────────────────────

func registerPayloadRoutes(mux *http.ServeMux, api *APIHandler, payload *PayloadHandler) {
	mux.HandleFunc("/api/v1/key-exchange", func(w http.ResponseWriter, r *http.Request) {
		payload.HandleKeyExchange(w, r, api.cm)
	})
	mux.HandleFunc("/api/v1/payload/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/info"):
			payload.HandlePayloadInfo(w, r)
		case strings.Contains(path, "/chunk/"):
			handleChunkAuth(w, r, payload)
		default:
			writeError(w, http.StatusBadRequest, "invalid path")
		}
	})
}

func handleChunkAuth(w http.ResponseWriter, r *http.Request, payload *PayloadHandler) {
	if err := verifyRequestHMAC(r, r.URL.Path); err != nil {
		writeError(w, http.StatusUnauthorized, "missing auth")
		return
	}
	payload.HandleChunkDownload(w, r)
}

func registerAdminRoutes(mux *http.ServeMux, h *AdminHandler, payload *PayloadHandler) {
	auth := h.adminAuth

	// Auth & settings
	mux.HandleFunc("/admin/api/login", h.HandleLogin)
	mux.HandleFunc("/admin/api/password", auth(h.HandlePasswordChange))

	// Dashboard & stats
	mux.HandleFunc("/admin/api/dashboard", auth(h.HandleDashboard))
	mux.HandleFunc("/admin/api/stats", auth(h.HandleServerStats))
	mux.HandleFunc("/admin/api/ops/overview", auth(h.HandleOpsOverview))
	mux.HandleFunc("/admin/api/modules/overview", auth(h.HandleModulesOverview))

	// Card management
	mux.HandleFunc("/admin/api/cards", auth(h.HandleListCards))
	mux.HandleFunc("/admin/api/card/generate", auth(h.HandleGenerateCard))
	mux.HandleFunc("/admin/api/card/batch-generate", auth(h.HandleBatchGenerateCards))
	mux.HandleFunc("/admin/api/card/update", auth(h.HandleUpdateCard))
	mux.HandleFunc("/admin/api/card/update-details", auth(h.HandleUpdateCardDetails))
	mux.HandleFunc("/admin/api/cards/bulk", auth(h.HandleBulkCards))
	mux.HandleFunc("/admin/api/cards/export", auth(h.HandleExportCards))
	mux.HandleFunc("/admin/api/cards/import", auth(h.HandleImportCards))

	// Sessions & blacklist
	mux.HandleFunc("/admin/api/sessions", auth(h.HandleListSessions))
	mux.HandleFunc("/admin/api/force-logout", auth(h.HandleForceLogout))
	mux.HandleFunc("/admin/api/blacklist", auth(h.HandleBlacklist))
	mux.HandleFunc("/admin/api/audit", auth(h.HandleAuditLog))

	// Announcement
	mux.HandleFunc("/admin/api/announcement", auth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.HandleAnnouncementSet(w, r)
		} else {
			h.HandleAnnouncementGet(w, r)
		}
	}))

	// Machines
	mux.HandleFunc("/admin/api/machines", auth(h.HandleMachines))
	mux.HandleFunc("/admin/api/machine/cards", auth(h.HandleMachineCards))

	// Agents
	mux.HandleFunc("/admin/api/agents", auth(h.HandleAdminListAgents))
	mux.HandleFunc("/admin/api/agent/update", auth(h.HandleAdminUpdateAgent))
	mux.HandleFunc("/admin/api/agent/cards", auth(h.HandleAdminAgentCards))

	// Invites
	mux.HandleFunc("/admin/api/invites", auth(h.HandleInviteList))
	mux.HandleFunc("/admin/api/invite/create", auth(h.HandleInviteCreate))
	mux.HandleFunc("/admin/api/invite/delete", auth(h.HandleInviteDelete))

	// Updates
	mux.HandleFunc("/admin/api/update/upload", auth(h.HandleUpdateUpload))
	mux.HandleFunc("/admin/api/update/info", auth(h.HandleUpdateInfo))
	mux.HandleFunc("/admin/api/update/manage", auth(h.HandleUpdateManage))
	mux.HandleFunc("/admin/api/update/download", h.HandleUpdateDownload)
	mux.HandleFunc("/admin/api/releases", auth(h.HandleReleases))
	mux.HandleFunc("/admin/api/releases/", auth(h.HandleReleaseRoute))
	mux.HandleFunc("/admin/api/script", auth(h.HandleScriptAdmin))

	// Payload upload (requires admin auth + upload key)
	mux.HandleFunc("/admin/api/payloads", auth(payload.HandleAdminList))
	mux.HandleFunc("/admin/api/payloads/manage", auth(payload.HandleAdminManage))
	mux.HandleFunc("/admin/api/payload/upload", auth(payload.HandleUpload))
}

// ── Server Helpers ───────────────────────────────────────────────────────────

func newServer(addr string, handler http.Handler) *http.Server {
	tlsConfig, err := getTLSConfig()
	if err != nil {
		log.Fatalf("[SERVER] Failed to setup TLS: %v", err)
	}

	srv := &http.Server{
		Addr:         addr,
		Handler:      handler,
		TLSConfig:    tlsConfig,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	// Note: payload upload endpoints need longer timeouts.
	// Use http.TimeoutHandler or per-route wrappers for those.
	srv.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler))
	return srv
}

func startServers(adminServer, agentServer *http.Server) {
	log.Printf("[SERVER] TLS enabled (cert: certs/server.crt, key: certs/server.key)")
	log.Printf("[SERVER] Admin panel:  https://localhost%s/admin/", adminServer.Addr)
	log.Printf("[SERVER] Agent panel:  https://localhost%s/", agentServer.Addr)
	log.Printf("[SERVER] Client API:   https://localhost%s/api/v1/activate", adminServer.Addr)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("[SERVER] Received signal %v, shutting down...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		adminServer.Shutdown(ctx)
		agentServer.Shutdown(ctx)
	}()

	// Start agent server
	go func() {
		log.Printf("[SERVER] Agent panel listening on %s (TLS)", agentServer.Addr)
		if err := agentServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[SERVER] Agent server failed: %v", err)
		}
	}()

	// Start admin server (blocking)
	log.Printf("[SERVER] Admin+API listening on %s (TLS)", adminServer.Addr)
	if err := adminServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[SERVER] Admin server failed: %v", err)
	}
	log.Println("[SERVER] All servers stopped")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok","time":"` + time.Now().Format(time.RFC3339) + `"}`))
}

func printStartupInfo(cm *CardManager) {
	cards := cm.AllCards()
	activeCount := 0
	for _, c := range cards {
		if c.Status == CardActive && time.Now().Before(c.ExpiresAt) {
			activeCount++
		}
	}
	log.Printf("[SERVER] Loaded %d cards (%d active), %d sessions",
		len(cards), activeCount, len(cm.AllSessions()))
	if len(cards) == 0 {
		log.Println("[SERVER] No cards in database — generate one with: go run . --generate-card")
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
