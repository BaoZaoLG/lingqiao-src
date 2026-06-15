package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	releasesvc "github.com/lingqiao/server/internal/releases"
)

func TestHandleUpdateCheckReturnsSignedManifestAndRecordsOffered(t *testing.T) {
	dir := t.TempDir()
	restore := useReleaseServiceForTest(t, dir, bytes.Repeat([]byte{9}, releasesvc.ManifestSeedSize))
	defer restore()
	svc := currentReleaseService()

	publishedAt := time.Unix(500, 0).UTC()
	release := releasesvc.Release{
		ID:             "rel-api",
		Version:        "7.0.0",
		Channel:        releasesvc.ChannelStable,
		Status:         releasesvc.StatusPublished,
		ForceUpdate:    true,
		RolloutPercent: 100,
		Notes:          "signed installer release",
		CreatedAt:      publishedAt,
		PublishedAt:    &publishedAt,
	}
	if err := svc.store.SaveRelease(t.Context(), release); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "updates"), 0755); err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join("updates", "LingqiaoSetup-7.0.0.exe")
	if err := os.WriteFile(filepath.Join(dir, packagePath), []byte("installer"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := svc.store.SavePackage(t.Context(), releasesvc.ReleasePackage{
		ID:        "pkg-api",
		ReleaseID: release.ID,
		Kind:      releasesvc.PackageKindBundle,
		Filename:  "LingqiaoSetup-7.0.0.exe",
		Path:      packagePath,
		FileSize:  int64(len("installer")),
		SHA256:    "sha-api",
		CreatedAt: publishedAt,
	}); err != nil {
		t.Fatal(err)
	}

	cm := NewCardManager(NewJSONStorage(filepath.Join(dir, "cards")))
	addBoundTestSession(cm, "session-api", "card-api", "machine-api")

	body := []byte(`{"client_version":"6.0.0","channel":"stable","machine_id":"machine-api","card":"card-api","session_token":"session-api","agent_id":"agent-api"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/update/check", bytes.NewReader(body))
	signPayloadRequest(t, req, body)
	rr := httptest.NewRecorder()

	NewAPIHandler(cm).HandleUpdateCheck(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var got struct {
		UpdateAvailable bool                `json:"update_available"`
		Manifest        releasesvc.Manifest `json:"manifest"`
		ManifestPayload string              `json:"manifest_payload"`
		ManifestHMAC    string              `json:"manifest_hmac"`
		Signature       string              `json:"signature"`
		PublicKey       string              `json:"public_key"`
		Status          string              `json:"status"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !got.UpdateAvailable || got.Manifest.ReleaseID != "rel-api" || got.Manifest.PackageURL != "/api/v1/update/package/pkg-api" {
		t.Fatalf("response = %#v, want signed rel-api manifest", got)
	}
	if !releasesvc.VerifySignedManifest(got.PublicKey, releasesvc.SignedManifest{Manifest: got.Manifest, Signature: got.Signature}) {
		t.Fatal("response signature did not verify")
	}
	payloadBytes, err := base64.StdEncoding.DecodeString(got.ManifestPayload)
	if err != nil {
		t.Fatalf("manifest_payload is not base64: %v", err)
	}
	derivedKey, err := getDerivedKey("injector_v1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ManifestHMAC != SignHMAC(string(derivedKey), string(payloadBytes)) {
		t.Fatal("manifest_hmac did not verify against canonical manifest payload")
	}
	metrics, err := svc.store.ReleaseMetrics(t.Context(), "rel-api")
	if err != nil {
		t.Fatal(err)
	}
	if metrics.Offered != 1 {
		t.Fatalf("offered events = %d, want 1", metrics.Offered)
	}
}

func TestHandleUpdatePackageDownloadSupportsRange(t *testing.T) {
	dir := t.TempDir()
	restore := useReleaseServiceForTest(t, dir, bytes.Repeat([]byte{8}, releasesvc.ManifestSeedSize))
	defer restore()
	svc := currentReleaseService()

	if err := os.MkdirAll(filepath.Join(dir, "updates"), 0755); err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join("updates", "setup.bin")
	if err := os.WriteFile(filepath.Join(dir, packagePath), []byte("abcdef"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := svc.store.SaveRelease(t.Context(), releasesvc.Release{
		ID:             "rel-range",
		Version:        "7.0.1",
		Channel:        releasesvc.ChannelStable,
		Status:         releasesvc.StatusPublished,
		RolloutPercent: 100,
		CreatedAt:      time.Unix(600, 0).UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.store.SavePackage(t.Context(), releasesvc.ReleasePackage{
		ID:        "pkg-range",
		ReleaseID: "rel-range",
		Kind:      releasesvc.PackageKindBundle,
		Filename:  "setup.bin",
		Path:      packagePath,
		FileSize:  6,
		SHA256:    "sha-range",
		CreatedAt: time.Unix(601, 0).UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	cm := NewCardManager(NewJSONStorage(filepath.Join(dir, "cards")))
	addBoundTestSession(cm, "session-range", "card-range", "machine-range")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/update/package/pkg-range", nil)
	req.Header.Set("X-Session-Token", "session-range")
	req.Header.Set("X-Machine-ID", "machine-range")
	req.Header.Set("X-Card-Code", "card-range")
	req.Header.Set("Range", "bytes=1-3")
	signGetRequest(t, req)
	rr := httptest.NewRecorder()

	NewAPIHandler(cm).HandleUpdatePackageDownload(rr, req)

	if rr.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206; body=%s", rr.Code, rr.Body.String())
	}
	if rr.Body.String() != "bcd" {
		t.Fatalf("body = %q, want range bcd", rr.Body.String())
	}
}

func TestUpdateAPIsRequireBoundSession(t *testing.T) {
	dir := t.TempDir()
	restore := useReleaseServiceForTest(t, dir, bytes.Repeat([]byte{7}, releasesvc.ManifestSeedSize))
	defer restore()
	svc := currentReleaseService()
	publishedAt := time.Unix(650, 0).UTC()
	if err := svc.store.SaveRelease(t.Context(), releasesvc.Release{
		ID:             "rel-auth",
		Version:        "7.0.2",
		Channel:        releasesvc.ChannelStable,
		Status:         releasesvc.StatusPublished,
		RolloutPercent: 100,
		CreatedAt:      publishedAt,
		PublishedAt:    &publishedAt,
	}); err != nil {
		t.Fatal(err)
	}
	packagePath := filepath.Join("updates", "setup-auth.bin")
	if err := os.MkdirAll(filepath.Join(dir, "updates"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, packagePath), []byte("abcdef"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := svc.store.SavePackage(t.Context(), releasesvc.ReleasePackage{
		ID:        "pkg-auth",
		ReleaseID: "rel-auth",
		Kind:      releasesvc.PackageKindBundle,
		Filename:  "setup-auth.bin",
		Path:      packagePath,
		FileSize:  6,
		SHA256:    "sha-auth",
		CreatedAt: publishedAt,
	}); err != nil {
		t.Fatal(err)
	}

	cm := NewCardManager(NewJSONStorage(filepath.Join(dir, "cards")))
	addBoundTestSession(cm, "session-auth", "card-auth", "machine-auth")
	handler := NewAPIHandler(cm)

	checkBody := []byte(`{"client_version":"6.0.0","channel":"stable","machine_id":"machine-auth","card":"card-auth"}`)
	checkReq := httptest.NewRequest(http.MethodPost, "/api/v1/update/check", bytes.NewReader(checkBody))
	signPayloadRequest(t, checkReq, checkBody)
	checkRR := httptest.NewRecorder()
	handler.HandleUpdateCheck(checkRR, checkReq)
	if checkRR.Code != http.StatusUnauthorized {
		t.Fatalf("update check without token status = %d, want 401", checkRR.Code)
	}

	packageReq := httptest.NewRequest(http.MethodGet, "/api/v1/update/package/pkg-auth", nil)
	packageReq.Header.Set("X-Machine-ID", "machine-auth")
	packageReq.Header.Set("X-Card-Code", "card-auth")
	signGetRequest(t, packageReq)
	packageRR := httptest.NewRecorder()
	handler.HandleUpdatePackageDownload(packageRR, packageReq)
	if packageRR.Code != http.StatusBadRequest {
		t.Fatalf("package download without token status = %d, want 400", packageRR.Code)
	}

	mismatchBody := []byte(`{"client_version":"6.0.0","channel":"stable","machine_id":"other-machine","card":"card-auth","session_token":"session-auth"}`)
	mismatchReq := httptest.NewRequest(http.MethodPost, "/api/v1/update/check", bytes.NewReader(mismatchBody))
	signPayloadRequest(t, mismatchReq, mismatchBody)
	mismatchRR := httptest.NewRecorder()
	handler.HandleUpdateCheck(mismatchRR, mismatchReq)
	if mismatchRR.Code != http.StatusUnauthorized {
		t.Fatalf("update check machine mismatch status = %d, want 401", mismatchRR.Code)
	}
}

func TestAdminReleaseActionsPublishPauseAndRollback(t *testing.T) {
	dir := t.TempDir()
	restore := useReleaseServiceForTest(t, dir, bytes.Repeat([]byte{6}, releasesvc.ManifestSeedSize))
	defer restore()
	svc := currentReleaseService()
	if err := svc.store.SaveRelease(t.Context(), releasesvc.Release{
		ID:             "rel-admin",
		Version:        "8.0.0",
		Channel:        releasesvc.ChannelStable,
		Status:         releasesvc.StatusDraft,
		RolloutPercent: 100,
		CreatedAt:      time.Unix(700, 0).UTC(),
	}); err != nil {
		t.Fatal(err)
	}
	cm := NewCardManager(NewJSONStorage(t.TempDir()))
	handler := &AdminHandler{cm: cm}

	createReq := httptest.NewRequest(http.MethodPost, "/admin/api/releases", bytes.NewBufferString(`{"version":"8.0.1","channel":"stable","rollout_percent":100}`))
	createRR := httptest.NewRecorder()
	handler.HandleReleases(createRR, createReq)
	if createRR.Code != http.StatusOK {
		t.Fatalf("create release status = %d, want 200; body=%s", createRR.Code, createRR.Body.String())
	}

	for _, action := range []string{"publish", "pause"} {
		req := httptest.NewRequest(http.MethodPost, "/admin/api/releases/rel-admin/"+action, nil)
		rr := httptest.NewRecorder()
		handler.HandleReleaseRoute(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want 200; body=%s", action, rr.Code, rr.Body.String())
		}
	}

	body := bytes.NewBufferString(`{"target_release_id":"rel-previous"}`)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/releases/rel-admin/rollback", body)
	rr := httptest.NewRecorder()
	handler.HandleReleaseRoute(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("rollback status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	loaded, err := svc.store.GetRelease(t.Context(), "rel-admin")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != releasesvc.StatusRolledBack || loaded.RolledBackTo != "rel-previous" {
		t.Fatalf("release after rollback = %#v, want rolled_back to rel-previous", loaded)
	}
	actions := map[string]bool{}
	for _, entry := range cm.AuditLog() {
		actions[entry.Action] = true
	}
	for _, action := range []string{"release_created", "release_published", "release_paused", "release_rolled_back"} {
		if !actions[action] {
			t.Fatalf("missing audit action %s in %#v", action, cm.AuditLog())
		}
	}
}

func signGetRequest(t *testing.T, req *http.Request) {
	t.Helper()
	derivedKey, err := getDerivedKey("injector_v1")
	if err != nil {
		t.Fatal(err)
	}
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	nonce := fmt.Sprintf("release-%d", time.Now().UnixNano())
	signedData := timestamp + "|" + nonce + "|" + req.URL.Path
	req.Header.Set("X-Client-ID", "injector_v1")
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-HMAC-Signature", SignHMAC(string(derivedKey), signedData))
}

func useReleaseServiceForTest(t *testing.T, dir string, seed []byte) func() {
	t.Helper()
	svc, err := newReleaseService(dir, seed)
	if err != nil {
		t.Fatal(err)
	}
	releaseSvcMu.Lock()
	old := releaseSvc
	releaseSvc = svc
	releaseSvcMu.Unlock()
	return func() {
		releaseSvcMu.Lock()
		releaseSvc = old
		releaseSvcMu.Unlock()
		_ = svc.store.Close()
	}
}
