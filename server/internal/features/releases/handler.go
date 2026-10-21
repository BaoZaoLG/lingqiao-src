package releases

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lingqiao/server/internal/audit"
	"github.com/lingqiao/server/internal/auth"
	"github.com/lingqiao/server/internal/features/sessions"
	"github.com/lingqiao/server/internal/httpapi"
	releasesvc "github.com/lingqiao/server/internal/releases"
)

type Handler struct {
	svc           *releaseService
	sessionsSvc   *sessions.Service
	auditRecorder *audit.Recorder
}

func NewHandler(svc *releaseService, sessionsSvc *sessions.Service, auditRecorder *audit.Recorder) *Handler {
	return &Handler{
		svc:           svc,
		sessionsSvc:   sessionsSvc,
		auditRecorder: auditRecorder,
	}
}

type updateCheckRequest struct {
	ClientVersion string             `json:"client_version"`
	Channel       releasesvc.Channel `json:"channel"`
	MachineID     string             `json:"machine_id"`
	Card          string             `json:"card"`
	CardCode      string             `json:"card_code"`
	SessionToken  string             `json:"session_token"`
	AgentID       string             `json:"agent_id"`
}

type updateEventRequest struct {
	releasesvc.ReleaseEvent
	SessionToken string `json:"session_token"`
}

func (h *Handler) HandleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	body, err := httpapi.ReadBody(r)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := auth.VerifyRequest(r, string(body)); err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var req updateCheckRequest
	if err := json.Unmarshal(body, &req); err != nil {
		httpapi.WriteProblem(w, r, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON", "请求体不是有效 JSON")
		return
	}
	if req.Channel == "" {
		req.Channel = releasesvc.ChannelStable
	}
	if req.CardCode == "" {
		req.CardCode = req.Card
	}
	if _, err := h.sessionsSvc.ValidateBound(r.Context(), req.SessionToken, req.MachineID, req.CardCode); err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	svc := h.svc
	if svc == nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "release service unavailable")
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
		httpapi.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if selected == nil {
		httpapi.WriteOK(w, map[string]interface{}{"update_available": false})
		return
	}
	manifest := manifestFromSelection(*selected)
	signed, err := svc.signer.Sign(manifest)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "manifest signing failed")
		return
	}
	manifestPayload, err := json.Marshal(signed.Manifest)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "manifest encoding failed")
		return
	}
	manifestHMAC := ""
	derivedKey, err := auth.GetDerivedKey(r.Header.Get("X-Client-ID"))
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "client key unavailable")
		return
	}
	manifestHMAC = auth.SignHMAC(string(derivedKey), string(manifestPayload))
	_ = svc.store.RecordEvent(r.Context(), releasesvc.ReleaseEvent{
		ReleaseID: selected.Release.ID,
		Version:   selected.Release.Version,
		MachineID: req.MachineID,
		CardCode:  req.CardCode,
		AgentID:   req.AgentID,
		Type:      releasesvc.EventOffered,
		CreatedAt: time.Now().UTC(),
	})
	httpapi.WriteOK(w, map[string]interface{}{
		"update_available": true,
		"manifest":         signed.Manifest,
		"manifest_payload": base64.StdEncoding.EncodeToString(manifestPayload),
		"manifest_hmac":    manifestHMAC,
		"signature":        signed.Signature,
		"public_key":       svc.signer.PublicKeyHex(),
	})
}

func (h *Handler) CheckVersion(w http.ResponseWriter, req sessions.ClientRequest) (bool, map[string]interface{}) {
	if req.ClientVersion == "" || h == nil || h.svc == nil {
		return false, nil
	}
	selected, err := releasesvc.NewSelector(h.svc.store).Select(context.Background(), releasesvc.ClientContext{
		Version:   req.ClientVersion,
		Channel:   releasesvc.ChannelStable,
		MachineID: req.MachineID,
		CardCode:  req.Card,
		AgentID:   "",
	})
	if err != nil || selected == nil {
		return false, nil
	}

	manifest := manifestFromSelection(*selected)
	info := map[string]interface{}{
		"update_available": true,
		"latest_version":   selected.Release.Version,
		"force_update":     selected.Release.ForceUpdate,
		"download_url":     manifest.PackageURL,
		"sha256":           manifest.PackageSHA256,
	}

	if selected.Release.MinVersion != "" && versionOlder(req.ClientVersion, selected.Release.MinVersion) {
		httpapi.WriteJSON(w, http.StatusForbidden, map[string]interface{}{
			"status":       "error",
			"message":      fmt.Sprintf("版本过低，请更新到 v%s 或更高版本", selected.Release.MinVersion),
			"download_url": manifest.PackageURL,
			"sha256":       manifest.PackageSHA256,
		})
		return true, nil
	}
	return false, info
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
		ScriptID:       selected.Release.ScriptID,
		ScriptVersion:  selected.Release.ScriptVersion,
		ScriptSHA256:   selected.Release.ScriptSHA256,
		RolloutPercent: selected.Release.RolloutPercent,
		CreatedAt:      selected.Release.CreatedAt,
	}
}

