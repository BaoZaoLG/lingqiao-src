package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Announcement represents a public announcement with optional legacy version push.
type Announcement struct {
	ID            string     `json:"id,omitempty"`
	Content       string     `json:"content"`
	UpdatedAt     time.Time  `json:"updated_at"`
	LatestVersion string     `json:"latest_version"`
	MinVersion    string     `json:"min_version"`
	ForceUpdate   bool       `json:"force_update"`
	DownloadURL   string     `json:"download_url,omitempty"`
	SHA256        string     `json:"sha256,omitempty"`
	Status        string     `json:"status,omitempty"`
	CreatedAt     time.Time  `json:"created_at,omitempty"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
}

var announcement Announcement
var annMu sync.RWMutex

const (
	announcementStatusDraft      = "draft"
	announcementStatusPublished  = "published"
	announcementStatusSuperseded = "superseded"
	maxAnnouncementBytes         = 64 << 10
)

type announcementIndex struct {
	ActiveID      string         `json:"active_id"`
	Announcements []Announcement `json:"announcements"`
}

func announcementDir() string { return dataPath("announcements") }

func announcementIndexPath() string { return dataPath("announcements", "index.json") }

func GetAnnouncement() *Announcement {
	annMu.RLock()
	if announcement.Content == "" && announcement.LatestVersion == "" {
		annMu.RUnlock()
		return nil
	}
	a := announcement
	annMu.RUnlock()
	if a.SHA256 == "" && a.LatestVersion != "" {
		a.SHA256 = getUpdateSHA256(a.LatestVersion)
	}
	return &a
}

func SetAnnouncement(content, latestVersion, minVersion string, forceUpdate bool) {
	a, err := SaveAnnouncementRevision(content, latestVersion, minVersion, forceUpdate, true)
	if err != nil {
		return
	}
	setActiveAnnouncement(*a)
}

func SaveAnnouncementRevision(content, latestVersion, minVersion string, forceUpdate, publish bool) (*Announcement, error) {
	if len([]byte(content)) > maxAnnouncementBytes {
		return nil, fmt.Errorf("content exceeds %d bytes", maxAnnouncementBytes)
	}
	now := time.Now().UTC()
	status := announcementStatusDraft
	var publishedAt *time.Time
	if publish {
		status = announcementStatusPublished
		publishedAt = &now
	}
	dlURL := ""
	if latestVersion != "" {
		dlURL = "/admin/api/update/download"
	}
	a := Announcement{
		ID:            newAnnouncementID(now),
		Content:       content,
		UpdatedAt:     now,
		LatestVersion: latestVersion,
		MinVersion:    minVersion,
		ForceUpdate:   forceUpdate,
		DownloadURL:   dlURL,
		Status:        status,
		CreatedAt:     now,
		PublishedAt:   publishedAt,
	}
	idx, err := readAnnouncementIndex()
	if err != nil {
		return nil, err
	}
	idx.Announcements = append([]Announcement{a}, idx.Announcements...)
	if publish {
		idx.ActiveID = a.ID
		markActiveAnnouncement(&idx, a.ID)
	}
	if err := writeAnnouncementIndex(idx); err != nil {
		return nil, err
	}
	if publish {
		setActiveAnnouncement(a)
	}
	return &a, nil
}

func PublishAnnouncementRevision(id string) (*Announcement, error) {
	idx, err := readAnnouncementIndex()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var selected *Announcement
	for i := range idx.Announcements {
		if idx.Announcements[i].ID == id {
			idx.Announcements[i].Status = announcementStatusPublished
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
	markActiveAnnouncement(&idx, id)
	if err := writeAnnouncementIndex(idx); err != nil {
		return nil, err
	}
	setActiveAnnouncement(*selected)
	return selected, nil
}

func DeleteAnnouncementRevision(id string) error {
	idx, err := readAnnouncementIndex()
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
	return writeAnnouncementIndex(idx)
}

func ListAnnouncementRevisions() ([]Announcement, string, error) {
	idx, err := readAnnouncementIndex()
	if err != nil {
		return nil, "", err
	}
	return idx.Announcements, idx.ActiveID, nil
}

func setActiveAnnouncement(a Announcement) {
	annMu.Lock()
	if a.LatestVersion != "" && a.DownloadURL == "" {
		a.DownloadURL = "/admin/api/update/download"
	}
	announcement = a
	annMu.Unlock()
	data, _ := json.Marshal(announcement)
	os.WriteFile(dataPath("announcement.json"), data, 0600)
}

func initAnnouncement() {
	annMu.Lock()
	announcement = Announcement{}
	annMu.Unlock()

	if idx, err := readAnnouncementIndex(); err == nil {
		for _, a := range idx.Announcements {
			if a.ID == idx.ActiveID {
				setActiveAnnouncement(a)
				return
			}
		}
	}
	data, err := os.ReadFile(dataPath("announcement.json"))
	if err != nil {
		return
	}
	var a Announcement
	if err := json.Unmarshal(data, &a); err != nil {
		return
	}
	if a.LatestVersion != "" && a.DownloadURL == "" {
		a.DownloadURL = "/admin/api/update/download"
	}
	annMu.Lock()
	announcement = a
	annMu.Unlock()
}

func readAnnouncementIndex() (announcementIndex, error) {
	if err := migrateLegacyAnnouncement(); err != nil {
		return announcementIndex{}, err
	}
	var idx announcementIndex
	data, err := os.ReadFile(announcementIndexPath())
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

func writeAnnouncementIndex(idx announcementIndex) error {
	if err := os.MkdirAll(announcementDir(), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(announcementIndexPath(), data, 0600)
}

func migrateLegacyAnnouncement() error {
	if _, err := os.Stat(announcementIndexPath()); err == nil {
		return nil
	}
	data, err := os.ReadFile(dataPath("announcement.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var a Announcement
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	if a.Content == "" && a.LatestVersion == "" {
		return nil
	}
	now := a.UpdatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if a.ID == "" {
		a.ID = newAnnouncementID(now)
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = now
	}
	if a.PublishedAt == nil {
		t := now
		a.PublishedAt = &t
	}
	a.Status = announcementStatusPublished
	return writeAnnouncementIndex(announcementIndex{ActiveID: a.ID, Announcements: []Announcement{a}})
}

func newAnnouncementID(t time.Time) string {
	return fmt.Sprintf("ann-%s-%d", t.Format("20060102T150405000Z"), t.UnixNano())
}

func markActiveAnnouncement(idx *announcementIndex, activeID string) {
	for i := range idx.Announcements {
		switch {
		case idx.Announcements[i].ID == activeID:
			idx.Announcements[i].Status = announcementStatusPublished
		case idx.Announcements[i].Status == announcementStatusPublished:
			idx.Announcements[i].Status = announcementStatusSuperseded
		}
	}
}
