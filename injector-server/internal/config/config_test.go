package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("AGENT_PORT", "")
	t.Setenv("DATA_DIR", "")
	t.Setenv("SESSION_TTL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.AdminAddr != ":48901" {
		t.Fatalf("AdminAddr = %q, want :48901", cfg.AdminAddr)
	}
	if cfg.AgentAddr != ":38472" {
		t.Fatalf("AgentAddr = %q, want :38472", cfg.AgentAddr)
	}
	if cfg.DataDir != "data" {
		t.Fatalf("DataDir = %q, want data", cfg.DataDir)
	}
	if cfg.SessionTTL != 4*time.Hour {
		t.Fatalf("SessionTTL = %s, want 4h", cfg.SessionTTL)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("PORT", "19001")
	t.Setenv("AGENT_PORT", "19002")
	t.Setenv("DATA_DIR", "custom-data")
	t.Setenv("SESSION_TTL", "30m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.AdminAddr != ":19001" {
		t.Fatalf("AdminAddr = %q, want :19001", cfg.AdminAddr)
	}
	if cfg.AgentAddr != ":19002" {
		t.Fatalf("AgentAddr = %q, want :19002", cfg.AgentAddr)
	}
	if cfg.DataDir != "custom-data" {
		t.Fatalf("DataDir = %q, want custom-data", cfg.DataDir)
	}
	if cfg.SessionTTL != 30*time.Minute {
		t.Fatalf("SessionTTL = %s, want 30m", cfg.SessionTTL)
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	t.Setenv("SESSION_TTL", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("Load should reject invalid SESSION_TTL")
	}
}
