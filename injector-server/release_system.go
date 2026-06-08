package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
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

type updateCheckRequest struct {
	ClientVersion string             `json:"client_version"`
	Channel       releasesvc.Channel `json:"channel"`
	MachineID     string             `json:"machine_id"`
	Card          string             `json:"card"`
	CardCode      string             `json:"card_code"`
	AgentID       string             `json:"agent_id"`
}

func (h *APIHandler) HandleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	body, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := verifyRequestHMAC(r, string(body)); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var req updateCheckRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Channel == "" {
		req.Channel = releasesvc.ChannelStable
	}
	if req.CardCode == "" {
		req.CardCode = req.Card
	}
	svc := currentReleaseService()
	if svc == nil {
		writeError(w, http.StatusServiceUnavailable, "release service unavailable")
		return
	}
	selected, err := releasesvc.NewSelector(svc.store).Select(r.Context(), releasesvc.ClientContext{
		Version:   req.ClientVersion,
		Channel:   req.Channel,
		MachineID: req.MachineID,
		CardCode:  req.CardCode,
		AgentID:   req.AgentID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if selected == nil {
		writeOK(w, map[string]interface{}{"update_available": false})
		return
	}
	manifest := manifestFromSelection(*selected)
	signed, err := svc.signer.Sign(manifest)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "manifest signing failed")
		return
	}
	manifestPayload, err := json.Marshal(signed.Manifest)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "manifest encoding failed")
		return
	}
	manifestHMAC := ""
	if derivedKey, err := getDerivedKey(r.Header.Get("X-Client-ID")); err == nil {
		manifestHMAC = SignHMAC(string(derivedKey), string(manifestPayload))
	}
	_ = svc.store.RecordEvent(r.Context(), releasesvc.ReleaseEvent{
		ReleaseID: selected.Release.ID,
		Version:   selected.Release.Version,
		MachineID: req.MachineID,
		CardCode:  req.CardCode,
		AgentID:   req.AgentID,
		Type:      releasesvc.EventOffered,
		CreatedAt: time.Now().UTC(),
	})
	writeOK(w, map[string]interface{}{
		"update_available": true,
		"manifest":         signed.Manifest,
		"manifest_payload": base64.StdEncoding.EncodeToString(manifestPayload),
		"manifest_hmac":    manifestHMAC,
		"signature":        signed.Signature,
		"public_key":       svc.signer.PublicKeyHex(),
	})
}

func manifestFromSelection(selected releasesvc.SelectedRelease) releasesvc.Manifest {
	return releasesvc.Manifest{
		ReleaseID:      selected.Release.ID,
		Version:        selected.Release.Version,
		Channel:        selected.Release.Channel,
		MinVersion:     selected.Release.MinVersion,
		ForceUpdate:    selected.Release.ForceUpdate,
		ReleaseNotes:   selected.Release.Notes,
		PackageID:      selected.Package.ID,
		PackageKind:    string(selected.Package.Kind),
		PackageURL:     "/api/v1/update/package/" + selected.Package.ID,
		PackageSize:    selected.Package.FileSize,
		PackageSHA256:  selected.Package.SHA256,
		RolloutPercent: selected.Release.RolloutPercent,
		CreatedAt:      selected.Release.CreatedAt,
	}
}

func (h *APIHandler) HandleUpdateEvent(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	body, err := readBody(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := verifyRequestHMAC(r, string(body)); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var event releasesvc.ReleaseEvent
	if err := json.Unmarshal(body, &event); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	svc := currentReleaseService()
	if svc == nil {
		writeError(w, http.StatusServiceUnavailable, "release service unavailable")
		return
	}
	if err := svc.store.RecordEvent(r.Context(), event); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, nil)
}

func (h *APIHandler) HandleUpdatePackageDownload(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if err := verifyRequestHMAC(r, r.URL.Path); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/update/package/")
	if id == "" || strings.Contains(id, "/") || strings.Contains(id, "..") {
		writeError(w, http.StatusBadRequest, "invalid package id")
		return
	}
	svc := currentReleaseService()
	if svc == nil {
		writeError(w, http.StatusServiceUnavailable, "release service unavailable")
		return
	}
	pkg, err := svc.store.GetPackage(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "package not found")
		return
	}
	path, err := svc.resolvePackagePath(pkg)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	file, err := os.Open(path)
	if err != nil {
		writeError(w, http.StatusNotFound, "package file not found")
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "package stat failed")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", pkg.Filename))
	w.Header().Set("X-Update-Package-ID", pkg.ID)
	w.Header().Set("X-Update-SHA256", pkg.SHA256)
	http.ServeContent(w, r, pkg.Filename, stat.ModTime(), file)
}

