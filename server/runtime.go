package main

import (
	"os"
	"path/filepath"
	"sync"
	"time"
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

	configureInviteService(cfg.DataDir)
	configureUpdateStore(cfg.DataDir)
	configureReleaseService(cfg.DataDir)
	initAnnouncement()
	configureAuthClients()
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
