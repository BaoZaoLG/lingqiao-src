package main

import (
	"github.com/lingqiao/server/internal/httpapi"

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

	"github.com/lingqiao/server/internal/audit"
	"github.com/lingqiao/server/internal/auth"
	"github.com/lingqiao/server/internal/config"
	"github.com/lingqiao/server/internal/features/agents"
	"github.com/lingqiao/server/internal/features/cards"
	"github.com/lingqiao/server/internal/features/chat"
	"github.com/lingqiao/server/internal/features/payloads"
	"github.com/lingqiao/server/internal/features/releases"
	"github.com/lingqiao/server/internal/features/scripts"
	"github.com/lingqiao/server/internal/features/security/blacklist"
	"github.com/lingqiao/server/internal/features/sessions"
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

	dbPath := cfg.DataDir + "/lingqiao.db"
	jsonPath := cfg.DataDir + "/data.json"

	needsMigration := false
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		if _, err := os.Stat(jsonPath); err == nil {
			needsMigration = true
		}
	}

	db, err := openDB(dbPath)
	if err != nil {
		log.Fatalf("[DB] Failed to init sqlite: %v", err)
	}
	defer db.Close()

	if needsMigration {
		if err := migrateJSONToSQLite(db, storage); err != nil {
			log.Fatalf("[DB] Fatal error during migration: %v", err)
		}
	}

	initAnnouncement()

	cardsRepo := cards.NewSQLiteRepository(db)
	cardsSvc := cards.NewService(cardsRepo, cards.GenerateCardCode, time.Now)

	auditRepo := audit.NewSQLiteRepository(db)
	auditRecorder := audit.NewRecorder(auditRepo)

	// CLI mode: generate card and exit
	if handleCLIGenerate(cardsSvc, auditRecorder) {
		return
	}

	log.Println("[SERVER] Starting DLL Injector Auth Server...")

	// Migrate invites from JSON storage if SQLite database is empty but json storage has invites
	if db != nil {
		var inviteCount int
		err = db.QueryRow(`SELECT COUNT(*) FROM invites`).Scan(&inviteCount)
		if err == nil && inviteCount == 0 {
			var jsonInvites []agents.InviteCode
			if err := storage.Load("invites", &jsonInvites); err == nil && len(jsonInvites) > 0 {
				log.Println("[DB] Migrating invites from JSON to SQLite...")
				tx, err := db.Begin()
				if err == nil {
					var errExec error
					for _, iv := range jsonInvites {
						var usedAt *time.Time
						if !iv.UsedAt.IsZero() {
							usedAt = &iv.UsedAt
						}
						_, errExec = tx.Exec(`INSERT OR IGNORE INTO invites (code, created_at, created_by, used_by, used_at, max_uses, use_count) VALUES (?, ?, ?, ?, ?, ?, ?)`,
							iv.Code, iv.CreatedAt, iv.CreatedBy, iv.UsedBy, usedAt, iv.MaxUses, iv.UseCount)
						if errExec != nil {
							break
						}
					}
					if errExec != nil {
						_ = tx.Rollback()
						log.Printf("[DB] Failed to migrate invites: %v", errExec)
					} else if errCommit := tx.Commit(); errCommit != nil {
						log.Printf("[DB] Failed to commit invites migration: %v", errCommit)
					} else {
						log.Println("[DB] Invites migration completed successfully.")
					}
				}
			}
		}
	}
	blacklistRepo := blacklist.NewSQLiteRepository(db)
	blacklistSvc := blacklist.NewService(blacklistRepo)

	cardsHandler := cards.NewHandler(cardsSvc, cardsRepo)
	cardsHandler.SetAuditSink(func(action, cardCode, detail string) {
		if err := auditRecorder.Append(audit.Event{
			Action: action,
			Card:   cardCode,
			Detail: detail,
		}); err != nil {
			log.Printf("[AUDIT] Failed to record audit for card %s: %v", cardCode, err)
		}
	})

	sessionsRepo := sessions.NewSQLiteRepository(db)
	sessionsSvc := sessions.NewService(sessionsRepo, 30*time.Minute, nil, time.Now)
	sessionsSvc.SetAuditSink(func(action, cardCode, machineID, remoteAddr string) {
		if err := auditRecorder.Append(audit.Event{
			Action:  action,
			Card:    cardCode,
			Machine: machineID,
			IP:      remoteAddr,
		}); err != nil {
			log.Printf("[AUDIT] Failed to record audit for session %s: %v", cardCode, err)
		}
	})

	inviteSvc := agents.NewInviteService(db)
	agentsRepo := agents.NewSQLiteRepository(db)
	agentsSvc := agents.NewService(agentsRepo, time.Now)
	agentsSvc.SetAuditSink(func(action, agentID, detail string) {
		if err := auditRecorder.Append(audit.Event{
			Action:  action,
			AgentID: agentID,
			Detail:  detail,
		}); err != nil {
			log.Printf("[AUDIT] Failed to record audit for agent %s: %v", agentID, err)
		}
	})

	chatSvc := chat.NewService(storage, time.Now)
	chatHandler := chat.NewHandler(chatSvc, sessionsSvc, blacklistSvc)
	releaseHandler := releases.NewHandler(releases.CurrentService(), sessionsSvc, auditRecorder)
	scriptsHandler := scripts.NewHandler(auditRecorder, sessionsSvc)

	api := NewAPIHandler(blacklistSvc, storage, sessionsSvc, releaseHandler)
	admin := NewAdminHandler(cardsSvc, sessionsSvc, agentsSvc, inviteSvc, blacklistSvc, auditRecorder, db)

	adapter := &agentCardManagerAdapter{
		cardsSvc:  cardsSvc,
		agentsSvc: agentsSvc,
	}
	agentAdmin := agents.NewAdminHandler(agentsSvc, adapter)
	agent := agents.NewAgentHandler(agentsSvc, adapter, adapter, inviteSvc, cfg.SessionTTL, cfg.DataDir)
	agent.SetAuditSink(func(action, cardCode, agentID, detail string) {
		if err := auditRecorder.Append(audit.Event{
			Action:  action,
			Card:    cardCode,
			AgentID: agentID,
			Detail:  detail,
		}); err != nil {
			log.Printf("[AUDIT] Failed to record audit for agent %s: %v", agentID, err)
		}
	})
	payloadStore := payloads.NewStore(storage)
	payload := payloads.NewHandler(payloadStore, sessionsSvc, cfg.DataDir)
	payload.SetAuditSink(func(e audit.Event) {
		if err := auditRecorder.Append(e); err != nil {
			log.Printf("[AUDIT] Failed to record audit for payload: %v", err)
		}
	})

	printStartupInfo(cardsSvc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go api.SessionCleanupTask(ctx)

	adminServer := buildAdminServer(cfg.AdminAddr, api, admin, payload, cardsHandler, agentAdmin, chatHandler, releaseHandler, scriptsHandler)
	agentServer := buildAgentServer(cfg.AgentAddr, agent)

	startServers(adminServer, agentServer)
}