func (s *releaseService) resolvePackagePath(pkg releasesvc.ReleasePackage) (string, error) {
	path := pkg.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(s.dataDir, path)
	}
	baseAbs, err := filepath.Abs(s.dataDir)
	if err != nil {
		return "", err
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	baseClean := strings.ToLower(filepath.Clean(baseAbs))
	pathClean := strings.ToLower(filepath.Clean(pathAbs))
	if pathClean != baseClean && !strings.HasPrefix(pathClean, baseClean+string(os.PathSeparator)) {
		return "", fmt.Errorf("package path escapes data directory")
	}
	return pathAbs, nil
}

func (h *AdminHandler) HandleReleases(w http.ResponseWriter, r *http.Request) {
	svc := currentReleaseService()
	if svc == nil {
		writeError(w, http.StatusServiceUnavailable, "release service unavailable")
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.writeReleaseList(w, r, svc)
	case http.MethodPost:
		h.createRelease(w, r, svc)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *AdminHandler) writeReleaseList(w http.ResponseWriter, r *http.Request, svc *releaseService) {
	releases, err := svc.store.ListReleases(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]map[string]interface{}, 0, len(releases))
	for _, release := range releases {
		packages, _ := svc.store.ListPackages(r.Context(), release.ID)
		metrics, _ := svc.store.ReleaseMetrics(r.Context(), release.ID)
		items = append(items, map[string]interface{}{
			"release":  release,
			"packages": packages,
			"metrics":  metrics,
		})
	}
	writeOK(w, map[string]interface{}{"releases": items, "public_key": svc.signer.PublicKeyHex()})
}

