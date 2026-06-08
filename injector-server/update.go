package main

import (
	"crypto/sha256"
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

	"github.com/lingqiao/server/internal/storage"
	updatesvc "github.com/lingqiao/server/internal/updates"
)

type UpdateInfo = updatesvc.UpdateInfo

type UpdatePackage struct {
	Version    string    `json:"version"`
	Filename   string    `json:"filename"`
	FileSize   int64     `json:"file_size"`
	SHA256     string    `json:"sha256"`
	UploadedAt time.Time `json:"uploaded_at"`
	Active     bool      `json:"active"`
}

type updateIndex struct {
	ActiveVersion string          `json:"active_version"`
	Packages      []UpdatePackage `json:"packages"`
}

var (
	currentUpdate       *UpdateInfo
	updateMu            sync.RWMutex
	updateMetadataStore = updatesvc.NewMetadataStore(storage.NewJSONStore(dataPath("updates")))
)

func updateDir() string {
	return dataPath("updates")
}

func configureUpdateStore(dir string) {
	updateMu.Lock()
	updateMetadataStore = updatesvc.NewMetadataStore(storage.NewJSONStore(filepath.Join(dir, "updates")))
	currentUpdate = nil
	updateMu.Unlock()
}

func getCurrentUpdate() *UpdateInfo {
	updateMu.RLock()
	if currentUpdate != nil {
		info := *currentUpdate
		updateMu.RUnlock()
		return &info
	}
	updateMu.RUnlock()

	info, err := updateMetadataStore.Load()
	if err != nil || info == nil {
		return nil
	}
	updateMu.Lock()
	currentUpdate = info
	updateMu.Unlock()
	cp := *info
	return &cp
}

func getUpdateSHA256(version string) string {
	info := getCurrentUpdate()
	if info == nil {
		return ""
	}
	if version == "" || info.Version == version {
		return info.SHA256
	}
	return ""
}

func saveCurrentUpdate(info *UpdateInfo) error {
	if info == nil {
		return fmt.Errorf("update info is nil")
	}
	if err := updateMetadataStore.Save(*info); err != nil {
		return err
	}
	updateMu.Lock()
	cp := *info
	currentUpdate = &cp
	updateMu.Unlock()
	return nil
}

func updateIndexPath() string {
	return dataPath("updates", "index.json")
}

func AddUpdatePackage(info *UpdateInfo, active bool) error {
	if info == nil || info.Version == "" || info.Filename == "" {
		return fmt.Errorf("update info is incomplete")
	}
	idx, err := readUpdateIndex()
	if err != nil {
		return err
	}
	pkg := UpdatePackage{
		Version:    info.Version,
		Filename:   info.Filename,
		FileSize:   info.FileSize,
		SHA256:     info.SHA256,
		UploadedAt: info.UploadedAt,
		Active:     active,
	}
	replaced := false
	for i := range idx.Packages {
		if idx.Packages[i].Version == pkg.Version {
			idx.Packages[i] = pkg
			replaced = true
			break
		}
	}
	if !replaced {
		idx.Packages = append([]UpdatePackage{pkg}, idx.Packages...)
	}
	if active || idx.ActiveVersion == "" {
		idx.ActiveVersion = pkg.Version
	}
	markActiveUpdatePackages(&idx)
	if err := writeUpdateIndex(idx); err != nil {
		return err
	}
	if idx.ActiveVersion == pkg.Version {
		return saveCurrentUpdate(updateInfoFromPackage(pkg))
	}
	return nil
}

func ListUpdatePackages() ([]UpdatePackage, string, error) {
	if err := migrateLegacyUpdateInfo(); err != nil {
		return nil, "", err
	}
	idx, err := readUpdateIndex()
	if err != nil {
		return nil, "", err
	}
	markActiveUpdatePackages(&idx)
	return idx.Packages, idx.ActiveVersion, nil
}