func versionOlder(client, latest string) bool {
	parse := func(s string) []int {
		parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
		nums := make([]int, len(parts))
		for i, p := range parts {
			n := 0
			for _, c := range p {
				if c < '0' || c > '9' {
					break
				}
				n = n*10 + int(c-'0')
			}
			nums[i] = n
		}
		return nums
	}
	cv := parse(client)
	lv := parse(latest)
	maxLen := len(cv)
	if len(lv) > maxLen {
		maxLen = len(lv)
	}
	for i := 0; i < maxLen; i++ {
		a, b := 0, 0
		if i < len(cv) {
			a = cv[i]
		}
		if i < len(lv) {
			b = lv[i]
		}
		if a < b {
			return true
		}
		if a > b {
			return false
		}
	}
	return false
}

func (h *Handler) HandleUpdateEvent(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	body, err := httpapi.ReadBody(r)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := auth.VerifyRequest(r, string(body)); err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	var req updateEventRequest
	if err := json.Unmarshal(body, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	event := req.ReleaseEvent
	// Validate required fields — reject empty/malformed events
	if event.ReleaseID == "" || event.Type == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "release_id and type are required")
		return
	}
	token := firstNonEmptyString(r.Header.Get("X-Session-Token"), req.SessionToken)
	machineID := firstNonEmptyString(r.Header.Get("X-Machine-ID"), event.MachineID)
	cardCode := firstNonEmptyString(r.Header.Get("X-Card-Code"), event.CardCode)
	if h.sessionsSvc == nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "session service unavailable")
		return
	}
	session, err := h.sessionsSvc.ValidateBound(r.Context(), token, machineID, cardCode)
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}
	event.MachineID = session.MachineID
	event.CardCode = session.CardCode
	// Sanitize: cap detail length to prevent abuse
	if len(event.Detail) > 500 {
		event.Detail = event.Detail[:500]
	}
	svc := h.svc
	if svc == nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "release service unavailable")
		return
	}
	if err := svc.store.RecordEvent(r.Context(), event); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpapi.WriteOK(w, nil)
}

func (h *Handler) HandleUpdatePackageDownload(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	if err := auth.VerifyRequest(r, r.URL.Path); err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if _, err := h.sessionsSvc.ValidateBound(r.Context(), r.Header.Get("X-Session-Token"), r.Header.Get("X-Machine-ID"), r.Header.Get("X-Card-Code")); err != nil {
		status := http.StatusUnauthorized
		if r.Header.Get("X-Session-Token") == "" {
			status = http.StatusBadRequest
		}
		httpapi.WriteError(w, status, err.Error())
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/update/package/")
	if id == "" || strings.Contains(id, "/") || strings.Contains(id, "..") {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid package id")
		return
	}
	svc := h.svc
	if svc == nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "release service unavailable")
		return
	}
	pkg, err := svc.store.GetPackage(r.Context(), id)
	if err != nil {
		httpapi.WriteError(w, http.StatusNotFound, "package not found")
		return
	}
	path, err := svc.resolvePackagePath(pkg)
	if err != nil {
		httpapi.WriteError(w, http.StatusForbidden, err.Error())
		return
	}
	file, err := os.Open(path)
	if err != nil {
		httpapi.WriteError(w, http.StatusNotFound, "package file not found")
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "package stat failed")
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

func (h *Handler) HandleReleases(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet, http.MethodPost:
	default:
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	svc := h.svc
	if svc == nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "release service unavailable")
		return
	}
	if r.Method == http.MethodGet {
		h.writeReleaseList(w, r, svc)
		return
	}
	h.createRelease(w, r, svc)
}

