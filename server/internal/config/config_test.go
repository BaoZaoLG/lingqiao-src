package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// clear relevant env
	for _, k := range []string{"PORT", "AGENT_PORT", "DATA_DIR", "SESSION_TTL", "JAVA_CUTOVER_ENABLED", "JAVA_SIDECAR_BASE_URL"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AdminAddr != ":48901" || cfg.AgentAddr != ":38472" {
		t.Fatalf("addrs = %q %q", cfg.AdminAddr, cfg.AgentAddr)
	}
	if cfg.DataDir != "data" {
		t.Fatalf("DataDir = %q", cfg.DataDir)
	}
	if cfg.SessionTTL != 4*time.Hour {
		t.Fatalf("SessionTTL = %v", cfg.SessionTTL)
	}
	if cfg.Cutover.Enabled || cfg.Cutover.BaseURL != "http://127.0.0.1:49001" {
		t.Fatalf("Cutover defaults = %+v", cfg.Cutover)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("PORT", "12345")
	t.Setenv("AGENT_PORT", "23456")
	t.Setenv("DATA_DIR", "tmpdata")
	t.Setenv("SESSION_TTL", "90m")
	t.Setenv("JAVA_CUTOVER_ENABLED", "true")
	t.Setenv("JAVA_CUTOVER_ADMIN", "true")
	t.Setenv("JAVA_CUTOVER_ADMIN_WRITE", "true")
	t.Setenv("JAVA_CUTOVER_EXAM", "true")
	t.Setenv("JAVA_SIDECAR_BASE_URL", "http://127.0.0.1:59001")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AdminAddr != ":12345" || cfg.AgentAddr != ":23456" || cfg.DataDir != "tmpdata" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if cfg.SessionTTL != 90*time.Minute {
		t.Fatalf("ttl = %v", cfg.SessionTTL)
	}
	if !cfg.Cutover.Enabled || !cfg.Cutover.Admin || !cfg.Cutover.AdminWrite ||
		!cfg.Cutover.Exam || cfg.Cutover.BaseURL != "http://127.0.0.1:59001" {
		t.Fatalf("Cutover overrides = %+v", cfg.Cutover)
	}
}

func TestLoadInvalidTTL(t *testing.T) {
	t.Setenv("SESSION_TTL", "not-a-duration")
	_, err := Load()
	if err == nil {
		t.Fatal("expected invalid duration error")
	}
}

func TestStringAndDurationEnv(t *testing.T) {
	if stringEnv("NO_SUCH_ENV_XYZ", "fb") != "fb" {
		t.Fatal("fallback")
	}
	t.Setenv("NO_SUCH_ENV_XYZ", "set")
	if stringEnv("NO_SUCH_ENV_XYZ", "fb") != "set" {
		t.Fatal("override")
	}
	d, err := durationEnv("NO_DUR", time.Second)
	if err != nil || d != time.Second {
		t.Fatalf("default duration: %v %v", d, err)
	}
}