func SetCurrentUpdateVersion(version string) error {
	idx, err := readUpdateIndex()
	if err != nil {
		return err
	}
	var selected *UpdatePackage
	for i := range idx.Packages {
		if idx.Packages[i].Version == version {
			selected = &idx.Packages[i]
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("update package not found")
	}
	idx.ActiveVersion = version
	markActiveUpdatePackages(&idx)
	if err := writeUpdateIndex(idx); err != nil {
		return err
	}
	return saveCurrentUpdate(updateInfoFromPackage(*selected))
}

func DeleteUpdatePackage(version string) error {
	idx, err := readUpdateIndex()
	if err != nil {
		return err
	}
	if idx.ActiveVersion == version {
		return fmt.Errorf("cannot delete active update package")
	}
	next := make([]UpdatePackage, 0, len(idx.Packages))
	var filename string
	found := false
	for _, pkg := range idx.Packages {
		if pkg.Version == version {
			found = true
			filename = pkg.Filename
			continue
		}
		next = append(next, pkg)
	}
	if !found {
		return fmt.Errorf("update package not found")
	}
	idx.Packages = next
	if err := writeUpdateIndex(idx); err != nil {
		return err
	}
	if filename != "" {
		_ = os.Remove(dataPath("updates", filename))
	}
	return nil
}

func readUpdateIndex() (updateIndex, error) {
	var idx updateIndex
	data, err := os.ReadFile(updateIndexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return idx, nil
		}
		return idx, err
	}
	if err := json.Unmarshal(data, &idx); err != nil {
		return idx, err
	}
	return idx, nil
}

func writeUpdateIndex(idx updateIndex) error {
	if err := os.MkdirAll(updateDir(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(updateIndexPath(), data, 0600)
}

func migrateLegacyUpdateInfo() error {
	if _, err := os.Stat(updateIndexPath()); err == nil {
		return nil
	}
	info := getCurrentUpdate()
	if info == nil || info.Version == "" {
		return nil
	}
	return writeUpdateIndex(updateIndex{
		ActiveVersion: info.Version,
		Packages: []UpdatePackage{{
			Version:    info.Version,
			Filename:   info.Filename,
			FileSize:   info.FileSize,
			SHA256:     info.SHA256,
			UploadedAt: info.UploadedAt,
			Active:     true,
		}},
	})
}

func markActiveUpdatePackages(idx *updateIndex) {
	for i := range idx.Packages {
		idx.Packages[i].Active = idx.Packages[i].Version == idx.ActiveVersion
	}
}

func updateInfoFromPackage(pkg UpdatePackage) *UpdateInfo {
	return &UpdateInfo{
		Version:    pkg.Version,
		Filename:   pkg.Filename,
		FileSize:   pkg.FileSize,
		SHA256:     pkg.SHA256,
		UploadedAt: pkg.UploadedAt,
	}
}

func useUpdateStoreForTest(dir string) func() {
	updateMu.Lock()
	oldStore := updateMetadataStore
	oldCurrent := currentUpdate
	updateMetadataStore = updatesvc.NewMetadataStore(storage.NewJSONStore(dir))
	currentUpdate = nil
	updateMu.Unlock()
	return func() {
		updateMu.Lock()
		updateMetadataStore = oldStore
		currentUpdate = oldCurrent
		updateMu.Unlock()
	}
}

func fileSHA256(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return ""
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (h *AdminHandler) HandleUpdateUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := r.ParseMultipartForm(200 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "文件太大或格式错误")
		return
	}

	version := r.FormValue("version")
	if version == "" {
		writeError(w, http.StatusBadRequest, "版本号不能为空")
		return
	}

	// Sanitize version — only allow digits and dots
	for _, c := range version {
		if !((c >= '0' && c <= '9') || c == '.') {
			writeError(w, http.StatusBadRequest, "版本号格式无效")
			return
		}
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "未找到上传文件")
		return
	}
	defer file.Close()

	filename := header.Filename
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".exe" {
		writeError(w, http.StatusBadRequest, "只支持 .exe 文件")
		return
	}

	safeFilename := fmt.Sprintf("Injector_v%s%s", version, ext)
	savePath := dataPath("updates", safeFilename)

	dst, err := os.Create(savePath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "保存文件失败")
		return
	}
	written, err := io.Copy(dst, file)
	dst.Close()
	if err != nil {
		os.Remove(savePath)
		writeError(w, http.StatusInternalServerError, "写入文件失败")
		return
	}

	sha256Hex := fileSHA256(savePath)

	info := &UpdateInfo{
		Version:    version,
		Filename:   safeFilename,
		FileSize:   written,
		SHA256:     sha256Hex,
		UploadedAt: time.Now(),
	}
	if err := AddUpdatePackage(info, true); err != nil {
		writeError(w, http.StatusInternalServerError, "保存更新元数据失败")
		return
	}

	a := GetAnnouncement()
	if a == nil {
		SetAnnouncement("", version, "", false)
	} else {
		SetAnnouncement(a.Content, version, a.MinVersion, a.ForceUpdate)
	}

	log.Printf("[ADMIN] Update uploaded: v%s (%s, %d bytes)", version, safeFilename, written)
	writeJSON(w, map[string]interface{}{
		"status":    "ok",
		"version":   version,
		"filename":  safeFilename,
		"file_size": written,
		"sha256":    sha256Hex,
	})
}

