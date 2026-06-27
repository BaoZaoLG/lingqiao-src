package releases

import "time"
// Channel ...
type Channel string

const (
	ChannelStable Channel = "stable"
	ChannelBeta   Channel = "beta"
	ChannelCanary Channel = "canary"
)
// ReleaseStatus ...
type ReleaseStatus string

const (
	StatusDraft      ReleaseStatus = "draft"
	StatusPublished  ReleaseStatus = "published"
	StatusPaused     ReleaseStatus = "paused"
	StatusRolledBack ReleaseStatus = "rolled_back"
)
// PackageKind ...
type PackageKind string

const (
	PackageKindBundle PackageKind = "bundle"
	PackageKindMSI    PackageKind = "msi"
)
// TargetingRules ...
type TargetingRules struct {
	AllowCards    []string `json:"allow_cards,omitempty"`
	DenyCards     []string `json:"deny_cards,omitempty"`
	AllowMachines []string `json:"allow_machines,omitempty"`
	DenyMachines  []string `json:"deny_machines,omitempty"`
	AllowAgents   []string `json:"allow_agents,omitempty"`
	DenyAgents    []string `json:"deny_agents,omitempty"`
}
// Release ...
type Release struct {
	ID             string         `json:"id"`
	Version        string         `json:"version"`
	Channel        Channel        `json:"channel"`
	Status         ReleaseStatus  `json:"status"`
	MinVersion     string         `json:"min_version,omitempty"`
	ForceUpdate    bool           `json:"force_update"`
	RolloutPercent int            `json:"rollout_percent"`
	Notes          string         `json:"notes,omitempty"`
	Targeting      TargetingRules `json:"targeting"`
	CreatedAt      time.Time      `json:"created_at"`
	PublishedAt    *time.Time     `json:"published_at,omitempty"`
	PausedAt       *time.Time     `json:"paused_at,omitempty"`
	RolledBackAt   *time.Time     `json:"rolled_back_at,omitempty"`
	RolledBackTo   string         `json:"rolled_back_to,omitempty"`
}
// ReleasePackage ...
type ReleasePackage struct {
	ID        string      `json:"id"`
	ReleaseID string      `json:"release_id"`
	Kind      PackageKind `json:"kind"`
	Filename  string      `json:"filename"`
	Path      string      `json:"path"`
	FileSize  int64       `json:"file_size"`
	SHA256    string      `json:"sha256"`
	CreatedAt time.Time   `json:"created_at"`
}
// ClientContext ...
type ClientContext struct {
	Version   string  `json:"version"`
	Channel   Channel `json:"channel"`
	MachineID string  `json:"machine_id"`
	CardCode  string  `json:"card_code"`
	AgentID   string  `json:"agent_id"`
}
// SelectedRelease ...
type SelectedRelease struct {
	Release Release        `json:"release"`
	Package ReleasePackage `json:"package"`
}
// Manifest ...
type Manifest struct {
	ReleaseID      string    `json:"release_id"`
	Version        string    `json:"version"`
	Channel        Channel   `json:"channel"`
	MinVersion     string    `json:"min_version,omitempty"`
	ForceUpdate    bool      `json:"force_update"`
	ReleaseNotes   string    `json:"release_notes,omitempty"`
	PackageID      string    `json:"package_id"`
	PackageKind    string    `json:"package_kind,omitempty"`
	PackageURL     string    `json:"package_url"`
	PackageSize    int64     `json:"package_size"`
	PackageSHA256  string    `json:"package_sha256"`
	RolloutPercent int       `json:"rollout_percent"`
	CreatedAt      time.Time `json:"created_at"`
}
// SignedManifest ...
type SignedManifest struct {
	Manifest  Manifest `json:"manifest"`
	Signature string   `json:"signature"`
}
// EventType ...
type EventType string

const (
	EventOffered         EventType = "offered"
	EventDownloadStarted EventType = "download_started"
	EventDownloadFailed  EventType = "download_failed"
	EventDownloadDone    EventType = "download_completed"
	EventInstallStarted  EventType = "install_started"
	EventInstallSuccess  EventType = "install_success"
	EventInstallFailed   EventType = "install_failed"
	EventRollback        EventType = "rollback"
	EventDismissed       EventType = "dismissed"
)
// ReleaseEvent ...
type ReleaseEvent struct {
	ID        int64     `json:"id"`
	ReleaseID string    `json:"release_id"`
	Version   string    `json:"version"`
	MachineID string    `json:"machine_id"`
	CardCode  string    `json:"card_code,omitempty"`
	AgentID   string    `json:"agent_id,omitempty"`
	Type      EventType `json:"type"`
	ErrorCode string    `json:"error_code,omitempty"`
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
// ReleaseMetrics ...
type ReleaseMetrics struct {
	ReleaseID       string `json:"release_id"`
	Offered         int    `json:"offered"`
	DownloadStarted int    `json:"download_started"`
	DownloadFailed  int    `json:"download_failed"`
	DownloadDone    int    `json:"download_completed"`
	InstallStarted  int    `json:"install_started"`
	InstallSuccess  int    `json:"install_success"`
	InstallFailed   int    `json:"install_failed"`
	Rollback        int    `json:"rollback"`
	Dismissed       int    `json:"dismissed"`
	UniqueMachines  int    `json:"unique_machines"`
}
