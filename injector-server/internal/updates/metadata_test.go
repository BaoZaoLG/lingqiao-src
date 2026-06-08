package updates

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lingqiao/server/internal/storage"
)

func TestValidateVersion(t *testing.T) {
	if err := ValidateVersion("1.2.3"); err != nil {
		t.Fatalf("ValidateVersion returned error: %v", err)
	}
	if err := ValidateVersion("1.2-beta"); err == nil {
		t.Fatal("ValidateVersion should reject non numeric version")
	}
	if err := ValidateVersion(""); err == nil {
		t.Fatal("ValidateVersion should reject empty version")
	}
}

func TestSafeFilename(t *testing.T) {
	name, err := SafeFilename("2.1.0", ".exe")
	if err != nil {
		t.Fatalf("SafeFilename returned error: %v", err)
	}
	if name != "Injector_v2.1.0.exe" {
		t.Fatalf("name = %q, want Injector_v2.1.0.exe", name)
	}
}

func TestMetadataStoreSaveLoadAndSHAForVersion(t *testing.T) {
	store := NewMetadataStore(storage.NewJSONStore(t.TempDir()))
	info := UpdateInfo{
		Version:    "2.0.0",
		Filename:   "Injector_v2.0.0.exe",
		FileSize:   123,
		SHA256:     "abc123",
		UploadedAt: time.Now(),
	}

	if err := store.Save(info); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if loaded == nil {
		t.Fatal("Load returned nil")
	}
	if loaded.Version != "2.0.0" {
		t.Fatalf("Version = %q, want 2.0.0", loaded.Version)
	}
	if sha := store.SHAForVersion("2.0.0"); sha != "abc123" {
		t.Fatalf("SHAForVersion = %q, want abc123", sha)
	}
	if sha := store.SHAForVersion("3.0.0"); sha != "" {
		t.Fatalf("SHAForVersion mismatch returned %q, want empty", sha)
	}
}

func TestMetadataStoreUsesCompatibleInfoFilename(t *testing.T) {
	dir := t.TempDir()
	store := NewMetadataStore(storage.NewJSONStore(dir))

	if err := store.Save(UpdateInfo{Version: "2.0.0", Filename: "Injector_v2.0.0.exe"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "info.json")); err != nil {
		t.Fatalf("expected compatible info.json file: %v", err)
	}
}
