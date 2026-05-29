package main

import (
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
)

type UpdateInfo struct {
	Version    string    `json:"version"`
	Filename   string    `json:"filename"`
	FileSize   int64     `json:"file_size"`
	UploadedAt time.Time `json:"uploaded_at"`
}

var (
	currentUpdate *UpdateInfo
	updateMu      sync.RWMutex
)

func init() {
	os.MkdirAll("data/updates", 0755)
	data, err := os.ReadFile("data/updates/info.json")
	if err != nil {
		return
	}
	var info UpdateInfo
	if json.Unmarshal(data, &info) == nil && info.Version != "" {
		path := filepath.Join("data/updates", info.Filename)
		if _, err := os.Stat(path); err == nil {
			currentUpdate = &info
		}
	}
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
	savePath := filepath.Join("data/updates", safeFilename)

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

	updateMu.Lock()
	if currentUpdate != nil && currentUpdate.Filename != safeFilename {
		oldPath := filepath.Join("data/updates", currentUpdate.Filename)
		os.Remove(oldPath)
	}
	info := &UpdateInfo{
		Version:    version,
		Filename:   safeFilename,
		FileSize:   written,
		UploadedAt: time.Now(),
	}
	currentUpdate = info
	updateMu.Unlock()

	infoData, _ := json.Marshal(info)
	os.WriteFile("data/updates/info.json", infoData, 0600)

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
	})
}

func (h *AdminHandler) HandleUpdateInfo(w http.ResponseWriter, r *http.Request) {
	updateMu.RLock()
	info := currentUpdate
	updateMu.RUnlock()

	if info == nil {
		writeJSON(w, map[string]interface{}{"status": "ok", "update": nil})
		return
	}
	writeJSON(w, map[string]interface{}{"status": "ok", "update": info})
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

	updateMu.RLock()
	info := currentUpdate
	updateMu.RUnlock()

	if info == nil {
		log.Printf("[UPDATE] Download rejected: no update available (client: %s)", clientID)
		writeError(w, http.StatusNotFound, "no update available")
		return
	}

	filePath := filepath.Join("data/updates", info.Filename)
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
	io.Copy(w, file)
}