// ── CLI Mode ─────────────────────────────────────────────────────────────────

func handleCLIGenerate(cardsSvc *cards.Service, auditRecorder *audit.Recorder) bool {
	genCard := flag.Bool("generate-card", false, "Generate a new card for testing")
	cardDuration := flag.Int("duration", 720, "Card validity duration in hours")
	cardSessions := flag.Int("max-sessions", 1, "Max concurrent sessions")
	cardNote := flag.String("note", "CLI-generated", "Note for the generated card")
	flag.Parse()

	if !*genCard {
		return false
	}

	card, err := cardsSvc.Create(
		context.Background(),
		time.Duration(*cardDuration)*time.Hour,
		*cardNote,
		*cardSessions,
		"",
	)
	if err != nil {
		log.Fatalf("Failed to generate card: %v\n", err)
	}

	if err := auditRecorder.Append(audit.Event{
		Action: "card_generated",
		Card:   card.Code,
		Detail: fmt.Sprintf("Generated %s (note: %s, duration: %dh)", card.Code, card.Note, *cardDuration),
	}); err != nil {
		log.Fatalf("Warning: failed to record audit event: %v", err)
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

func buildAdminServer(addr string, api *APIHandler, admin *AdminHandler, payload *payloads.Handler, cardsHandler *cards.Handler, agentAdmin *agents.AdminHandler, chatHandler *chat.Handler, releaseHandler *releases.Handler, scriptsHandler *scripts.Handler) *http.Server {
	mux := http.NewServeMux()
	activateLimiter := httpapi.NewRateLimiter(1*time.Minute, 20)

	// Client API routes
	mux.HandleFunc("/api/v1/activate", func(w http.ResponseWriter, r *http.Request) {
		if !activateLimiter.Allow(httpapi.GetClientIP(r)) {
			httpapi.WriteError(w, http.StatusTooManyRequests, "激活请求过于频繁，请稍后再试")
			return
		}
		api.HandleActivate(w, r)
	})
	mux.HandleFunc("/api/v1/heartbeat", api.HandleHeartbeat)
	mux.HandleFunc("/api/v1/deactivate", api.HandleDeactivate)
	mux.HandleFunc("/api/v1/announcement", api.HandleAnnouncement)
	mux.HandleFunc("/api/v1/chat/profile", chatHandler.HandleChatProfile)
	mux.HandleFunc("/api/v1/chat/messages", chatHandler.HandleChatMessages)
	mux.HandleFunc("/api/v1/chat/presence", chatHandler.HandleChatPresence)
	mux.HandleFunc("/api/v1/chat/react", chatHandler.HandleChatReact)
	mux.HandleFunc("/api/v1/chat/send", chatHandler.HandleChatSend)
	mux.HandleFunc("/api/v1/dll", api.HandleDllDownload)
	mux.HandleFunc("/api/v1/script", scriptsHandler.HandleScriptDownload)
	mux.HandleFunc("/api/v1/update/check", releaseHandler.HandleUpdateCheck)
	mux.HandleFunc("/api/v1/update/events", releaseHandler.HandleUpdateEvent)
	mux.HandleFunc("/api/v1/update/package/", releaseHandler.HandleUpdatePackageDownload)

	// Payload routes
	registerPayloadRoutes(mux, api, payload)

	// Health check
	mux.HandleFunc("/api/health", healthHandler)

	// Admin routes (authenticated)
	registerAdminRoutes(mux, admin, payload, cardsHandler, agentAdmin, chatHandler, releaseHandler, scriptsHandler)

	// Serve admin panel HTML
	adminSubFS, _ := fs.Sub(webDistFS, "web-dist/admin")
	assetsSubFS, _ := fs.Sub(webDistFS, "web-dist/assets")
	mux.Handle("/admin/", http.StripPrefix("/admin/", http.FileServer(http.FS(adminSubFS))))
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsSubFS))))

	return newServer(addr, corsMiddleware(mux))
}

