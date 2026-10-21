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
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/lingqiao/server/internal/audit"
	"github.com/lingqiao/server/internal/auth"
	"github.com/lingqiao/server/internal/config"
	"github.com/lingqiao/server/internal/cutover"
	"github.com/lingqiao/server/internal/features/agents"
	authorizations "github.com/lingqiao/server/internal/features/authorizations"
	"github.com/lingqiao/server/internal/features/cards"
	"github.com/lingqiao/server/internal/features/chat"
	"github.com/lingqiao/server/internal/features/exam"
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

	// Migrate invites from JSON storage if needed.
	if err := migrateJSONInvitesToSQLite(db, storage); err != nil {
		log.Printf("[DB] Failed to migrate invites: %v", err)
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
	sessionsSvc := sessions.NewService(sessionsRepo, cfg.SessionTTL, nil, time.Now)
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
	authorizationsSvc := authorizations.NewService(cardsRepo, sessionsRepo, time.Now)
	authorizationsHandler := authorizations.NewHandler(authorizationsSvc)
	authorizationsHandler.SetAuditSink(func(action, cardCode, detail string) {
		if err := auditRecorder.Append(audit.Event{
			Action: action,
			Card:   cardCode,
			Detail: detail,
		}); err != nil {
			log.Printf("[AUDIT] Failed to record audit for authorization %s: %v", cardCode, err)
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
	scriptsHandler := scripts.NewHandler(cfg.DataDir, auditRecorder, sessionsSvc)

	api := NewAPIHandler(blacklistSvc, storage, sessionsSvc, releaseHandler)
	admin, err := NewAdminHandler(cardsSvc, sessionsSvc, agentsSvc, inviteSvc, blacklistSvc, auditRecorder, db)
	if err != nil {
		log.Fatalf("[ADMIN] Failed to initialize admin handler: %v", err)
	}

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
	payload, err := payloads.NewHandler(payloadStore, sessionsSvc, cfg.DataDir)
	if err != nil {
		log.Fatalf("[PAYLOAD] Failed to initialize payload handler: %v", err)
	}
	api.SetPayloadStore(payloadStore, cfg.DataDir)
	payload.SetAuditSink(func(e audit.Event) {
		if err := auditRecorder.Append(e); err != nil {
			log.Printf("[AUDIT] Failed to record audit for payload: %v", err)
		}
	})

	examRepo := exam.NewRepository(db)
	examHandler := exam.NewHandler(examRepo, sessionsSvc)

	printStartupInfo(cardsSvc)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go api.SessionCleanupTask(ctx)

	cutoverRouter, err := cutover.New(cfg.Cutover)
	if err != nil {
		log.Fatalf("[CUTOVER] Failed to initialize Java sidecar proxy: %v", err)
	}
	adminServer, err := buildAdminServer(cfg.AdminAddr, api, admin, payload, cardsHandler, authorizationsHandler, agentAdmin, chatHandler, releaseHandler, scriptsHandler, examHandler, cutoverRouter)
	if err != nil {
		log.Fatalf("[SERVER] Failed to build admin server: %v", err)
	}
	agentServer, err := buildAgentServer(cfg.AgentAddr, agent, cutoverRouter)
	if err != nil {
		log.Fatalf("[SERVER] Failed to build agent server: %v", err)
	}

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

func buildAdminServer(addr string, api *APIHandler, admin *AdminHandler, payload *payloads.Handler, cardsHandler *cards.Handler, authorizationsHandler *authorizations.Handler, agentAdmin *agents.AdminHandler, chatHandler *chat.Handler, releaseHandler *releases.Handler, scriptsHandler *scripts.Handler, examHandler *exam.Handler, cutoverRouter *cutover.Router) (*http.Server, error) {
	mux := http.NewServeMux()
	activateLimiter := httpapi.NewRateLimiter(1*time.Minute, 20)
	activateHandler := cutoverRouter.Wrap("sessions", api.HandleActivate)

	// Client API routes
	mux.HandleFunc("/api/v1/activate", func(w http.ResponseWriter, r *http.Request) {
		if !activateLimiter.Allow(httpapi.GetClientIP(r)) {
			httpapi.WriteError(w, http.StatusTooManyRequests, "激活请求过于频繁，请稍后再试")
			return
		}
		activateHandler(w, r)
	})
	mux.HandleFunc("/api/v1/heartbeat", cutoverRouter.Wrap("sessions", api.HandleHeartbeat))
	mux.HandleFunc("/api/v1/deactivate", cutoverRouter.Wrap("sessions", api.HandleDeactivate))
	mux.HandleFunc("/api/v1/announcement", cutoverRouter.Wrap("announcement", api.HandleAnnouncement))
	mux.HandleFunc("/api/v1/chat/profile", cutoverRouter.Wrap("chat", chatHandler.HandleChatProfile))
	mux.HandleFunc("/api/v1/chat/messages", cutoverRouter.Wrap("chat", chatHandler.HandleChatMessages))
	mux.HandleFunc("/api/v1/chat/presence", cutoverRouter.Wrap("chat", chatHandler.HandleChatPresence))
	mux.HandleFunc("/api/v1/chat/react", cutoverRouter.Wrap("chat", chatHandler.HandleChatReact))
	mux.HandleFunc("/api/v1/chat/send", cutoverRouter.Wrap("chat", chatHandler.HandleChatSend))
	mux.HandleFunc("/api/v1/dll", cutoverRouter.Wrap("payloads", api.HandleDllDownload))
	mux.HandleFunc("/api/v1/script", cutoverRouter.Wrap("scripts", scriptsHandler.HandleScriptDownload))
	mux.HandleFunc("/api/v1/update/check", cutoverRouter.Wrap("releases", releaseHandler.HandleUpdateCheck))
	mux.HandleFunc("/api/v1/update/events", cutoverRouter.Wrap("releases", releaseHandler.HandleUpdateEvent))
	mux.HandleFunc("/api/v1/update/package/", cutoverRouter.Wrap("releases", releaseHandler.HandleUpdatePackageDownload))

	// Exam reporting (client API, HMAC + session auth)
	mux.HandleFunc("/api/v1/exam/report", cutoverRouter.Wrap("exam", examHandler.HandleReport))
	// Exam collect (page-context, session token only — no HMAC)
	mux.HandleFunc("/api/v1/exam/collect", cutoverRouter.Wrap("exam", examHandler.HandleCollect))

	// Payload routes
	registerPayloadRoutes(mux, api, payload, cutoverRouter)

	// Health check
	mux.HandleFunc("/api/health", healthHandler)

	// Admin routes (authenticated)
	registerAdminRoutes(mux, admin, payload, cardsHandler, authorizationsHandler, agentAdmin, chatHandler, releaseHandler, scriptsHandler, examHandler, cutoverRouter)

	// Serve admin panel HTML
	adminSubFS, err := fs.Sub(webDistFS, "web-dist/admin")
	if err != nil {
		return nil, err
	}
	assetsSubFS, err := fs.Sub(webDistFS, "web-dist/assets")
	if err != nil {
		return nil, err
	}
	mux.Handle("/admin/", http.StripPrefix("/admin/", http.FileServer(http.FS(adminSubFS))))
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsSubFS))))

	return newServer(addr, corsMiddleware(mux)), nil
}

