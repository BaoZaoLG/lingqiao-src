package main

import (
	"github.com/lingqiao/server/internal/httpapi"

	"context"
	"crypto/tls"
	"database/sql"
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
	"github.com/lingqiao/server/internal/features/agents"
	"github.com/lingqiao/server/internal/features/cards"
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

	sqliteCm, err := NewSQLiteCardManager(dbPath)
	if err != nil {
		log.Fatalf("[DB] Failed to init sqlite: %v", err)
	}

	if needsMigration {
		log.Println("[DB] Migrating from JSON to SQLite...")
		oldCm := NewJSONCardManager(storage)
		if sqliteImpl, ok := sqliteCm.(*SQLiteCardManager); ok {
			if err := sqliteImpl.ImportJSONData(oldCm); err != nil {
				log.Printf("[DB] Migration warning: %v", err)
			} else {
				log.Println("[DB] Migration completed successfully.")
			}
		}
	}

	var cm CardManager = sqliteCm
	initAnnouncement()

	// CLI mode: generate card and exit
	if handleCLIGenerate(cm) {
		return
	}

	log.Println("[SERVER] Starting DLL Injector Auth Server...")

	var db *sql.DB
	if impl, ok := sqliteCm.(*SQLiteCardManager); ok {
		db = impl.DB()
	}

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
	cardsRepo := cards.NewSQLiteRepository(db)
	cardsSvc := cards.NewService(cardsRepo, generateCardCode, time.Now)
	cardsHandler := cards.NewHandler(cardsSvc, cardsRepo)
	cardsHandler.SetAuditSink(func(action, cardCode, detail string) {
		cm.RecordAudit(AuditEntry{
			Action: action,
			Card:   cardCode,
			Detail: detail,
		})
	})

	sessionsRepo := sessions.NewSQLiteRepository(db)
	sessionsSvc := sessions.NewService(sessionsRepo, 30*time.Minute, nil, time.Now)
	sessionsSvc.SetAuditSink(func(action, cardCode, machineID, remoteAddr string) {
		cm.RecordAudit(AuditEntry{
			Action: action,
			Card:   cardCode,
			Detail: fmt.Sprintf("Machine: %s, IP: %s", machineID, remoteAddr),
		})
	})

	inviteSvc := agents.NewInviteService(db)
	agentsRepo := agents.NewSQLiteRepository(db)
	agentsSvc := agents.NewService(agentsRepo, time.Now)
	agentsSvc.SetAuditSink(func(action, agentID, detail string) {
		cm.RecordAudit(AuditEntry{
			Action:  action,
			AgentID: agentID,
			Detail:  detail,
		})
	})

	api := NewAPIHandler(cm, sessionsSvc)
	admin := NewAdminHandler(cm, cardsSvc, sessionsSvc, agentsSvc, inviteSvc)
	admin.chat = api.chat

	adapter := &agentCardManagerAdapter{
		cardsSvc:  cardsSvc,
		agentsSvc: agentsSvc,
	}
	agentAdmin := agents.NewAdminHandler(agentsSvc, adapter)
	agent := agents.NewAgentHandler(agentsSvc, adapter, adapter, inviteSvc, cfg.SessionTTL, cfg.DataDir)
	agent.SetAuditSink(func(action, cardCode, agentID, detail string) {
		cm.RecordAudit(AuditEntry{
			Action:  action,
			Card:    cardCode,
			AgentID: agentID,
			Detail:  detail,
		})
	})
	payloadStore := NewPayloadStore(storage)
	payload := NewPayloadHandler(payloadStore, sessionsSvc)
	payload.SetAuditSink(cm.RecordAudit)

	printStartupInfo(cm)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go api.SessionCleanupTask(ctx)

	adminServer := buildAdminServer(cfg.AdminAddr, api, admin, payload, cardsHandler, agentAdmin)
	agentServer := buildAgentServer(cfg.AgentAddr, agent)

	startServers(adminServer, agentServer)
}

// ── CLI Mode ─────────────────────────────────────────────────────────────────

func handleCLIGenerate(cm CardManager) bool {
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

func buildAdminServer(addr string, api *APIHandler, admin *AdminHandler, payload *PayloadHandler, cardsHandler *cards.Handler, agentAdmin *agents.AdminHandler) *http.Server {
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
	mux.HandleFunc("/api/v1/chat/profile", api.HandleChatProfile)
	mux.HandleFunc("/api/v1/chat/messages", api.HandleChatMessages)
	mux.HandleFunc("/api/v1/chat/presence", api.HandleChatPresence)
	mux.HandleFunc("/api/v1/chat/react", api.HandleChatReact)
	mux.HandleFunc("/api/v1/chat/send", api.HandleChatSend)
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
	registerAdminRoutes(mux, admin, payload, cardsHandler, agentAdmin)

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

func registerPayloadRoutes(mux *http.ServeMux, api *APIHandler, payload *PayloadHandler) {
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

func handleChunkAuth(w http.ResponseWriter, r *http.Request, payload *PayloadHandler) {
	if err := verifyRequestHMAC(r, r.URL.Path); err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "missing auth")
		return
	}
	payload.HandleChunkDownload(w, r)
}

func registerAdminRoutes(mux *http.ServeMux, h *AdminHandler, payload *PayloadHandler, cardsHandler *cards.Handler, agentAdmin *agents.AdminHandler) {
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
	mux.HandleFunc("/admin/api/cards", auth(cardsHandler.HandleListCards))
	mux.HandleFunc("/admin/api/card/generate", auth(cardsHandler.HandleGenerateCard))
	mux.HandleFunc("/admin/api/card/batch-generate", auth(cardsHandler.HandleBatchGenerateCards))
	mux.HandleFunc("/admin/api/card/update", auth(cardsHandler.HandleUpdateCard))
	mux.HandleFunc("/admin/api/card/update-details", auth(cardsHandler.HandleUpdateCardDetails))
	mux.HandleFunc("/admin/api/cards/bulk", auth(cardsHandler.HandleBulkCards))
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

	// Community chat
	mux.HandleFunc("/admin/api/chat/messages", auth(h.HandleChatMessages))
	mux.HandleFunc("/admin/api/chat/delete", auth(h.HandleChatDelete))
	mux.HandleFunc("/admin/api/chat/system", auth(h.HandleChatSystem))
	mux.HandleFunc("/admin/api/chat/mute", auth(h.HandleChatMute))

	// Machines
	mux.HandleFunc("/admin/api/machines", auth(h.HandleMachines))
	mux.HandleFunc("/admin/api/machine/cards", auth(h.HandleMachineCards))

	// Agents
	agentAdmin.RegisterRoutes(mux, auth)

	// Invites
	mux.HandleFunc("/admin/api/invites", auth(h.HandleInviteList))
	mux.HandleFunc("/admin/api/invite/create", auth(h.HandleInviteCreate))
	mux.HandleFunc("/admin/api/invite/delete", auth(h.HandleInviteDelete))

	// Updates
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

func printStartupInfo(cm CardManager) {
	cards := cm.AllCards()
	activeCount := 0
	for _, c := range cards {
		if c.Status == CardActive && time.Now().Before(c.ExpiresAt) {
			activeCount++
		}
	}
	log.Printf("[SERVER] Loaded %d cards (%d active)",
		len(cards), activeCount)
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
