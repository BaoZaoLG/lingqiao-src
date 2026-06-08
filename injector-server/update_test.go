package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveCurrentUpdatePersistsMetadata(t *testing.T) {
	restore := useUpdateStoreForTest(t.TempDir())
	defer restore()

	info := &UpdateInfo{
		Version:    "3.0.0",
		Filename:   "Injector_v3.0.0.exe",
		FileSize:   42,
		SHA256:     "deadbeef",
		UploadedAt: time.Now(),
	}

	if err := saveCurrentUpdate(info); err != nil {
		t.Fatalf("saveCurrentUpdate returned error: %v", err)
	}

	currentUpdate = nil
	loaded := getCurrentUpdate()
	if loaded == nil {
		t.Fatal("getCurrentUpdate returned nil")
	}
	if loaded.Version != "3.0.0" {
		t.Fatalf("Version = %q, want 3.0.0", loaded.Version)
	}
	if sha := getUpdateSHA256("3.0.0"); sha != "deadbeef" {
		t.Fatalf("getUpdateSHA256 = %q, want deadbeef", sha)
	}
}

func TestUpdateRepositoryCanSelectCurrentVersion(t *testing.T) {
	dir := t.TempDir()
	restore := useUpdateStoreForTest(filepath.Join(dir, "updates"))
	defer restore()
	t.Chdir(dir)
	if err := os.MkdirAll(filepath.Join("data", "updates"), 0755); err != nil {
		t.Fatal(err)
	}

	first := &UpdateInfo{Version: "2.1.14", Filename: "Injector_v2.1.14.exe", FileSize: 10, SHA256: "sha14", UploadedAt: time.Now()}
	second := &UpdateInfo{Version: "2.1.15", Filename: "Injector_v2.1.15.exe", FileSize: 11, SHA256: "sha15", UploadedAt: time.Now()}
	if err := AddUpdatePackage(first, false); err != nil {
		t.Fatal(err)
	}
	if err := AddUpdatePackage(second, false); err != nil {
		t.Fatal(err)
	}

	packages, activeVersion, err := ListUpdatePackages()
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 2 {
		t.Fatalf("package count = %d, want 2", len(packages))
	}
	if activeVersion != "2.1.14" {
		t.Fatalf("active version = %q, want 2.1.14", activeVersion)
	}

	if err := SetCurrentUpdateVersion("2.1.15"); err != nil {
		t.Fatal(err)
	}
	if current := getCurrentUpdate(); current == nil || current.Version != "2.1.15" {
		t.Fatalf("current update = %#v, want 2.1.15", current)
	}
}

func TestUpdateRepositoryDoesNotDeleteCurrentVersion(t *testing.T) {
	dir := t.TempDir()
	restore := useUpdateStoreForTest(filepath.Join(dir, "updates"))
	defer restore()
	t.Chdir(dir)
	if err := os.MkdirAll(filepath.Join("data", "updates"), 0755); err != nil {
		t.Fatal(err)
	}

	info := &UpdateInfo{Version: "2.1.15", Filename: "Injector_v2.1.15.exe", FileSize: 11, SHA256: "sha15", UploadedAt: time.Now()}
	if err := AddUpdatePackage(info, true); err != nil {
		t.Fatal(err)
	}
	if err := DeleteUpdatePackage("2.1.15"); err == nil {
		t.Fatalf("DeleteUpdatePackage(current) succeeded, want error")
	}
}
