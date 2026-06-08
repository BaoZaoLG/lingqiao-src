package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const maxScriptModuleBytes = 2 << 20

var scriptVersionRe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

type ScriptModule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	SHA256    string    `json:"sha256"`
	Content   string    `json:"content"`
	Size      int       `json:"size"`
	Note      string    `json:"note,omitempty"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type scriptIndex struct {
	ActiveID string         `json:"active_id"`
	Scripts  []ScriptModule `json:"scripts"`
}

func scriptDir() string { return dataPath("scripts") }

func scriptIndexPath() string { return filepath.Join(scriptDir(), "index.json") }

func scriptModulePath(id string) string {
	return filepath.Join(scriptDir(), sanitizeScriptVersion(id)+".json")
}

func LoadActiveScriptModule() (*ScriptModule, error) {
	scripts, activeID, err := ListScriptModules()
	if err == nil && activeID != "" {
		for _, module := range scripts {
			if module.ID == activeID {
				return loadScriptModuleContent(module.ID)
			}
		}
	}

	data, err := os.ReadFile(filepath.Join(scriptDir(), "active.json"))
	if err != nil {
		return nil, err
	}
	var module ScriptModule
	if err := json.Unmarshal(data, &module); err != nil {
		return nil, err
	}
	if module.Content == "" || module.SHA256 == "" {
		return nil, fmt.Errorf("active script is incomplete")
	}
	normalizeScriptModule(&module)
	return &module, nil
}

func SaveActiveScriptModule(version, content string) (*ScriptModule, error) {
	module, err := saveScriptModule(version, content, "", "active", true)
	if err != nil {
		return nil, err
	}
	return module, SetActiveScriptModule(module.ID)
}

func SaveScriptModuleDraft(version, content, note string) (*ScriptModule, error) {
	return saveScriptModule(version, content, note, "", false)
}

func saveScriptModule(version, content, note, name string, active bool) (*ScriptModule, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if len([]byte(content)) > maxScriptModuleBytes {
		return nil, fmt.Errorf("content exceeds %d bytes", maxScriptModuleBytes)
	}

	version = strings.TrimSpace(version)
	if version == "" {
		version = time.Now().Format("20060102.150405")
	}
	version = sanitizeScriptVersion(version)
	if version == "" {
		return nil, fmt.Errorf("version is invalid")
	}

	now := time.Now().UTC()
	sum := sha256.Sum256([]byte(content))
	id := fmt.Sprintf("%s-%s", version, now.Format("20060102T150405000Z"))
	if name == "" {
		name = "JS " + version
	}
	module := &ScriptModule{
		ID:        id,
		Name:      name,
		Version:   version,
		SHA256:    hex.EncodeToString(sum[:]),
		Content:   content,
		Size:      len([]byte(content)),
		Note:      strings.TrimSpace(note),
		Active:    active,
		CreatedAt: now,
		UpdatedAt: now,
	}

	dir := scriptDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(module, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(scriptModulePath(module.ID), data, 0600); err != nil {
		return nil, err
	}
	historyName := fmt.Sprintf("%s-%s.json", module.Version, module.UpdatedAt.Format("20060102T150405Z"))
	if err := os.WriteFile(filepath.Join(dir, historyName), data, 0600); err != nil {
		return nil, err
	}
	if err := upsertScriptIndex(*module, active); err != nil {
		return nil, err
	}
	return module, nil
}

func ListScriptModules() ([]ScriptModule, string, error) {
	if err := migrateLegacyActiveScript(); err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(scriptIndexPath())
	if err != nil {
		if os.IsNotExist(err) {
			return []ScriptModule{}, "", nil
		}
		return nil, "", err
	}
	var idx scriptIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, "", err
	}
	for i := range idx.Scripts {
		idx.Scripts[i].Active = idx.Scripts[i].ID == idx.ActiveID
		idx.Scripts[i].Content = ""
	}
	return idx.Scripts, idx.ActiveID, nil
}

func SetActiveScriptModule(id string) error {
	idx, err := readScriptIndex()
	if err != nil {
		return err
	}
	found := false
	for i := range idx.Scripts {
		idx.Scripts[i].Active = idx.Scripts[i].ID == id
		if idx.Scripts[i].ID == id {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("script not found")
	}
	idx.ActiveID = id
	if err := writeScriptIndex(idx); err != nil {
		return err
	}
	module, err := loadScriptModuleContent(id)
	if err != nil {
		return err
	}
	module.Active = true
	data, _ := json.MarshalIndent(module, "", "  ")
	return os.WriteFile(filepath.Join(scriptDir(), "active.json"), data, 0600)
}

func DeleteScriptModule(id string) error {
	idx, err := readScriptIndex()
	if err != nil {
		return err
	}
	if idx.ActiveID == id {
		return fmt.Errorf("cannot delete active script")
	}
	next := make([]ScriptModule, 0, len(idx.Scripts))
	found := false
	for _, module := range idx.Scripts {
		if module.ID == id {
			found = true
			continue
		}
		next = append(next, module)
	}
	if !found {
		return fmt.Errorf("script not found")
	}
	idx.Scripts = next
	if err := writeScriptIndex(idx); err != nil {
		return err
	}
	return os.Remove(scriptModulePath(id))
}

func loadScriptModuleContent(id string) (*ScriptModule, error) {
	data, err := os.ReadFile(scriptModulePath(id))
	if err != nil {
		return nil, err
	}
	var module ScriptModule
	if err := json.Unmarshal(data, &module); err != nil {
		return nil, err
	}
	normalizeScriptModule(&module)
	return &module, nil
}

func upsertScriptIndex(module ScriptModule, active bool) error {
	idx, err := readScriptIndex()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	module.Content = ""
	replaced := false
	for i := range idx.Scripts {
		if idx.Scripts[i].ID == module.ID {
			idx.Scripts[i] = module
			replaced = true
			break
		}
	}
	if !replaced {
		idx.Scripts = append([]ScriptModule{module}, idx.Scripts...)
	}
	if active {
		idx.ActiveID = module.ID
	}
	for i := range idx.Scripts {
		idx.Scripts[i].Active = idx.Scripts[i].ID == idx.ActiveID
	}
	return writeScriptIndex(idx)
}

func readScriptIndex() (scriptIndex, error) {
	var idx scriptIndex
	data, err := os.ReadFile(scriptIndexPath())
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

func writeScriptIndex(idx scriptIndex) error {
	if err := os.MkdirAll(scriptDir(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(scriptIndexPath(), data, 0600)
}

func migrateLegacyActiveScript() error {
	if _, err := os.Stat(scriptIndexPath()); err == nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(scriptDir(), "active.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var module ScriptModule
	if err := json.Unmarshal(data, &module); err != nil {
		return err
	}
	normalizeScriptModule(&module)
	module.Active = true
	if err := os.MkdirAll(scriptDir(), 0755); err != nil {
		return err
	}
	moduleData, err := json.MarshalIndent(module, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(scriptModulePath(module.ID), moduleData, 0600); err != nil {
		return err
	}
	module.Content = ""
	return writeScriptIndex(scriptIndex{ActiveID: module.ID, Scripts: []ScriptModule{module}})
}

func normalizeScriptModule(module *ScriptModule) {
	if module.ID == "" {
		base := module.Version
		if base == "" {
			base = "legacy"
		}
		t := module.UpdatedAt
		if t.IsZero() {
			t = time.Now().UTC()
		}
		module.ID = sanitizeScriptVersion(base + "-" + t.Format("20060102T150405Z"))
	}
	if module.Name == "" {
		module.Name = "JS " + module.Version
	}
	if module.CreatedAt.IsZero() {
		module.CreatedAt = module.UpdatedAt
	}
	if module.UpdatedAt.IsZero() {
		module.UpdatedAt = time.Now().UTC()
	}
	if module.Size == 0 && module.Content != "" {
		module.Size = len([]byte(module.Content))
	}
}

func sanitizeScriptVersion(version string) string {
	version = scriptVersionRe.ReplaceAllString(version, "-")
	return strings.Trim(version, ".-_")
}

func (h *APIHandler) HandleScriptDownload(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if r.Header.Get("X-HMAC-Signature") != "" {
		if err := verifyRequestHMAC(r, r.URL.Path); err != nil {
			writeError(w, http.StatusUnauthorized, "signature verification failed")
			return
		}
	}

	token := r.Header.Get("X-Session-Token")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		writeError(w, http.StatusBadRequest, "token is required")
		return
	}

	h.cm.mu.RLock()
	session, exists := h.cm.sessions[token]
	h.cm.mu.RUnlock()
	if !exists || session.ExpiresAt.Before(time.Now()) {
		writeError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}

	module, err := LoadActiveScriptModule()
	if err != nil {
		log.Printf("[SCRIPT] active script unavailable: %v", err)
		writeError(w, http.StatusNotFound, "script module not published")
		return
	}
	writeOK(w, map[string]interface{}{
		"version":    module.Version,
		"sha256":     module.SHA256,
		"content":    module.Content,
		"size":       module.Size,
		"updated_at": module.UpdatedAt,
	})
}

func (h *AdminHandler) HandleScriptAdmin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if id := r.URL.Query().Get("id"); id != "" {
			module, err := loadScriptModuleContent(id)
			if err != nil {
				writeError(w, http.StatusNotFound, "script not found")
				return
			}
			writeOK(w, map[string]interface{}{"script": module})
			return
		}
		list, activeID, err := ListScriptModules()
		if err == nil {
			data := map[string]interface{}{"scripts": list, "active_id": activeID}
			if activeID != "" {
				if active, err := loadScriptModuleContent(activeID); err == nil {
					data["script"] = active
					data["id"] = active.ID
					data["version"] = active.Version
					data["sha256"] = active.SHA256
					data["content"] = active.Content
					data["size"] = active.Size
					data["updated_at"] = active.UpdatedAt
				}
			}
			writeOK(w, data)
			return
		}
		module, err := LoadActiveScriptModule()
		if err != nil {
			writeOK(w, map[string]interface{}{"script": nil})
			return
		}
		writeOK(w, map[string]interface{}{
			"version":    module.Version,
			"sha256":     module.SHA256,
			"content":    module.Content,
			"size":       module.Size,
			"updated_at": module.UpdatedAt,
		})
	case http.MethodPost:
		var req struct {
			Version string `json:"version"`
			Content string `json:"content"`
			Note    string `json:"note"`
			Action  string `json:"action"`
			ID      string `json:"id"`
		}
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		switch req.Action {
		case "activate":
			_, previousActiveID, _ := ListScriptModules()
			if err := SetActiveScriptModule(req.ID); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			module, _ := loadScriptModuleContent(req.ID)
			h.recordScriptAudit("script_activated", module, fmt.Sprintf("previous_id=%s", previousActiveID))
			writeOK(w, nil)
			return
		case "delete":
			module, _ := loadScriptModuleContent(req.ID)
			if err := DeleteScriptModule(req.ID); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			h.recordScriptAudit("script_deleted", module, "")
			writeOK(w, nil)
			return
		case "save":
			module, err := SaveScriptModuleDraft(req.Version, req.Content, req.Note)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			h.recordScriptAudit("script_saved", module, "")
			writeOK(w, map[string]interface{}{"script": module, "id": module.ID, "version": module.Version, "sha256": module.SHA256, "content": module.Content, "size": module.Size, "updated_at": module.UpdatedAt})
			return
		}
		module, err := saveScriptModule(req.Version, req.Content, req.Note, "", true)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := SetActiveScriptModule(module.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.recordScriptAudit("script_published", module, "")
		log.Printf("[SCRIPT] Published version=%s sha256=%s size=%d", module.Version, module.SHA256, module.Size)
		writeOK(w, map[string]interface{}{
			"id":         module.ID,
			"name":       module.Name,
			"version":    module.Version,
			"sha256":     module.SHA256,
			"content":    module.Content,
			"size":       module.Size,
			"updated_at": module.UpdatedAt,
		})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *AdminHandler) recordScriptAudit(action string, module *ScriptModule, extra string) {
	if h == nil || h.cm == nil || module == nil {
		return
	}
	detail := fmt.Sprintf("id=%s version=%s sha256=%s size=%d", module.ID, module.Version, module.SHA256, module.Size)
	if extra != "" {
		detail += " " + extra
	}
	h.cm.RecordAudit(AuditEntry{
		Action: action,
		Detail: detail,
	})
}