func buildAgentServer(addr string, agent *agents.AgentHandler, cutoverRouter *cutover.Router) (*http.Server, error) {
	mux := http.NewServeMux()
	agent.RegisterRoutes(mux)

	// Health check
	mux.HandleFunc("/api/health", healthHandler)

	// Serve agent panel HTML
	agentSubFS, err := fs.Sub(webDistFS, "web-dist/agent")
	if err != nil {
		return nil, err
	}
	assetsSubFS, err := fs.Sub(webDistFS, "web-dist/assets")
	if err != nil {
		return nil, err
	}
	mux.Handle("/agent/", http.StripPrefix("/agent/", http.FileServer(http.FS(agentSubFS))))
	mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.FS(assetsSubFS))))
	mux.Handle("/", http.FileServer(http.FS(agentSubFS)))

	// Agent portal routes may be owned by the Java sidecar when cutover is enabled.
	portal := cutoverRouter.Wrap("agent-portal", mux.ServeHTTP)

	return newServer(addr, corsMiddlewareAgent(http.HandlerFunc(portal))), nil
}

// ── Route Registration ───────────────────────────────────────────────────────

func registerPayloadRoutes(mux *http.ServeMux, api *APIHandler, payload *payloads.Handler, cutoverRouter *cutover.Router) {
	mux.HandleFunc("/api/v1/key-exchange", cutoverRouter.Wrap("payloads", func(w http.ResponseWriter, r *http.Request) {
		payload.HandleKeyExchange(w, r)
	}))
	mux.HandleFunc("/api/v1/payload/", cutoverRouter.Wrap("payloads", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/info"):
			payload.HandlePayloadInfo(w, r)
		case strings.Contains(path, "/chunk/"):
			handleChunkAuth(w, r, payload)
		default:
			httpapi.WriteError(w, http.StatusBadRequest, "invalid path")
		}
	}))
}

func handleChunkAuth(w http.ResponseWriter, r *http.Request, payload *payloads.Handler) {
	if err := auth.VerifyRequest(r, r.URL.Path); err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "missing auth")
		return
	}
	payload.HandleChunkDownload(w, r)
}

