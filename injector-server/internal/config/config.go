package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	AdminAddr  string
	AgentAddr  string
	DataDir    string
	SessionTTL time.Duration
}

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
