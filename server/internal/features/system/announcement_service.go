package system

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Announcement struct {
	ID          string     `json:"id,omitempty"`
	Content     string     `json:"content"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Status      string     `json:"status,omitempty"`
	CreatedAt   time.Time  `json:"created_at,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

const (
	AnnouncementStatusDraft      = "draft"
	AnnouncementStatusPublished  = "published"
	AnnouncementStatusSuperseded = "superseded"
	maxAnnouncementBytes         = 64 << 10
)

type announcementIndex struct {
	ActiveID      string         `json:"active_id"`
	Announcements []Announcement `json:"announcements"`
}

type AnnouncementService struct {
	dataDir      string
	announcement Announcement
	annMu        sync.RWMutex
	indexMu      sync.Mutex
}

func NewAnnouncementService(dataDir string) *AnnouncementService {
	s := &AnnouncementService{
		dataDir: dataDir,
	}
	s.init()
	return s
}

func (s *AnnouncementService) dir() string {
	return filepath.Join(s.dataDir, "announcements")
}

func (s *AnnouncementService) indexPath() string {
	return filepath.Join(s.dir(), "index.json")
}

func (s *AnnouncementService) legacyPath() string {
	return filepath.Join(s.dataDir, "announcement.json")
}

func (s *AnnouncementService) Get() *Announcement {
	s.annMu.RLock()
	defer s.annMu.RUnlock()
	if s.announcement.Content == "" {
		return nil
	}
	a := s.announcement
	return &a
}

func (s *AnnouncementService) Set(content string) error {
	_, err := s.SaveRevision(content, true)
	return err
}

func (s *AnnouncementService) SaveRevision(content string, publish bool) (*Announcement, error) {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()

	if len([]byte(content)) > maxAnnouncementBytes {
		return nil, fmt.Errorf("content exceeds %d bytes", maxAnnouncementBytes)
	}
	now := time.Now().UTC()
	status := AnnouncementStatusDraft
	var publishedAt *time.Time
	if publish {
		status = AnnouncementStatusPublished
		publishedAt = &now
	}
	a := Announcement{
		ID:          s.newID(now),
		Content:     content,
		UpdatedAt:   now,
		Status:      status,
		CreatedAt:   now,
		PublishedAt: publishedAt,
	}
	idx, err := s.readIndex()
	if err != nil {
		return nil, err
	}
	idx.Announcements = append([]Announcement{a}, idx.Announcements...)
	if publish {
		idx.ActiveID = a.ID
		s.markActive(&idx, a.ID)
	}
	if err := s.writeIndex(idx); err != nil {
		return nil, err
	}
	if publish {
		s.setActive(a)
	}
	return &a, nil
}

func (s *AnnouncementService) PublishRevision(id string) (*Announcement, error) {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()

	idx, err := s.readIndex()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var selected *Announcement
	for i := range idx.Announcements {
		if idx.Announcements[i].ID == id {
			idx.Announcements[i].Status = AnnouncementStatusPublished
			idx.Announcements[i].UpdatedAt = now
			idx.Announcements[i].PublishedAt = &now
			selected = &idx.Announcements[i]
			break
		}
	}
	if selected == nil {
		return nil, fmt.Errorf("announcement revision not found")
	}
	idx.ActiveID = id
	s.markActive(&idx, id)
	if err := s.writeIndex(idx); err != nil {
		return nil, err
	}
	s.setActive(*selected)
	return selected, nil
}

func (s *AnnouncementService) DeleteRevision(id string) error {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()

	idx, err := s.readIndex()
	if err != nil {
		return err
	}
	if idx.ActiveID == id {
		return fmt.Errorf("cannot delete active announcement")
	}
	next := make([]Announcement, 0, len(idx.Announcements))
	found := false
	for _, item := range idx.Announcements {
		if item.ID == id {
			found = true
			continue
		}
		next = append(next, item)
	}
	if !found {
		return fmt.Errorf("announcement revision not found")
	}
	idx.Announcements = next
	return s.writeIndex(idx)
}

func (s *AnnouncementService) ListRevisions() ([]Announcement, string, error) {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()

	idx, err := s.readIndex()
	if err != nil {
		return nil, "", err
	}
	return idx.Announcements, idx.ActiveID, nil
}

func (s *AnnouncementService) setActive(a Announcement) {
	s.annMu.Lock()
	s.announcement = a
	s.annMu.Unlock()
	data, _ := json.Marshal(s.announcement)
	os.WriteFile(s.legacyPath(), data, 0600)
}

func (s *AnnouncementService) init() {
	if idx, err := s.readIndex(); err == nil {
		for _, a := range idx.Announcements {
			if a.ID == idx.ActiveID {
				s.setActive(a)
				return
			}
		}
	}
	data, err := os.ReadFile(s.legacyPath())
	if err != nil {
		return
	}
	var a Announcement
	if err := json.Unmarshal(data, &a); err != nil {
		return
	}
	s.annMu.Lock()
	s.announcement = a
	s.annMu.Unlock()
}

func (s *AnnouncementService) readIndex() (announcementIndex, error) {
	if err := s.migrateLegacy(); err != nil {
		return announcementIndex{}, err
	}
	var idx announcementIndex
	data, err := os.ReadFile(s.indexPath())
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

func (s *AnnouncementService) writeIndex(idx announcementIndex) error {
	if err := os.MkdirAll(s.dir(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.indexPath(), data, 0600)
}

func (s *AnnouncementService) migrateLegacy() error {
	if _, err := os.Stat(s.indexPath()); err == nil {
		return nil
	}
	data, err := os.ReadFile(s.legacyPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	_ = os.WriteFile(filepath.Join(s.dataDir, "announcement.legacy.json"), data, 0600)
	var a Announcement
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	if a.Content == "" {
		return nil
	}
	now := a.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if a.ID == "" {
		a.ID = s.newID(now)
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	if a.PublishedAt == nil {
		t := now
		a.PublishedAt = &t
	}
	a.Status = AnnouncementStatusPublished
	return s.writeIndex(announcementIndex{ActiveID: a.ID, Announcements: []Announcement{a}})
}

func (s *AnnouncementService) newID(t time.Time) string {
	return fmt.Sprintf("ann-%s-%d", t.Format("20060102T150405000Z"), t.UnixNano())
}

func (s *AnnouncementService) markActive(idx *announcementIndex, activeID string) {
	for i := range idx.Announcements {
		switch {
		case idx.Announcements[i].ID == activeID:
			idx.Announcements[i].Status = AnnouncementStatusPublished
		case idx.Announcements[i].Status == AnnouncementStatusPublished:
			idx.Announcements[i].Status = AnnouncementStatusSuperseded
		}
	}
}
