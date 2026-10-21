package scripts

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/lingqiao/server/internal/audit"
	"github.com/lingqiao/server/internal/auth"
	"github.com/lingqiao/server/internal/features/sessions"
	"github.com/lingqiao/server/internal/httpapi"
)

// Handler handles script module admin and download endpoints.
type Handler struct {
	repo          *Repository
	auditRecorder *audit.Recorder
	sessionsSvc   *sessions.Service
}

// NewHandler creates a new script handler rooted at dataDir.
func NewHandler(dataDir string, auditRecorder *audit.Recorder, sessionsSvc *sessions.Service) *Handler {
	return &Handler{
		repo:          NewRepository(dataDir),
		auditRecorder: auditRecorder,
		sessionsSvc:   sessionsSvc,
	}
}

// SetAuditSink sets the audit sink.
func (h *Handler) SetAuditSink(sink func(audit.Event)) {
	// Kept for backwards compatibility; auditRecorder is already provided at construction.
}

// HandleScriptDownload serves the active script to clients.
func (h *Handler) HandleScriptDownload(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	if err := auth.VerifyRequest(r, r.URL.Path); err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "signature verification failed")
		return
	}

	token := r.Header.Get("X-Session-Token")
	if token == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "token is required")
		return
	}

	machineID := r.Header.Get("X-Machine-ID")
	cardCode := r.Header.Get("X-Card-Code")

	var sess sessions.Session
	var err error
	if machineID != "" && cardCode != "" {
		sess, err = h.sessionsSvc.ValidateBound(r.Context(), token, machineID, cardCode)
	} else {
		sess, err = h.sessionsSvc.ValidateActiveToken(r.Context(), token)
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusUnauthorized, "invalid or expired session")
		return
	}
	_ = sess

	module, err := h.repo.GetActive()
	if err != nil {
		log.Printf("[SCRIPT] active script unavailable: %v", err)
		httpapi.WriteError(w, http.StatusNotFound, "script module not published")
		return
	}

	w.Header().Set("X-Script-ID", module.ID)
	w.Header().Set("X-Script-Version", module.Version)
	w.Header().Set("X-Script-SHA256", module.SHA256)
	w.Header().Set("X-Script-Hash", module.SHA256)
	w.Header().Set("X-Script-Size", fmt.Sprintf("%d", module.Size))

	if strings.Contains(r.Header.Get("Accept"), "application/json") || r.URL.Query().Get("format") == "json" {
		httpapi.WriteOK(w, map[string]interface{}{
			"version":    module.Version,
			"sha256":     module.SHA256,
			"content":    module.Content,
			"size":       module.Size,
			"updated_at": module.UpdatedAt,
		})
		return
	}

	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(module.Content))
}

// HandleList returns the list of scripts (with content) and active ID.
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	list, activeID, err := h.repo.List()
	if err != nil {
		log.Printf("[SCRIPT] list failed: %v", err)
		httpapi.WriteError(w, http.StatusInternalServerError, "failed to list scripts")
		return
	}
	active := (*ScriptModule)(nil)
	for i := range list {
		if list[i].ID == activeID {
			active = &list[i]
			break
		}
	}
	httpapi.WriteOK(w, map[string]interface{}{
		"scripts":   list,
		"active_id": activeID,
		"active":    active,
	})
}

// HandleGet returns a single script version.
func (h *Handler) HandleGet(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "id is required")
		return
	}
	module, err := h.repo.Get(id)
	if err != nil {
		httpapi.WriteError(w, http.StatusNotFound, "script not found")
		return
	}
	httpapi.WriteOK(w, map[string]interface{}{"script": module})
}

