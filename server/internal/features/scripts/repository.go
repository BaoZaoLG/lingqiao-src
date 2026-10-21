package scripts

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const maxScriptModuleBytes = 2 << 20

var scriptVersionRe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Status constants for script versions.
const (
	StatusDraft    = "draft"
	StatusActive   = "active"
	StatusArchived = "archived"
)

// ScriptModule represents a single script version.
type ScriptModule struct {
	ID          string    `json:"id"`
	Version     string    `json:"version"`
	SHA256      string    `json:"sha256"`
	Content     string    `json:"content"`
	Size        int       `json:"size"`
	Note        string    `json:"note,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ActivatedAt time.Time `json:"activated_at,omitempty"`
}

// IsActive returns true if the script is the active version.
func (s ScriptModule) IsActive() bool { return s.Status == StatusActive }

// scriptIndex is the persistent index of all versions.
type scriptIndex struct {
	ActiveID string         `json:"active_id"`
	Scripts  []ScriptModule `json:"scripts"`
}

// Repository provides script version storage.
type Repository struct {
	scriptDir string
}

// NewRepository creates a new script repository rooted at dataDir.
func NewRepository(dataDir string) *Repository {
	return &Repository{scriptDir: filepath.Join(dataDir, "scripts")}
}

func (r *Repository) indexPath() string        { return filepath.Join(r.scriptDir, "index.json") }
func (r *Repository) modulePath(id string) string { return filepath.Join(r.scriptDir, sanitizeVersion(id)+".json") }

// sanitizeVersion normalizes a version string to be filesystem-safe.
func sanitizeVersion(version string) string {
	version = scriptVersionRe.ReplaceAllString(version, "-")
	return strings.Trim(version, ".-_")
}

// generateVersion returns a timestamp-based default version.
func generateVersion() string {
	return time.Now().UTC().Format("20060102.150405")
}

// generateID creates a unique ID from version and timestamp.
func generateID(version string) string {
	return fmt.Sprintf("%s-%s", version, time.Now().UTC().Format("20060102T150405000Z"))
}

// readIndex loads the index from disk and migrates legacy data.
func (r *Repository) readIndex() (scriptIndex, error) {
	var idx scriptIndex
	data, err := os.ReadFile(r.indexPath())
	if err != nil {
		if os.IsNotExist(err) {
			// Migrate from legacy active.json if present.
			return r.migrateLegacyActive()
		}
		return idx, err
	}
	if err := json.Unmarshal(data, &idx); err != nil {
		return idx, err
	}
	// Migrate legacy entries that used the `active` boolean field.
	needsWrite := false
	for i := range idx.Scripts {
		if idx.Scripts[i].Status == "" {
			if idx.Scripts[i].ID == idx.ActiveID {
				idx.Scripts[i].Status = StatusActive
			} else {
				idx.Scripts[i].Status = StatusArchived
			}
			needsWrite = true
		}
	}
	if needsWrite {
		if err := r.writeIndex(idx); err != nil {
			return idx, err
		}
	}
	return idx, nil
}

// migrateLegacyActive imports a legacy active.json into the index format.
func (r *Repository) migrateLegacyActive() (scriptIndex, error) {
	var idx scriptIndex
	legacyPath := filepath.Join(r.scriptDir, "active.json")
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return idx, nil
		}
		return idx, err
	}
	var module ScriptModule
	if err := json.Unmarshal(data, &module); err != nil {
		return idx, err
	}
	if module.Content == "" || module.SHA256 == "" {
		return idx, nil
	}
	module.Status = StatusActive
	if module.CreatedAt.IsZero() {
		module.CreatedAt = module.UpdatedAt
	}
	if err := r.writeModule(module); err != nil {
		return idx, err
	}
	idx.ActiveID = module.ID
	idx.Scripts = []ScriptModule{module}
	if err := r.writeIndex(idx); err != nil {
		return idx, err
	}
	return idx, nil
}

// writeIndex persists the index atomically using temp file + rename.
func (r *Repository) writeIndex(idx scriptIndex) error {
	if err := os.MkdirAll(r.scriptDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(r.indexPath(), data, 0600)
}

// writeModule persists a single module file atomically.
func (r *Repository) writeModule(module ScriptModule) error {
	if err := os.MkdirAll(r.scriptDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(module, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(r.modulePath(module.ID), data, 0600)
}

// writeFileAtomic writes data to path via a temporary file and rename.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// List returns all script versions and the active ID.
// Content is preserved (not stripped) so the admin panel can display
// the full JS code for every version. Scripts with empty content in
// the index are backfilled from their module files on disk.
func (r *Repository) List() ([]ScriptModule, string, error) {
	idx, err := r.readIndex()
	if err != nil {
		return nil, "", err
	}
	// Backfill content from module files for scripts with empty content in the index.
	for i := range idx.Scripts {
		if idx.Scripts[i].Content == "" {
			if module, err := r.Get(idx.Scripts[i].ID); err == nil && module.Content != "" {
				idx.Scripts[i].Content = module.Content
				idx.Scripts[i].SHA256 = module.SHA256
				idx.Scripts[i].Size = module.Size
			}
		}
	}
	// Sort by updated_at descending.
	sort.Slice(idx.Scripts, func(a, b int) bool {
		return idx.Scripts[a].UpdatedAt.After(idx.Scripts[b].UpdatedAt)
	})
	return idx.Scripts, idx.ActiveID, nil
}

// Get returns a single script version with content.
func (r *Repository) Get(id string) (*ScriptModule, error) {
	data, err := os.ReadFile(r.modulePath(id))
	if err != nil {
		return nil, err
	}
	var module ScriptModule
	if err := json.Unmarshal(data, &module); err != nil {
		return nil, err
	}
	return &module, nil
}

// GetActive returns the active script content.
func (r *Repository) GetActive() (*ScriptModule, error) {
	idx, err := r.readIndex()
	if err != nil {
		return nil, err
	}
	if idx.ActiveID == "" {
		return nil, fmt.Errorf("no active script")
	}
	return r.Get(idx.ActiveID)
}

// computeSHA256 computes SHA256 of content.
func computeSHA256(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

// makeModule builds a ScriptModule from input without saving.
func makeModule(version, content, note string) ScriptModule {
	content = strings.TrimSpace(content)
	version = strings.TrimSpace(version)
	if version == "" {
		version = generateVersion()
	}
	version = sanitizeVersion(version)
	if version == "" {
		version = generateVersion()
	}
	now := time.Now().UTC()
	module := ScriptModule{
		ID:        generateID(version),
		Version:   version,
		SHA256:    computeSHA256(content),
		Content:   content,
		Size:      len([]byte(content)),
		Note:      strings.TrimSpace(note),
		Status:    StatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return module
}

// uniqueVersion checks if the version already exists and returns a unique one.
func (r *Repository) uniqueVersion(version string) string {
	idx, err := r.readIndex()
	if err != nil {
		return version
	}
	for {
		found := false
		for _, s := range idx.Scripts {
			if s.Version == version {
				found = true
				break
			}
		}
		if !found {
			break
		}
		version = version + "-" + time.Now().UTC().Format("150405")
	}
	return version
}

// Create saves a new draft version. Returns the saved module.
func (r *Repository) Create(version, content, note string) (*ScriptModule, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if len([]byte(content)) > maxScriptModuleBytes {
		return nil, fmt.Errorf("content exceeds %d bytes", maxScriptModuleBytes)
	}
	if strings.TrimSpace(version) != "" {
		version = sanitizeVersion(version)
	}
	// Auto-resolve version conflicts by appending a timestamp.
	version = r.uniqueVersion(version)
	module := makeModule(version, content, note)
	idx, err := r.readIndex()
	if err != nil {
		return nil, err
	}
	if err := r.writeModule(module); err != nil {
		return nil, err
	}
	idx.Scripts = append([]ScriptModule{module}, idx.Scripts...)
	if err := r.writeIndex(idx); err != nil {
		return nil, err
	}
	return &module, nil
}

// Activate sets a script version as active and archives the previous active.
func (r *Repository) Activate(id string) (*ScriptModule, error) {
	idx, err := r.readIndex()
	if err != nil {
		return nil, err
	}
	var target *ScriptModule
	for i := range idx.Scripts {
		if idx.Scripts[i].ID == id {
			target = &idx.Scripts[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("script not found")
	}
	if target.Status == StatusActive {
		return nil, fmt.Errorf("already active")
	}
	// Archive previous active — read from disk to preserve content.
	for i := range idx.Scripts {
		if idx.Scripts[i].Status == StatusActive {
			idx.Scripts[i].Status = StatusArchived
			idx.Scripts[i].UpdatedAt = time.Now().UTC()
			if existing, err := r.Get(idx.Scripts[i].ID); err == nil {
				// Preserve content/SHA256/Size from disk.
				existing.Status = StatusArchived
				existing.UpdatedAt = idx.Scripts[i].UpdatedAt
				if err := r.writeModule(*existing); err != nil {
					return nil, err
				}
			} else {
				if err := r.writeModule(idx.Scripts[i]); err != nil {
					return nil, err
				}
			}
		}
	}
	// Activate target — read from disk to preserve content.
	now := time.Now().UTC()
	idx.ActiveID = target.ID
	if existing, err := r.Get(target.ID); err == nil {
		existing.Status = StatusActive
		existing.ActivatedAt = now
		existing.UpdatedAt = now
		if err := r.writeModule(*existing); err != nil {
			return nil, err
		}
	} else {
		target.Status = StatusActive
		target.ActivatedAt = now
		target.UpdatedAt = now
		if err := r.writeModule(*target); err != nil {
			return nil, err
		}
	}
	target.Status = StatusActive
	target.ActivatedAt = now
	target.UpdatedAt = now
	if err := r.writeIndex(idx); err != nil {
		return nil, err
	}
	return r.Get(target.ID)
}

// Publish creates a new version and atomically activates it.
func (r *Repository) Publish(version, content, note string) (*ScriptModule, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	if len([]byte(content)) > maxScriptModuleBytes {
		return nil, fmt.Errorf("content exceeds %d bytes", maxScriptModuleBytes)
	}
	if strings.TrimSpace(version) != "" {
		version = sanitizeVersion(version)
	}
	version = r.uniqueVersion(version)
	module := makeModule(version, content, note)
	module.Status = StatusActive
	module.ActivatedAt = module.UpdatedAt

	idx, err := r.readIndex()
	if err != nil {
		return nil, err
	}
	// Archive previous active — read from disk to preserve content.
	for i := range idx.Scripts {
		if idx.Scripts[i].Status == StatusActive {
			idx.Scripts[i].Status = StatusArchived
			idx.Scripts[i].UpdatedAt = time.Now().UTC()
			if existing, err := r.Get(idx.Scripts[i].ID); err == nil {
				existing.Status = StatusArchived
				existing.UpdatedAt = idx.Scripts[i].UpdatedAt
				if err := r.writeModule(*existing); err != nil {
					return nil, err
				}
			} else {
				if err := r.writeModule(idx.Scripts[i]); err != nil {
					return nil, err
				}
			}
		}
	}
	if err := r.writeModule(module); err != nil {
		return nil, err
	}
	idx.ActiveID = module.ID
	idx.Scripts = append([]ScriptModule{module}, idx.Scripts...)
	if err := r.writeIndex(idx); err != nil {
		return nil, err
	}
	return &module, nil
}

// Delete removes a non-active script version.
func (r *Repository) Delete(id string) error {
	idx, err := r.readIndex()
	if err != nil {
		return err
	}
	if idx.ActiveID == id {
		return fmt.Errorf("cannot delete active script")
	}
	found := false
	filtered := make([]ScriptModule, 0, len(idx.Scripts))
	for _, s := range idx.Scripts {
		if s.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, s)
	}
	if !found {
		return fmt.Errorf("script not found")
	}
	idx.Scripts = filtered
	if err := r.writeIndex(idx); err != nil {
		return err
	}
	if err := os.Remove(r.modulePath(id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
