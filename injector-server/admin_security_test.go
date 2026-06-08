package main

import (
	"bytes"
	"log"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewAdminHandlerLoadsPersistedBcryptPassword(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	cm := NewCardManager(NewJSONStorage(filepath.Join(dir, "store")))

	t.Setenv("ADMIN_PASSWORD", "initial-password")
	first := NewAdminHandler(cm)
	if ok, _ := verifyPassword("initial-password", first.adminPassHash); !ok {
		t.Fatal("initial password did not verify")
	}

	t.Setenv("ADMIN_PASSWORD", "replacement-password")
	second := NewAdminHandler(cm)

	if second.adminPassHash != first.adminPassHash {
		t.Fatal("persisted bcrypt hash was not loaded on restart")
	}
	if ok, _ := verifyPassword("initial-password", second.adminPassHash); !ok {
		t.Fatal("persisted password no longer verifies")
	}
	if ok, _ := verifyPassword("replacement-password", second.adminPassHash); ok {
		t.Fatal("restart should not replace persisted admin password hash")
	}
}

func TestNewAdminHandlerDoesNotGeneratePasswordWhenPersistedHashExists(t *testing.T) {
	dir := t.TempDir()
	restore := configureRuntimeForTest(t, RuntimeConfig{DataDir: dir})
	defer restore()

	cm := NewCardManager(NewJSONStorage(filepath.Join(t.TempDir(), "store")))
	t.Setenv("ADMIN_PASSWORD", "initial-password")
	first := NewAdminHandler(cm)
	if ok, _ := verifyPassword("initial-password", first.adminPassHash); !ok {
		t.Fatal("initial password did not verify")
	}

	t.Setenv("ADMIN_PASSWORD", "")
	var logs bytes.Buffer
	previousLogOutput := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(previousLogOutput)

	second := NewAdminHandler(cm)

	if second.adminPassHash != first.adminPassHash {
		t.Fatal("persisted admin password hash was not reused")
	}
	if strings.Contains(logs.String(), "Generated random password") {
		t.Fatalf("handler generated a startup password even though a persisted hash exists: %s", logs.String())
	}
}
