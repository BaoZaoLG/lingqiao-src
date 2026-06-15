package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	releasesvc "github.com/lingqiao/server/internal/releases"
)

type releaseService struct {
	store   *releasesvc.SQLiteStore
	signer  *releasesvc.ManifestSigner
	dataDir string
}

var (
	releaseSvcMu   sync.RWMutex
	releaseSvc     *releaseService
	releaseDataDir string
)

func configureReleaseService(dir string) {
	releaseSvcMu.Lock()
	old := releaseSvc
	releaseSvc = nil
	releaseDataDir = dir
	releaseSvcMu.Unlock()
	if old != nil && old.store != nil {
		_ = old.store.Close()
	}
}

func newReleaseService(dir string, seed []byte) (*releaseService, error) {
	store, err := releasesvc.OpenSQLiteStore(filepath.Join(dir, "releases.db"))
	if err != nil {
		return nil, err
	}
	if seed == nil {
		seed, err = loadOrCreateManifestSeed(dir)
		if err != nil {
			_ = store.Close()
			return nil, err
		}
	}
	signer, err := releasesvc.NewManifestSigner(seed)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	return &releaseService{store: store, signer: signer, dataDir: dir}, nil
}

func currentReleaseService() *releaseService {
	releaseSvcMu.RLock()
	svc := releaseSvc
	dir := releaseDataDir
	releaseSvcMu.RUnlock()
	if svc != nil || dir == "" {
		return svc
	}

	releaseSvcMu.Lock()
	defer releaseSvcMu.Unlock()
	if releaseSvc != nil {
		return releaseSvc
	}
	created, err := newReleaseService(dir, nil)
	if err != nil {
		log.Printf("[RELEASE] Failed to initialize release service: %v", err)
		return nil
	}
	if err := migrateLegacyUpdatePackages(context.Background(), created); err != nil {
		log.Printf("[RELEASE] Legacy update migration skipped: %v", err)
	}
	releaseSvc = created
	return releaseSvc
}

func loadOrCreateManifestSeed(dir string) ([]byte, error) {
	if value := os.Getenv("UPDATE_SIGNING_SEED_HEX"); value != "" {
		seed, err := hex.DecodeString(value)
		if err != nil {
			return nil, fmt.Errorf("invalid UPDATE_SIGNING_SEED_HEX: %w", err)
		}
		return seed, nil
	}
	path := filepath.Join(dir, "update_manifest_seed.key")
	if data, err := os.ReadFile(path); err == nil {
		seed, err := hex.DecodeString(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, fmt.Errorf("invalid manifest seed file: %w", err)
		}
		return seed, nil
	}
	seed := make([]byte, releasesvc.ManifestSeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(hex.EncodeToString(seed)), 0600); err != nil {
		return nil, err
	}
	return seed, nil
}

func migrateLegacyUpdatePackages(ctx context.Context, svc *releaseService) error {
	existing, err := svc.store.ListReleases(ctx)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	packages, _, err := ListUpdatePackages()
	if err != nil {
		return err
	}
	for _, pkg := range packages {
		releaseID := legacyReleaseID(pkg.Version)
		status := releasesvc.StatusDraft
		var publishedAt *time.Time
		if pkg.Active {
			status = releasesvc.StatusPublished
			t := pkg.UploadedAt
			if t.IsZero() {
				t = time.Now().UTC()
			}
			publishedAt = &t
		}
		createdAt := pkg.UploadedAt
		if createdAt.IsZero() {
			createdAt = time.Now().UTC()
		}
		if err := svc.store.SaveRelease(ctx, releasesvc.Release{
			ID:             releaseID,
			Version:        pkg.Version,
			Channel:        releasesvc.ChannelStable,
			Status:         status,
			RolloutPercent: 100,
			Notes:          "Migrated from legacy update package index",
			CreatedAt:      createdAt,
			PublishedAt:    publishedAt,
		}); err != nil {
			return err
		}
		if err := svc.store.SavePackage(ctx, releasesvc.ReleasePackage{
			ID:        legacyPackageID(pkg.Version),
			ReleaseID: releaseID,
			Kind:      releasesvc.PackageKindBundle,
			Filename:  pkg.Filename,
			Path:      filepath.Join("updates", pkg.Filename),
			FileSize:  pkg.FileSize,
			SHA256:    pkg.SHA256,
			CreatedAt: createdAt,
		}); err != nil {
			return err
		}
	}
	return nil
}

func legacyReleaseID(version string) string {
	return "legacy-" + strings.ReplaceAll(version, ".", "-")
}

func legacyPackageID(version string) string {
	return legacyReleaseID(version) + "-bundle"
}
