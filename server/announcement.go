package main

import (
	"sync"

	"github.com/lingqiao/server/internal/features/system"
)

type Announcement = system.Announcement

var (
	announcementServiceMu sync.RWMutex
	announcementService   *system.AnnouncementService
)

func currentAnnouncementService() *system.AnnouncementService {
	announcementServiceMu.RLock()
	svc := announcementService
	announcementServiceMu.RUnlock()
	if svc != nil {
		return svc
	}

	candidate := system.NewAnnouncementService(dataDir())
	announcementServiceMu.Lock()
	if announcementService == nil {
		announcementService = candidate
	}
	svc = announcementService
	announcementServiceMu.Unlock()
	return svc
}

func GetAnnouncement() *Announcement {
	return currentAnnouncementService().Get()
}

func SetAnnouncement(content string) {
	_ = currentAnnouncementService().Set(content)
}

func SaveAnnouncementRevision(content string, publish bool) (*Announcement, error) {
	return currentAnnouncementService().SaveRevision(content, publish)
}

func PublishAnnouncementRevision(id string) (*Announcement, error) {
	return currentAnnouncementService().PublishRevision(id)
}

func DeleteAnnouncementRevision(id string) error {
	return currentAnnouncementService().DeleteRevision(id)
}

func ListAnnouncementRevisions() ([]Announcement, string, error) {
	return currentAnnouncementService().ListRevisions()
}

func initAnnouncement() {
	svc := system.NewAnnouncementService(dataDir())
	announcementServiceMu.Lock()
	announcementService = svc
	announcementServiceMu.Unlock()
}