func registerAdminRoutes(mux *http.ServeMux, h *AdminHandler, payload *payloads.Handler, cardsHandler *cards.Handler, authorizationsHandler *authorizations.Handler, agentAdmin *agents.AdminHandler, chatHandler *chat.Handler, releaseHandler *releases.Handler, scriptsHandler *scripts.Handler, examHandler *exam.Handler, cutoverRouter *cutover.Router) {
	auth := h.adminAuth
	authP := func(perm string, handler http.HandlerFunc) http.HandlerFunc {
		return auth(h.RequirePermission(perm, handler))
	}
	if cutoverRouter == nil {
		cutoverRouter, _ = cutover.New(config.CutoverConfig{})
	}

	// Auth & settings
	mux.HandleFunc("/admin/api/login", cutoverRouter.Wrap("admin", h.HandleLogin))
	mux.HandleFunc("/admin/api/password", auth(cutoverRouter.Wrap("admin", h.HandlePasswordChange)))

	// Dashboard & stats
	mux.HandleFunc("/admin/api/dashboard", auth(cutoverRouter.Wrap("admin", h.HandleDashboard)))
	mux.HandleFunc("/admin/api/stats", auth(cutoverRouter.Wrap("admin", h.HandleServerStats)))
	mux.HandleFunc("/admin/api/ops/overview", auth(cutoverRouter.Wrap("admin", h.HandleOpsOverview)))
	mux.HandleFunc("/admin/api/modules/overview", auth(cutoverRouter.Wrap("admin", h.HandleModulesOverview)))

	// Admin Users
	mux.HandleFunc("/admin/api/admin-users", authP("admins.write", cutoverRouter.Wrap("admin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.HandleAdminUserCreate(w, r)
		} else {
			h.HandleAdminUsersList(w, r)
		}
	})))
	mux.HandleFunc("/admin/api/admin-user/update", authP("admins.write", cutoverRouter.Wrap("admin", h.HandleAdminUserUpdate)))

	// Card management
	mux.HandleFunc("/admin/api/cards", authP("cards.read", cutoverRouter.Wrap("cards", cardsHandler.HandleListCards)))
	mux.HandleFunc("/admin/api/card/generate", authP("cards.write", cutoverRouter.Wrap("cards", cardsHandler.HandleGenerateCard)))
	mux.HandleFunc("/admin/api/card/batch-generate", authP("cards.write", cutoverRouter.Wrap("cards", cardsHandler.HandleBatchGenerateCards)))
	mux.HandleFunc("/admin/api/card/update", authP("cards.write", cutoverRouter.Wrap("cards", cardsHandler.HandleUpdateCard)))
	mux.HandleFunc("/admin/api/card/update-details", authP("cards.write", cutoverRouter.Wrap("cards", cardsHandler.HandleUpdateCardDetails)))
	mux.HandleFunc("/admin/api/cards/bulk", authP("cards.bulk", cutoverRouter.Wrap("cards", cardsHandler.HandleBulkCards)))
	mux.HandleFunc("/admin/api/cards/export", authP("cards.read", cutoverRouter.Wrap("cards", h.HandleExportCards)))
	mux.HandleFunc("/admin/api/cards/import", authP("cards.write", cutoverRouter.Wrap("cards", h.HandleImportCards)))
	mux.HandleFunc("/admin/api/control/authorizations", authP("cards.read", cutoverRouter.Wrap("cards", authorizationsHandler.Handle)))
	mux.HandleFunc("/admin/api/control/authorizations/", authP("cards.read", cutoverRouter.Wrap("cards", authorizationsHandler.Handle)))
	mux.HandleFunc("/admin/api/control/authorizations/bulk/impact", authP("cards.bulk", cutoverRouter.Wrap("cards", authorizationsHandler.HandleBulkImpact)))
	mux.HandleFunc("/admin/api/control/authorizations/bulk/actions", authP("cards.bulk", cutoverRouter.Wrap("cards", authorizationsHandler.HandleBulkAction)))

	// Sessions & blacklist
	mux.HandleFunc("/admin/api/sessions", authP("sessions.read", cutoverRouter.Wrap("sessions", h.HandleListSessions)))
	mux.HandleFunc("/admin/api/force-logout", authP("sessions.force_logout", cutoverRouter.Wrap("sessions", h.HandleForceLogout)))
	mux.HandleFunc("/admin/api/blacklist", authP("system.read", cutoverRouter.Wrap("blacklist", h.HandleBlacklist)))
	mux.HandleFunc("/admin/api/audit", authP("audit.read", cutoverRouter.Wrap("audit", h.HandleAuditLog)))

	// Announcement
	mux.HandleFunc("/admin/api/announcement", auth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.RequirePermission(
				"announcement.write",
				cutoverRouter.Wrap("announcement", h.HandleAnnouncementSet),
			)(w, r)
		} else {
			h.RequirePermission("announcement.read", cutoverRouter.Wrap("announcement", h.HandleAnnouncementGet))(w, r)
		}
	}))

	// Community chat
	mux.HandleFunc("/admin/api/chat/messages", authP("chat.read", cutoverRouter.Wrap("chat", chatHandler.HandleAdminChatMessages)))
	mux.HandleFunc("/admin/api/chat/delete", authP("chat.write", cutoverRouter.Wrap("chat", chatHandler.HandleChatDelete)))
	mux.HandleFunc("/admin/api/chat/system", authP("chat.write", cutoverRouter.Wrap("chat", chatHandler.HandleChatSystem)))
	mux.HandleFunc("/admin/api/chat/mute", authP("chat.write", cutoverRouter.Wrap("chat", chatHandler.HandleChatMute)))

	// Machines
	mux.HandleFunc("/admin/api/machines", authP("agents.read", cutoverRouter.Wrap("agents", h.HandleAdminMachines)))
	mux.HandleFunc("/admin/api/machine/cards", authP("agents.read", cutoverRouter.Wrap("agents", h.HandleMachineCards)))

	// Agents
	agentAdmin.RegisterRoutesWithCutover(mux, authP, cutoverRouter.Wrap)

	// Invites
	mux.HandleFunc("/admin/api/invites", authP("invites.write", cutoverRouter.Wrap("admin", h.HandleInviteList)))
	mux.HandleFunc("/admin/api/invite/create", authP("invites.write", cutoverRouter.Wrap("admin", h.HandleInviteCreate)))
	mux.HandleFunc("/admin/api/invite/delete", authP("invites.write", cutoverRouter.Wrap("admin", h.HandleInviteDelete)))

	// Client releases
	mux.HandleFunc("/admin/api/releases", authP("releases.write", cutoverRouter.Wrap("releases", releaseHandler.HandleReleases)))
	mux.HandleFunc("/admin/api/releases/", authP("releases.write", cutoverRouter.Wrap("releases", releaseHandler.HandleReleaseRoute)))

	// Script repository (new RESTful endpoints)
	mux.HandleFunc("/admin/api/scripts", authP("scripts.write", cutoverRouter.Wrap("scripts", scriptsHandler.HandleList)))
	mux.HandleFunc("/admin/api/scripts/{id}", authP("scripts.write", cutoverRouter.Wrap("scripts", scriptsHandler.HandleGet)))
	mux.HandleFunc("/admin/api/scripts/create", authP("scripts.write", cutoverRouter.Wrap("scripts", scriptsHandler.HandleCreate)))
	mux.HandleFunc("/admin/api/scripts/publish", authP("scripts.write", cutoverRouter.Wrap("scripts", scriptsHandler.HandlePublish)))
	mux.HandleFunc("/admin/api/scripts/{id}/activate", authP("scripts.write", cutoverRouter.Wrap("scripts", scriptsHandler.HandleActivate)))
	mux.HandleFunc("/admin/api/scripts/{id}/delete", authP("scripts.write", cutoverRouter.Wrap("scripts", scriptsHandler.HandleDelete)))
	// Legacy compatibility endpoint
	mux.HandleFunc("/admin/api/script", authP("scripts.write", cutoverRouter.Wrap("scripts", scriptsHandler.HandleScriptAdmin)))

	// Payload upload (requires admin auth + upload key)
	mux.HandleFunc("/admin/api/payloads", authP("payloads.write", cutoverRouter.Wrap("payloads", payload.HandleAdminList)))
	mux.HandleFunc("/admin/api/payloads/manage", authP("payloads.write", cutoverRouter.Wrap("payloads", payload.HandleAdminManage)))
	mux.HandleFunc("/admin/api/payload/upload", authP("payloads.write", cutoverRouter.Wrap("payloads", payload.HandleUpload)))
	mux.HandleFunc("/admin/api/payloads/upload-dll", authP("payloads.write", cutoverRouter.Wrap("payloads", payload.HandleAdminDLLUpload)))

	// Exam records
	mux.HandleFunc("/admin/api/exam/records", authP("exam.read", cutoverRouter.Wrap("exam", examHandler.HandleAdminList)))
	mux.HandleFunc("/admin/api/exam/stats", authP("exam.read", cutoverRouter.Wrap("exam", examHandler.HandleAdminStats)))
	mux.HandleFunc("/admin/api/exam/export", authP("exam.read", cutoverRouter.Wrap("exam", examHandler.HandleAdminExport)))
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
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); adminServer.Shutdown(ctx) }()
		go func() { defer wg.Done(); agentServer.Shutdown(ctx) }()
		wg.Wait()
	}()

	// Start agent server
	go func() {
		log.Printf("[SERVER] Agent panel listening on %s (TLS)", agentServer.Addr)
		if err := agentServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Printf("[SERVER] Agent server failed: %v", err)
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
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}

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