func (h *AdminHandler) createRelease(w http.ResponseWriter, r *http.Request, svc *releaseService) {
	var req struct {
		Version        string                    `json:"version"`
		Channel        releasesvc.Channel        `json:"channel"`
		MinVersion     string                    `json:"min_version"`
		ForceUpdate    bool                      `json:"force_update"`
		RolloutPercent int                       `json:"rollout_percent"`
		Notes          string                    `json:"notes"`
		Targeting      releasesvc.TargetingRules `json:"targeting"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Version == "" {
		writeError(w, http.StatusBadRequest, "version is required")
		return
	}
	if req.Channel == "" {
		req.Channel = releasesvc.ChannelStable
	}
	if req.RolloutPercent == 0 {
		req.RolloutPercent = 100
	}
	release := releasesvc.Release{
		ID:             newReleaseID(req.Version, req.Channel),
		Version:        req.Version,
		Channel:        req.Channel,
		Status:         releasesvc.StatusDraft,
		MinVersion:     req.MinVersion,
		ForceUpdate:    req.ForceUpdate,
		RolloutPercent: req.RolloutPercent,
		Notes:          req.Notes,
		Targeting:      req.Targeting,
		CreatedAt:      time.Now().UTC(),
	}
	if err := svc.store.SaveRelease(r.Context(), release); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.recordReleaseAudit("release_created", release, "")
	writeOK(w, map[string]interface{}{"release": release})
}

func newReleaseID(version string, channel releasesvc.Channel) string {
	clean := strings.NewReplacer(".", "-", " ", "-", "/", "-", "\\", "-").Replace(version)
	return fmt.Sprintf("rel-%s-%s-%d", channel, clean, time.Now().UnixNano())
}

func (h *AdminHandler) HandleReleaseRoute(w http.ResponseWriter, r *http.Request) {
	svc := currentReleaseService()
	if svc == nil {
		writeError(w, http.StatusServiceUnavailable, "release service unavailable")
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/admin/api/releases/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "release route not found")
		return
	}
	releaseID, action := parts[0], parts[1]
	switch action {
	case "publish":
		h.updateReleaseStatus(w, r, svc, releaseID, releasesvc.StatusPublished)
	case "pause":
		h.updateReleaseStatus(w, r, svc, releaseID, releasesvc.StatusPaused)
	case "rollback":
		h.rollbackRelease(w, r, svc, releaseID)
	case "events":
		h.releaseEvents(w, r, svc, releaseID)
	case "packages":
		h.uploadReleasePackage(w, r, svc, releaseID)
	default:
		writeError(w, http.StatusNotFound, "release action not found")
	}
}

func (h *AdminHandler) updateReleaseStatus(w http.ResponseWriter, r *http.Request, svc *releaseService, releaseID string, status releasesvc.ReleaseStatus) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	release, err := svc.store.GetRelease(r.Context(), releaseID)
	if err != nil {
		writeError(w, http.StatusNotFound, "release not found")
		return
	}
	now := time.Now().UTC()
	release.Status = status
	switch status {
	case releasesvc.StatusPublished:
		release.PublishedAt = &now
		release.PausedAt = nil
	case releasesvc.StatusPaused:
		release.PausedAt = &now
	}
	if err := svc.store.SaveRelease(r.Context(), release); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	action := "release_status_changed"
	if status == releasesvc.StatusPublished {
		action = "release_published"
	}
	if status == releasesvc.StatusPaused {
		action = "release_paused"
	}
	h.recordReleaseAudit(action, release, fmt.Sprintf("status=%s", status))
	writeOK(w, map[string]interface{}{"release": release})
}

func (h *AdminHandler) rollbackRelease(w http.ResponseWriter, r *http.Request, svc *releaseService, releaseID string) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req struct {
		TargetReleaseID string `json:"target_release_id"`
	}
	if r.Body != nil {
		_ = readJSON(r, &req)
	}
	release, err := svc.store.GetRelease(r.Context(), releaseID)
	if err != nil {
		writeError(w, http.StatusNotFound, "release not found")
		return
	}
	now := time.Now().UTC()
	release.Status = releasesvc.StatusRolledBack
	release.RolledBackAt = &now
	release.RolledBackTo = req.TargetReleaseID
	if err := svc.store.SaveRelease(r.Context(), release); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.recordReleaseAudit("release_rolled_back", release, fmt.Sprintf("target_release_id=%s", req.TargetReleaseID))
	writeOK(w, map[string]interface{}{"release": release})
}

func (h *AdminHandler) releaseEvents(w http.ResponseWriter, r *http.Request, svc *releaseService, releaseID string) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	events, err := svc.store.ListEvents(r.Context(), releaseID, 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	metrics, err := svc.store.ReleaseMetrics(r.Context(), releaseID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"events": events, "metrics": metrics})
}

func (h *AdminHandler) uploadReleasePackage(w http.ResponseWriter, r *http.Request, svc *releaseService, releaseID string) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	release, err := svc.store.GetRelease(r.Context(), releaseID)
	if err != nil {
		writeError(w, http.StatusNotFound, "release not found")
		return
	}
	if err := r.ParseMultipartForm(500 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid package upload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()
	kind := releasesvc.PackageKind(r.FormValue("kind"))
	if kind == "" {
		kind = releasesvc.PackageKindBundle
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".exe" && ext != ".msi" {
		writeError(w, http.StatusBadRequest, "only .exe or .msi packages are supported")
		return
	}
	filename := sanitizePackageFilename(header.Filename, release.Version)
	relDir := filepath.Join("updates", release.ID)
	absDir := filepath.Join(svc.dataDir, relDir)
	if err := os.MkdirAll(absDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create package directory")
		return
	}
	relPath := filepath.Join(relDir, filename)
	absPath := filepath.Join(svc.dataDir, relPath)
	dst, err := os.Create(absPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create package file")
		return
	}
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(dst, hash), file)
	closeErr := dst.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(absPath)
		writeError(w, http.StatusInternalServerError, "failed to save package")
		return
	}
	pkg := releasesvc.ReleasePackage{
		ID:        fmt.Sprintf("%s-%s", releaseID, kind),
		ReleaseID: releaseID,
		Kind:      kind,
		Filename:  filename,
		Path:      relPath,
		FileSize:  written,
		SHA256:    hex.EncodeToString(hash.Sum(nil)),
		CreatedAt: time.Now().UTC(),
	}
	if err := svc.store.SavePackage(r.Context(), pkg); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.recordReleaseAudit("release_package_uploaded", release, fmt.Sprintf("package_id=%s kind=%s sha256=%s size=%d", pkg.ID, pkg.Kind, pkg.SHA256, pkg.FileSize))
	writeOK(w, map[string]interface{}{"package": pkg})
}

func (h *AdminHandler) recordReleaseAudit(action string, release releasesvc.Release, extra string) {
	if h == nil || h.cm == nil {
		return
	}
	detail := fmt.Sprintf("id=%s version=%s channel=%s", release.ID, release.Version, release.Channel)
	if extra != "" {
		detail += " " + extra
	}
	h.cm.RecordAudit(AuditEntry{
		Action: action,
		Detail: detail,
	})
}

func sanitizePackageFilename(filename, version string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	base := "LingqiaoSetup-" + version
	base = strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(base)
	return base + ext
}