func buildAgentServer(addr string, agent *agents.AgentHandler) *http.Server {
	mux := http.NewServeMux()
	agent.RegisterRoutes(mux)

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

func registerPayloadRoutes(mux *http.ServeMux, api *APIHandler, payload *payloads.Handler) {
	mux.HandleFunc("/api/v1/key-exchange", func(w http.ResponseWriter, r *http.Request) {
		payload.HandleKeyExchange(w, r)
	})
	mux.HandleFunc("/api/v1/payload/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/info"):
			payload.HandlePayloadInfo(w, r)
		case strings.Contains(path, "/chunk/"):
			handleChunkAuth(w, r, payload)
		default:
			httpapi.WriteError(w, http.StatusBadRequest, "invalid path")
		}
	})
}

func handleChunkAuth(w http.ResponseWriter, r *http.Request, payload *payloads.Handler) {
	if err := auth.VerifyRequest(r, r.URL.Path); err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "missing auth")
		return
	}
	payload.HandleChunkDownload(w, r)
}

func registerAdminRoutes(mux *http.ServeMux, h *AdminHandler, payload *payloads.Handler, cardsHandler *cards.Handler, agentAdmin *agents.AdminHandler, chatHandler *chat.Handler, releaseHandler *releases.Handler, scriptsHandler *scripts.Handler) {
	auth := h.adminAuth
	authP := func(perm string, handler http.HandlerFunc) http.HandlerFunc {
		return auth(h.RequirePermission(perm, handler))
	}

	// Auth & settings
	mux.HandleFunc("/admin/api/login", h.HandleLogin)
	mux.HandleFunc("/admin/api/password", auth(h.HandlePasswordChange))

	// Dashboard & stats
	mux.HandleFunc("/admin/api/dashboard", auth(h.HandleDashboard))
	mux.HandleFunc("/admin/api/stats", auth(h.HandleServerStats))
	mux.HandleFunc("/admin/api/ops/overview", auth(h.HandleOpsOverview))
	mux.HandleFunc("/admin/api/modules/overview", auth(h.HandleModulesOverview))

	// Admin Users
	mux.HandleFunc("/admin/api/admin-users", authP("admins.write", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.HandleAdminUserCreate(w, r)
		} else {
			h.HandleAdminUsersList(w, r)
		}
	}))
	mux.HandleFunc("/admin/api/admin-user/update", authP("admins.write", h.HandleAdminUserUpdate))

	// Card management
	mux.HandleFunc("/admin/api/cards", authP("cards.read", cardsHandler.HandleListCards))
	mux.HandleFunc("/admin/api/card/generate", authP("cards.write", cardsHandler.HandleGenerateCard))
	mux.HandleFunc("/admin/api/card/batch-generate", authP("cards.write", cardsHandler.HandleBatchGenerateCards))
	mux.HandleFunc("/admin/api/card/update", authP("cards.write", cardsHandler.HandleUpdateCard))
	mux.HandleFunc("/admin/api/card/update-details", authP("cards.write", cardsHandler.HandleUpdateCardDetails))
	mux.HandleFunc("/admin/api/cards/bulk", authP("cards.bulk", cardsHandler.HandleBulkCards))
	mux.HandleFunc("/admin/api/cards/export", authP("cards.read", h.HandleExportCards))
	mux.HandleFunc("/admin/api/cards/import", authP("cards.write", h.HandleImportCards))

	// Sessions & blacklist
	mux.HandleFunc("/admin/api/sessions", authP("sessions.read", h.HandleListSessions))
	mux.HandleFunc("/admin/api/force-logout", authP("sessions.force_logout", h.HandleForceLogout))
	mux.HandleFunc("/admin/api/blacklist", authP("system.read", h.HandleBlacklist))
	mux.HandleFunc("/admin/api/audit", authP("audit.read", h.HandleAuditLog))

	// Announcement
	mux.HandleFunc("/admin/api/announcement", auth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.HandleAnnouncementSet(w, r)
		} else {
			h.HandleAnnouncementGet(w, r)
		}
	}))

	// Community chat
	mux.HandleFunc("/admin/api/chat/messages", auth(chatHandler.HandleAdminChatMessages))
	mux.HandleFunc("/admin/api/chat/delete", auth(chatHandler.HandleChatDelete))
	mux.HandleFunc("/admin/api/chat/system", auth(chatHandler.HandleChatSystem))
	mux.HandleFunc("/admin/api/chat/mute", auth(chatHandler.HandleChatMute))

	// Machines
	mux.HandleFunc("/admin/api/machines", authP("agents.read", h.HandleAdminMachines))
	mux.HandleFunc("/admin/api/machine/cards", authP("agents.read", h.HandleMachineCards))

	// Agents
	agentAdmin.RegisterRoutes(mux, authP)

	// Invites
	mux.HandleFunc("/admin/api/invites", authP("invites.write", h.HandleInviteList))
	mux.HandleFunc("/admin/api/invite/create", authP("invites.write", h.HandleInviteCreate))
	mux.HandleFunc("/admin/api/invite/delete", authP("invites.write", h.HandleInviteDelete))

	// Updates
	mux.HandleFunc("/admin/api/releases", authP("releases.write", releaseHandler.HandleReleases))
	mux.HandleFunc("/admin/api/releases/", authP("releases.write", releaseHandler.HandleReleaseRoute))
	mux.HandleFunc("/admin/api/script", authP("scripts.write", scriptsHandler.HandleScriptAdmin))

	// Payload upload (requires admin auth + upload key)
	mux.HandleFunc("/admin/api/payloads", authP("payloads.write", payload.HandleAdminList))
	mux.HandleFunc("/admin/api/payloads/manage", authP("payloads.write", payload.HandleAdminManage))
	mux.HandleFunc("/admin/api/payload/upload", authP("payloads.write", payload.HandleUpload))
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

func printStartupInfo(svc *cards.Service) {
	ctx := context.Background()
	allCards, err := svc.AllCards(ctx)
	if err != nil {
		log.Printf("[SERVER] Could not load card count: %v", err)
		return
	}
	now := time.Now()
	activeCount := 0
	for _, c := range allCards {
		if c.Status == cards.StatusActive && now.Before(c.ExpiresAt) {
			activeCount++
		}
	}
	log.Printf("[SERVER] Loaded %d cards (%d active)",
		len(allCards), activeCount)
	if len(allCards) == 0 {
		log.Println("[SERVER] No cards in database — generate one with: go run . --generate-card")
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