func (h *AdminHandler) HandleUpdateInfo(w http.ResponseWriter, r *http.Request) {
	info := getCurrentUpdate()
	packages, activeVersion, _ := ListUpdatePackages()

	if info == nil {
		writeJSON(w, map[string]interface{}{"status": "ok", "update": nil, "packages": packages, "active_version": activeVersion})
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "update": info, "packages": packages, "active_version": activeVersion})
}

func (h *AdminHandler) HandleUpdateManage(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var req struct {
		Action  string `json:"action"`
		Version string `json:"version"`
	}
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	switch req.Action {
	case "activate":
		if err := SetCurrentUpdateVersion(req.Version); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	case "delete":
		if err := DeleteUpdatePackage(req.Version); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "unknown action")
		return
	}
	writeOK(w, nil)
}

func (h *AdminHandler) HandleUpdateDownload(w http.ResponseWriter, r *http.Request) {
	clientID := r.Header.Get("X-Client-ID")
	signature := r.Header.Get("X-HMAC-Signature")
	timestamp := r.Header.Get("X-Timestamp")
	nonce := r.Header.Get("X-Nonce")
	if clientID == "" || signature == "" || timestamp == "" || nonce == "" {
		log.Printf("[UPDATE] Download rejected: missing auth from %s", getClientIP(r))
		writeError(w, http.StatusUnauthorized, "missing authentication")
		return
	}
	if err := VerifyHMAC(clientID, r.URL.Path, signature, timestamp, nonce); err != nil {
		log.Printf("[UPDATE] Download rejected: HMAC failed from %s: %v", getClientIP(r), err)
		if !h.checkSession(r) {
			writeError(w, http.StatusUnauthorized, "authentication failed")
			return
		}
	}

	info := getCurrentUpdate()

	if info == nil {
		log.Printf("[UPDATE] Download rejected: no update available (client: %s)", clientID)
		writeError(w, http.StatusNotFound, "no update available")
		return
	}

	filePath := dataPath("updates", info.Filename)
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("[UPDATE] Download rejected: file not found: %s", filePath)
		writeError(w, http.StatusNotFound, "update file not found")
		return
	}
	defer file.Close()

	stat, _ := file.Stat()
	log.Printf("[UPDATE] Serving v%s to %s (%s, %d bytes)", info.Version, clientID, getClientIP(r), stat.Size())
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", info.Filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", stat.Size()))
	w.Header().Set("X-Update-Version", info.Version)
	if info.SHA256 != "" {
		w.Header().Set("X-Update-SHA256", info.SHA256)
	}
	io.Copy(w, file)
}