// HandleCreate creates a new draft version.
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	var req struct {
		Version string `json:"version"`
		Content string `json:"content"`
		Note    string `json:"note"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	module, err := h.repo.Create(req.Version, req.Content, req.Note)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "failed to create script")
		return
	}
	h.recordScriptAudit("script_created", module, "")
	httpapi.WriteOK(w, map[string]interface{}{"script": module})
}

// HandlePublish creates and immediately activates a new version.
func (h *Handler) HandlePublish(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	var req struct {
		Version string `json:"version"`
		Content string `json:"content"`
		Note    string `json:"note"`
	}
	if err := httpapi.ReadJSON(r, &req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	module, err := h.repo.Publish(req.Version, req.Content, req.Note)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "failed to publish script")
		return
	}
	h.recordScriptAudit("script_published", module, "")
	log.Printf("[SCRIPT] Published version=%s sha256=%s size=%d", module.Version, module.SHA256, module.Size)
	httpapi.WriteOK(w, map[string]interface{}{"script": module})
}

// HandleActivate activates an existing version.
func (h *Handler) HandleActivate(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "id is required")
		return
	}
	module, err := h.repo.Activate(id)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			httpapi.WriteError(w, http.StatusNotFound, "script not found")
			return
		}
		httpapi.WriteError(w, http.StatusBadRequest, "failed to activate script")
		return
	}
	h.recordScriptAudit("script_activated", module, "")
	httpapi.WriteOK(w, map[string]interface{}{"script": module})
}

// HandleDelete deletes a non-active script version.
func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodDelete) {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		httpapi.WriteError(w, http.StatusBadRequest, "id is required")
		return
	}
	if err := h.repo.Delete(id); err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "not found") {
			httpapi.WriteError(w, http.StatusNotFound, "script not found")
			return
		}
		if strings.Contains(msg, "active") {
			httpapi.WriteError(w, http.StatusBadRequest, "cannot delete active script")
			return
		}
		httpapi.WriteError(w, http.StatusBadRequest, "failed to delete script")
		return
	}
	h.recordScriptAudit("script_deleted", &ScriptModule{ID: id}, "")
	httpapi.WriteOK(w, map[string]interface{}{"id": id})
}

// HandleScriptAdmin is the legacy single-endpoint API kept for backwards compatibility.
// It delegates to the new handlers based on the request method and body fields.
func (h *Handler) HandleScriptAdmin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if id := r.URL.Query().Get("id"); id != "" {
			module, err := h.repo.Get(id)
			if err != nil {
				httpapi.WriteError(w, http.StatusNotFound, "script not found")
				return
			}
			httpapi.WriteOK(w, map[string]interface{}{"script": module})
			return
		}
		list, activeID, err := h.repo.List()
		if err != nil {
			log.Printf("[SCRIPT] legacy list failed: %v", err)
			httpapi.WriteError(w, http.StatusInternalServerError, "failed to list scripts")
			return
		}
		active := (*ScriptModule)(nil)
		for i := range list {
			if list[i].ID == activeID {
				active = &list[i]
				break
			}
		}
		// Return the active script content in legacy top-level fields.
		data := map[string]interface{}{
			"scripts":    list,
			"active_id":  activeID,
		}
		if active != nil {
			module, _ := h.repo.Get(active.ID)
			if module != nil {
				data["script"] = module
				data["id"] = module.ID
				data["version"] = module.Version
				data["sha256"] = module.SHA256
				data["content"] = module.Content
				data["size"] = module.Size
				data["updated_at"] = module.UpdatedAt
			}
		}
		httpapi.WriteOK(w, data)
	case http.MethodPost:
		var req struct {
			Version string `json:"version"`
			Content string `json:"content"`
			Note    string `json:"note"`
			Action  string `json:"action"`
			ID      string `json:"id"`
		}
		if err := httpapi.ReadJSON(r, &req); err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		switch req.Action {
		case "activate":
			module, err := h.repo.Activate(req.ID)
			if err != nil {
				if strings.Contains(strings.ToLower(err.Error()), "not found") {
					httpapi.WriteError(w, http.StatusNotFound, "script not found")
					return
				}
				httpapi.WriteError(w, http.StatusBadRequest, "failed to activate script")
				return
			}
			h.recordScriptAudit("script_activated", module, "")
			httpapi.WriteOK(w, map[string]interface{}{"script": module})
			return
		case "delete":
			if err := h.repo.Delete(req.ID); err != nil {
				msg := strings.ToLower(err.Error())
				if strings.Contains(msg, "not found") {
					httpapi.WriteError(w, http.StatusNotFound, "script not found")
					return
				}
				if strings.Contains(msg, "active") {
					httpapi.WriteError(w, http.StatusBadRequest, "cannot delete active script")
					return
				}
				httpapi.WriteError(w, http.StatusBadRequest, "failed to delete script")
				return
			}
			h.recordScriptAudit("script_deleted", &ScriptModule{ID: req.ID}, "")
			httpapi.WriteOK(w, map[string]interface{}{"id": req.ID})
			return
		case "save":
			module, err := h.repo.Create(req.Version, req.Content, req.Note)
			if err != nil {
				httpapi.WriteError(w, http.StatusBadRequest, "failed to create script")
				return
			}
			h.recordScriptAudit("script_created", module, "")
			httpapi.WriteOK(w, map[string]interface{}{"script": module})
			return
		}
		// Default legacy action: publish (create + activate).
		module, err := h.repo.Publish(req.Version, req.Content, req.Note)
		if err != nil {
			httpapi.WriteError(w, http.StatusBadRequest, "failed to publish script")
			return
		}
		h.recordScriptAudit("script_published", module, "")
		log.Printf("[SCRIPT] Published version=%s sha256=%s size=%d", module.Version, module.SHA256, module.Size)
		httpapi.WriteOK(w, map[string]interface{}{
			"id":         module.ID,
			"version":    module.Version,
			"sha256":     module.SHA256,
			"content":    module.Content,
			"size":       module.Size,
			"updated_at": module.UpdatedAt,
		})
	default:
		httpapi.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) recordScriptAudit(action string, module *ScriptModule, extra string) error {
	if h == nil || h.auditRecorder == nil || module == nil {
		return nil
	}
	detail := fmt.Sprintf("id=%s version=%s sha256=%s size=%d", module.ID, module.Version, module.SHA256, module.Size)
	if extra != "" {
		detail += " " + extra
	}
	return h.auditRecorder.Append(audit.Event{
		Action: action,
		Detail: detail,
	})
}

// LoadActiveScriptModule is kept for backwards compatibility with external callers.
func (h *Handler) LoadActiveScriptModule() (*ScriptModule, error) {
	return h.repo.GetActive()
}

// Init is kept for backwards compatibility.
func Init(dir string) {
	// Repository is now initialized per-handler; no global state.
}

// SaveActiveScriptModule is kept for backwards compatibility.
var SaveActiveScriptModule = func(version, content string) (*ScriptModule, error) {
	return nil, errors.New("deprecated: use Handler or Repository directly")
}

// SaveScriptModuleDraft is kept for backwards compatibility.
var SaveScriptModuleDraft = func(version, content, note string) (*ScriptModule, error) {
	return nil, errors.New("deprecated: use Handler or Repository directly")
}

// ListScriptModules is kept for backwards compatibility.
var ListScriptModules = func() ([]ScriptModule, string, error) {
	return nil, "", errors.New("deprecated: use Handler or Repository directly")
}

// SetActiveScriptModule is kept for backwards compatibility.
var SetActiveScriptModule = func(id string) error {
	_ = id
	return errors.New("deprecated: use Handler or Repository directly")
}

// DeleteScriptModule is kept for backwards compatibility.
var DeleteScriptModule = func(id string) error {
	_ = id
	return errors.New("deprecated: use Handler or Repository directly")
}