func (h *Handler) writeReleaseList(w http.ResponseWriter, r *http.Request, svc *releaseService) {
	releases, err := svc.store.ListReleases(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to list releases")
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
	httpapi.WriteOK(w, map[string]interface{}{"releases": items, "public_key": svc.signer.PublicKeyHex()})
}

func (h *Handler) createRelease(w http.ResponseWriter, r *http.Request, svc *releaseService) {
	var req struct {
		Version        string                    `json:"version"`
		Channel        releasesvc.Channel        `json:"channel"`
		MinVersion     string                    `json:"min_version"`
		ForceUpdate    bool                      `json:"force_update"`
		RolloutPercent int                       `json:"rollout_percent"`
		Notes          string                    `json:"notes"`
		Targeting      releasesvc.TargetingRules `json:"targeting"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Version == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "version is required")
		return
	}
	if req.Channel == "" {
		req.Channel = releasesvc.ChannelStable
	}
	if req.RolloutPercent == 0 {
		req.RolloutPercent = 100
	}

	var scriptID, scriptVersion, scriptSHA256 string
	id, version, sha256, err := h.getActiveScript()
	if err != nil || id == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "active script is required before creating release")
		return
	}
	scriptID = id
	scriptVersion = version
	scriptSHA256 = sha256

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
		ScriptID:       scriptID,
		ScriptVersion:  scriptVersion,
		ScriptSHA256:   scriptSHA256,
		CreatedAt:      time.Now().UTC(),
	}
	if err := svc.store.SaveRelease(r.Context(), release); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to create release")
		return
	}
	if err := h.recordReleaseAudit("release_created", release, ""); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "action succeeded but audit log failed")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"release": release})
}

func newReleaseID(version string, channel releasesvc.Channel) string {
	clean := strings.NewReplacer(".", "-", " ", "-", "/", "-", "\\", "-").Replace(version)
	return fmt.Sprintf("rel-%s-%s-%d", channel, clean, time.Now().UnixNano())
}

func (h *Handler) HandleReleaseRoute(w http.ResponseWriter, r *http.Request) {
	svc := h.svc
	if svc == nil {
		httpapi.WriteError(w, http.StatusServiceUnavailable, "release service unavailable")
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/admin/api/releases/")
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) != 2 || parts[0] == "" {
		httpapi.WriteError(w, http.StatusNotFound, "release route not found")
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
		httpapi.WriteError(w, http.StatusNotFound, "release action not found")
	}
}

func (h *Handler) updateReleaseStatus(w http.ResponseWriter, r *http.Request, svc *releaseService, releaseID string, status releasesvc.ReleaseStatus) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	release, err := svc.store.GetRelease(r.Context(), releaseID)
	if err != nil {
		httpapi.WriteError(w, http.StatusNotFound, "release not found")
		return
	}
	now := time.Now().UTC()
	release.Status = status
	switch status {
	case releasesvc.StatusPublished:
		id, version, sha256, err := h.getActiveScript()
		if err != nil || id == "" {
			httpapi.WriteError(w, http.StatusBadRequest, "Cannot publish release: no active script available")
			return
		}
		if err := svc.ensurePublishablePackage(r.Context(), release.ID); err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		release.ScriptID = id
		release.ScriptVersion = version
		release.ScriptSHA256 = sha256
		release.PublishedAt = &now
		release.PausedAt = nil
	case releasesvc.StatusPaused:
		release.PausedAt = &now
	}
	if err := svc.store.SaveRelease(r.Context(), release); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to update release")
		return
	}
	action := "release_status_changed"
	if status == releasesvc.StatusPublished {
		action = "release_published"
	}
	if status == releasesvc.StatusPaused {
		action = "release_paused"
	}
	if err := h.recordReleaseAudit(action, release, fmt.Sprintf("status=%s", status)); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "action succeeded but audit log failed")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"release": release})
}

func (h *Handler) rollbackRelease(w http.ResponseWriter, r *http.Request, svc *releaseService, releaseID string) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	var req struct {
		TargetReleaseID string `json:"target_release_id"`
	}
	if r.Body != nil {
		_ = httpapi.ReadJSON(r, &req)
	}
	release, err := svc.store.GetRelease(r.Context(), releaseID)
	if err != nil {
		httpapi.WriteError(w, http.StatusNotFound, "release not found")
		return
	}
	now := time.Now().UTC()
	release.Status = releasesvc.StatusRolledBack
	release.RolledBackAt = &now
	release.RolledBackTo = req.TargetReleaseID
	if err := svc.store.SaveRelease(r.Context(), release); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to rollback release")
		return
	}
	if err := h.recordReleaseAudit("release_rolled_back", release, fmt.Sprintf("target_release_id=%s", req.TargetReleaseID)); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "action succeeded but audit log failed")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"release": release})
}

func (h *Handler) releaseEvents(w http.ResponseWriter, r *http.Request, svc *releaseService, releaseID string) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	events, err := svc.store.ListEvents(r.Context(), releaseID, 100)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to list release events")
		return
	}
	metrics, err := svc.store.ReleaseMetrics(r.Context(), releaseID)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to load release metrics")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"events": events, "metrics": metrics})
}

func (h *Handler) uploadReleasePackage(w http.ResponseWriter, r *http.Request, svc *releaseService, releaseID string) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	release, err := svc.store.GetRelease(r.Context(), releaseID)
	if err != nil {
		httpapi.WriteError(w, http.StatusNotFound, "release not found")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 501<<20)
	if err := r.ParseMultipartForm(500 << 20); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid package upload")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()
	kind := releasesvc.PackageKind(r.FormValue("kind"))
	if kind == "" {
		kind = releasesvc.PackageKindBundle
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".exe" && ext != ".msi" {
		httpapi.WriteError(w, http.StatusBadRequest, "only .exe or .msi packages are supported")
		return
	}
	filename := sanitizePackageFilename(header.Filename, release.Version)
	relDir := filepath.Join("updates", release.ID)
	absDir := filepath.Join(svc.dataDir, relDir)
	if err := os.MkdirAll(absDir, 0755); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to create package directory")
		return
	}
	relPath := filepath.Join(relDir, filename)
	absPath := filepath.Join(svc.dataDir, relPath)
	dst, err := os.Create(absPath)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to create package file")
		return
	}
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(dst, hash), file)
	closeErr := dst.Close()
	if err != nil || closeErr != nil {
		_ = os.Remove(absPath)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to save package")
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
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to save package metadata")
		return
	}
	if err := h.recordReleaseAudit("release_package_uploaded", release, fmt.Sprintf("package_id=%s kind=%s sha256=%s size=%d", pkg.ID, pkg.Kind, pkg.SHA256, pkg.FileSize)); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "action succeeded but audit log failed")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"package": pkg})
}

func (h *Handler) recordReleaseAudit(action string, release releasesvc.Release, extra string) error {
	if h == nil || h.auditRecorder == nil {
		return nil
	}
	detail := fmt.Sprintf("id=%s version=%s channel=%s", release.ID, release.Version, release.Channel)
	if extra != "" {
		detail += " " + extra
	}
	return h.auditRecorder.Append(audit.Event{
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

func (h *Handler) getActiveScript() (string, string, string, error) {
	scriptDir := filepath.Join(h.svc.dataDir, "scripts")
	var index struct {
		ActiveID string `json:"active_id"`
		Scripts  []struct {
			ID      string `json:"id"`
			Version string `json:"version"`
			SHA256  string `json:"sha256"`
		} `json:"scripts"`
	}
	if data, err := os.ReadFile(filepath.Join(scriptDir, "index.json")); err == nil {
		if err := json.Unmarshal(data, &index); err == nil && index.ActiveID != "" {
			for _, script := range index.Scripts {
				if script.ID == index.ActiveID {
					return script.ID, script.Version, script.SHA256, nil
				}
			}
			modulePath := filepath.Join(scriptDir, index.ActiveID+".json")
			if data, err := os.ReadFile(modulePath); err == nil {
				var script struct {
					ID      string `json:"id"`
					Version string `json:"version"`
					SHA256  string `json:"sha256"`
				}
				if err := json.Unmarshal(data, &script); err == nil && script.ID != "" {
					return script.ID, script.Version, script.SHA256, nil
				}
			}
		}
	}

	data, err := os.ReadFile(filepath.Join(scriptDir, "active.json"))
	if err != nil {
		return "", "", "", err
	}
	var script struct {
		ID      string `json:"id"`
		Version string `json:"version"`
		SHA256  string `json:"sha256"`
	}
	if err := json.Unmarshal(data, &script); err != nil {
		return "", "", "", err
	}
	return script.ID, script.Version, script.SHA256, nil
}

func (s *releaseService) ensurePublishablePackage(ctx context.Context, releaseID string) error {
	packages, err := s.store.ListPackages(ctx, releaseID)
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		return fmt.Errorf("Cannot publish release: no update package uploaded")
	}
	for _, pkg := range packages {
		path, err := s.resolvePackagePath(pkg)
		if err != nil {
			return err
		}
		if stat, err := os.Stat(path); err == nil && !stat.IsDir() {
			return nil
		}
	}
	return fmt.Errorf("Cannot publish release: package file not found")
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
