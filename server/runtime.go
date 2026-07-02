package main

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/lingqiao/server/internal/auth"
	"github.com/lingqiao/server/internal/features/releases"
)

// RuntimeConfig ...
type RuntimeConfig struct {
	DataDir    string
	SessionTTL time.Duration
}

var (
	runtimeMu     sync.RWMutex
	runtimeConfig = RuntimeConfig{
		DataDir:    "data",
		SessionTTL: 4 * time.Hour,
	}
)

// ConfigureRuntime ...
func ConfigureRuntime(cfg RuntimeConfig) {
	if cfg.DataDir == "" {
		cfg.DataDir = "data"
	}
	if cfg.SessionTTL <= 0 {
		cfg.SessionTTL = 4 * time.Hour
	}
	_ = os.MkdirAll(cfg.DataDir, 0755)

	runtimeMu.Lock()
	runtimeConfig = cfg
	runtimeMu.Unlock()

	releases.ConfigureService(cfg.DataDir)
	initAnnouncement()
	auth.Init(cfg.DataDir)
	auth.ConfigureClients()
}

func currentRuntimeConfig() RuntimeConfig {
	runtimeMu.RLock()
	cfg := runtimeConfig
	runtimeMu.RUnlock()
	return cfg
}

func currentSessionTTL() time.Duration {
	return currentRuntimeConfig().SessionTTL
}

func dataDir() string {
	return currentRuntimeConfig().DataDir
}

func dataPath(parts ...string) string {
	all := append([]string{dataDir()}, parts...)
	return filepath.Join(all...)
}
