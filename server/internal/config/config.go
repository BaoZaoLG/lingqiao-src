package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds process runtime settings for the Go production entrypoint.
type Config struct {
	AdminAddr  string
	AgentAddr  string
	DataDir    string
	SessionTTL time.Duration
	Cutover    CutoverConfig
}

// CutoverConfig controls optional Java sidecar ownership for selected modules.
type CutoverConfig struct {
	Enabled           bool
	BaseURL           string
	Admin             bool
	Exam              bool
	Audit             bool
	Scripts           bool
	Cards             bool
	Sessions          bool
	Agents            bool
	Releases          bool
	Payloads          bool
	CardsWrite        bool
	BlacklistWrite    bool
	ExamWrite         bool
	SessionsWrite     bool
	AgentsWrite       bool
	ScriptsWrite      bool
	AnnouncementWrite bool
	ChatWrite         bool
	ReleasesWrite     bool
	PayloadsWrite     bool
	AdminWrite        bool
	Chat              bool
	Blacklist         bool
	Announcement      bool
	AgentPortal       bool
	AgentPortalWrite  bool
}

// Load reads environment configuration.
func Load() (Config, error) {
	ttl, err := durationEnv("SESSION_TTL", 4*time.Hour)
	if err != nil {
		return Config{}, err
	}

	return Config{
		AdminAddr:  ":" + stringEnv("PORT", "48901"),
		AgentAddr:  ":" + stringEnv("AGENT_PORT", "38472"),
		DataDir:    stringEnv("DATA_DIR", "data"),
		SessionTTL: ttl,
		Cutover: CutoverConfig{
			Enabled:           boolEnv("JAVA_CUTOVER_ENABLED", false),
			BaseURL:           stringEnv("JAVA_SIDECAR_BASE_URL", "http://127.0.0.1:49001"),
			Admin:             boolEnv("JAVA_CUTOVER_ADMIN", false),
			Exam:              boolEnv("JAVA_CUTOVER_EXAM", false),
			Audit:             boolEnv("JAVA_CUTOVER_AUDIT", false),
			Scripts:           boolEnv("JAVA_CUTOVER_SCRIPTS", false),
			Cards:             boolEnv("JAVA_CUTOVER_CARDS", false),
			Sessions:          boolEnv("JAVA_CUTOVER_SESSIONS", false),
			Agents:            boolEnv("JAVA_CUTOVER_AGENTS", false),
			Releases:          boolEnv("JAVA_CUTOVER_RELEASES", false),
			Payloads:          boolEnv("JAVA_CUTOVER_PAYLOADS", false),
			CardsWrite:        boolEnv("JAVA_CUTOVER_CARDS_WRITE", false),
			BlacklistWrite:    boolEnv("JAVA_CUTOVER_BLACKLIST_WRITE", false),
			ExamWrite:         boolEnv("JAVA_CUTOVER_EXAM_WRITE", false),
			SessionsWrite:     boolEnv("JAVA_CUTOVER_SESSIONS_WRITE", false),
			AgentsWrite:       boolEnv("JAVA_CUTOVER_AGENTS_WRITE", false),
			ScriptsWrite:      boolEnv("JAVA_CUTOVER_SCRIPTS_WRITE", false),
			AnnouncementWrite: boolEnv("JAVA_CUTOVER_ANNOUNCEMENT_WRITE", false),
			ChatWrite:         boolEnv("JAVA_CUTOVER_CHAT_WRITE", false),
			ReleasesWrite:     boolEnv("JAVA_CUTOVER_RELEASES_WRITE", false),
			PayloadsWrite:     boolEnv("JAVA_CUTOVER_PAYLOADS_WRITE", false),
			AdminWrite:        boolEnv("JAVA_CUTOVER_ADMIN_WRITE", false),
			Chat:              boolEnv("JAVA_CUTOVER_CHAT", false),
			Blacklist:         boolEnv("JAVA_CUTOVER_BLACKLIST", false),
			Announcement:      boolEnv("JAVA_CUTOVER_ANNOUNCEMENT", false),
			AgentPortal:       boolEnv("JAVA_CUTOVER_AGENT_PORTAL", false),
			AgentPortalWrite:  boolEnv("JAVA_CUTOVER_AGENT_PORTAL_WRITE", false),
		},
	}, nil
}

func stringEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}
	return d, nil
}

func boolEnv(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}
