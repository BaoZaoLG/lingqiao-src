package updates

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lingqiao/server/internal/storage"
)

type UpdateInfo struct {
	Version    string    `json:"version"`
	Filename   string    `json:"filename"`
	FileSize   int64     `json:"file_size"`
	SHA256     string    `json:"sha256"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type MetadataStore struct {
	store storage.Store
	mu    sync.RWMutex
	info  *UpdateInfo
}

const metadataKey = "info"

func NewMetadataStore(store storage.Store) *MetadataStore {
	return &MetadataStore{store: store}
}

func ValidateVersion(version string) error {
	if version == "" {
		return fmt.Errorf("版本号不能为空")
	}
	for _, c := range version {
		if !((c >= '0' && c <= '9') || c == '.') {
			return fmt.Errorf("版本号格式无效")
		}
	}
	return nil
}

func SafeFilename(version string, ext string) (string, error) {
	if err := ValidateVersion(version); err != nil {
		return "", err
	}
	ext = strings.ToLower(ext)
	if ext != ".exe" {
		return "", fmt.Errorf("只支持 .exe 文件")
	}
	return fmt.Sprintf("Injector_v%s%s", version, ext), nil
}

func (s *MetadataStore) Save(info UpdateInfo) error {
	if err := s.store.Save(metadataKey, info); err != nil {
		return err
	}
	s.mu.Lock()
	cp := info
	s.info = &cp
	s.mu.Unlock()
	return nil
}

func (s *MetadataStore) Load() (*UpdateInfo, error) {
	s.mu.RLock()
	if s.info != nil {
		cp := *s.info
		s.mu.RUnlock()
		return &cp, nil
	}
	s.mu.RUnlock()

	var info UpdateInfo
	if err := s.store.Load(metadataKey, &info); err != nil {
		return nil, err
	}
	if info.Version == "" {
		return nil, nil
	}
	s.mu.Lock()
	s.info = &info
	s.mu.Unlock()
	cp := info
	return &cp, nil
}

func (s *MetadataStore) SHAForVersion(version string) string {
	info, err := s.Load()
	if err != nil || info == nil {
		return ""
	}
	if version == "" || info.Version == version {
		return info.SHA256
	}
	return ""
}
