package releases

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore ...
type SQLiteStore struct {
	db *sql.DB
}

// OpenSQLiteStore ...
func OpenSQLiteStore(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	store := &SQLiteStore{db: db}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// Close ...
func (s *SQLiteStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteStore) migrate(ctx context.Context) error {
	stmts := []string{
		`PRAGMA foreign_keys = ON`,
		`PRAGMA busy_timeout = 5000`,
		`CREATE TABLE IF NOT EXISTS releases (
			id TEXT PRIMARY KEY,
			version TEXT NOT NULL,
			channel TEXT NOT NULL,
			status TEXT NOT NULL,
			min_version TEXT NOT NULL DEFAULT '',
			force_update INTEGER NOT NULL DEFAULT 0,
			rollout_percent INTEGER NOT NULL DEFAULT 100,
			notes TEXT NOT NULL DEFAULT '',
			targeting_json TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			published_at TEXT NOT NULL DEFAULT '',
			paused_at TEXT NOT NULL DEFAULT '',
			rolled_back_at TEXT NOT NULL DEFAULT '',
			rolled_back_to TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_releases_channel_status ON releases(channel, status)`,
		`CREATE TABLE IF NOT EXISTS release_packages (
			id TEXT PRIMARY KEY,
			release_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			filename TEXT NOT NULL,
			path TEXT NOT NULL,
			file_size INTEGER NOT NULL,
			sha256 TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(release_id) REFERENCES releases(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_release_packages_release ON release_packages(release_id)`,
		`CREATE TABLE IF NOT EXISTS release_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			release_id TEXT NOT NULL,
			version TEXT NOT NULL,
			machine_id TEXT NOT NULL,
			card_code TEXT NOT NULL DEFAULT '',
			agent_id TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL,
			error_code TEXT NOT NULL DEFAULT '',
			detail TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_release_events_release ON release_events(release_id, type)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// SaveRelease ...
func (s *SQLiteStore) SaveRelease(ctx context.Context, release Release) error {
	if release.ID == "" || release.Version == "" {
		return fmt.Errorf("release id and version are required")
	}
	if release.Channel == "" {
		release.Channel = ChannelStable
	}
	if release.Status == "" {
		release.Status = StatusDraft
	}
	if release.RolloutPercent < 0 {
		release.RolloutPercent = 0
	}
	if release.RolloutPercent > 100 {
		release.RolloutPercent = 100
	}
	if release.CreatedAt.IsZero() {
		release.CreatedAt = time.Now().UTC()
	}
	targeting, err := json.Marshal(release.Targeting)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO releases (
		id, version, channel, status, min_version, force_update, rollout_percent, notes,
		targeting_json, created_at, published_at, paused_at, rolled_back_at, rolled_back_to
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		version = excluded.version,
		channel = excluded.channel,
		status = excluded.status,
		min_version = excluded.min_version,
		force_update = excluded.force_update,
		rollout_percent = excluded.rollout_percent,
		notes = excluded.notes,
		targeting_json = excluded.targeting_json,
		created_at = excluded.created_at,
		published_at = excluded.published_at,
		paused_at = excluded.paused_at,
		rolled_back_at = excluded.rolled_back_at,
		rolled_back_to = excluded.rolled_back_to`,
		release.ID, release.Version, string(release.Channel), string(release.Status),
		release.MinVersion, boolInt(release.ForceUpdate), release.RolloutPercent, release.Notes,
		string(targeting), formatTime(release.CreatedAt), formatTimePtr(release.PublishedAt),
		formatTimePtr(release.PausedAt), formatTimePtr(release.RolledBackAt), release.RolledBackTo)
	return err
}

// GetRelease ...
func (s *SQLiteStore) GetRelease(ctx context.Context, id string) (Release, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, version, channel, status, min_version, force_update,
		rollout_percent, notes, targeting_json, created_at, published_at, paused_at, rolled_back_at, rolled_back_to
		FROM releases WHERE id = ?`, id)
	return scanRelease(row)
}

// ListReleases ...
func (s *SQLiteStore) ListReleases(ctx context.Context) ([]Release, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, version, channel, status, min_version, force_update,
		rollout_percent, notes, targeting_json, created_at, published_at, paused_at, rolled_back_at, rolled_back_to
		FROM releases ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReleases(rows)
}

// ListPublishedReleases ...
func (s *SQLiteStore) ListPublishedReleases(ctx context.Context, channel Channel) ([]Release, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, version, channel, status, min_version, force_update,
		rollout_percent, notes, targeting_json, created_at, published_at, paused_at, rolled_back_at, rolled_back_to
		FROM releases WHERE channel = ? AND status = ? ORDER BY published_at DESC, created_at DESC`,
		string(channel), string(StatusPublished))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReleases(rows)
}

// SavePackage ...
func (s *SQLiteStore) SavePackage(ctx context.Context, pkg ReleasePackage) error {
	if pkg.ID == "" || pkg.ReleaseID == "" || pkg.Filename == "" || pkg.Path == "" {
		return fmt.Errorf("package id, release id, filename, and path are required")
	}
	if pkg.Kind == "" {
		pkg.Kind = PackageKindBundle
	}
	if pkg.CreatedAt.IsZero() {
		pkg.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO release_packages (
		id, release_id, kind, filename, path, file_size, sha256, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(id) DO UPDATE SET
		release_id = excluded.release_id,
		kind = excluded.kind,
		filename = excluded.filename,
		path = excluded.path,
		file_size = excluded.file_size,
		sha256 = excluded.sha256,
		created_at = excluded.created_at`,
		pkg.ID, pkg.ReleaseID, string(pkg.Kind), pkg.Filename, pkg.Path, pkg.FileSize, pkg.SHA256, formatTime(pkg.CreatedAt))
	return err
}

// GetPackage ...
func (s *SQLiteStore) GetPackage(ctx context.Context, id string) (ReleasePackage, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, release_id, kind, filename, path, file_size, sha256, created_at
		FROM release_packages WHERE id = ?`, id)
	return scanPackage(row)
}

// ListPackages ...
func (s *SQLiteStore) ListPackages(ctx context.Context, releaseID string) ([]ReleasePackage, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, release_id, kind, filename, path, file_size, sha256, created_at
		FROM release_packages WHERE release_id = ? ORDER BY created_at DESC`, releaseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var packages []ReleasePackage
	for rows.Next() {
		pkg, err := scanPackage(rows)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkg)
	}
	return packages, rows.Err()
}

// RecordEvent ...
func (s *SQLiteStore) RecordEvent(ctx context.Context, event ReleaseEvent) error {
	if event.ReleaseID == "" || event.Type == "" {
		return fmt.Errorf("release id and event type are required")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO release_events (
		release_id, version, machine_id, card_code, agent_id, type, error_code, detail, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ReleaseID, event.Version, event.MachineID, event.CardCode, event.AgentID,
		string(event.Type), event.ErrorCode, event.Detail, formatTime(event.CreatedAt))
	return err
}

// ListEvents ...
func (s *SQLiteStore) ListEvents(ctx context.Context, releaseID string, limit int) ([]ReleaseEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, release_id, version, machine_id, card_code, agent_id, type, error_code, detail, created_at
		FROM release_events WHERE release_id = ? ORDER BY id DESC LIMIT ?`, releaseID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []ReleaseEvent
	for rows.Next() {
		var event ReleaseEvent
		var typ string
		var createdAt string
		if err := rows.Scan(&event.ID, &event.ReleaseID, &event.Version, &event.MachineID, &event.CardCode, &event.AgentID, &typ, &event.ErrorCode, &event.Detail, &createdAt); err != nil {
			return nil, err
		}
		event.Type = EventType(typ)
		event.CreatedAt = parseTime(createdAt)
		events = append(events, event)
	}
	return events, rows.Err()
}

// ReleaseMetrics ...
func (s *SQLiteStore) ReleaseMetrics(ctx context.Context, releaseID string) (ReleaseMetrics, error) {
	metrics := ReleaseMetrics{ReleaseID: releaseID}
	rows, err := s.db.QueryContext(ctx, `SELECT type, COUNT(*) FROM release_events WHERE release_id = ? GROUP BY type`, releaseID)
	if err != nil {
		return metrics, err
	}
	defer rows.Close()
	for rows.Next() {
		var typ string
		var count int
		if err := rows.Scan(&typ, &count); err != nil {
			return metrics, err
		}
		switch EventType(typ) {
		case EventOffered:
			metrics.Offered = count
		case EventDownloadStarted:
			metrics.DownloadStarted = count
		case EventDownloadFailed:
			metrics.DownloadFailed = count
		case EventDownloadDone:
			metrics.DownloadDone = count
		case EventInstallStarted:
			metrics.InstallStarted = count
		case EventInstallSuccess:
			metrics.InstallSuccess = count
		case EventInstallFailed:
			metrics.InstallFailed = count
		case EventRollback:
			metrics.Rollback = count
		case EventDismissed:
			metrics.Dismissed = count
		}
	}
	if err := rows.Err(); err != nil {
		return metrics, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT machine_id) FROM release_events WHERE release_id = ? AND machine_id != ''`, releaseID).Scan(&metrics.UniqueMachines); err != nil {
		return metrics, err
	}
	return metrics, nil
}

type releaseScanner interface {
	Scan(dest ...any) error
}

func scanRelease(row releaseScanner) (Release, error) {
	var release Release
	var channel, status, targeting, createdAt, publishedAt, pausedAt, rolledBackAt string
	var force int
	if err := row.Scan(&release.ID, &release.Version, &channel, &status, &release.MinVersion, &force,
		&release.RolloutPercent, &release.Notes, &targeting, &createdAt, &publishedAt, &pausedAt,
		&rolledBackAt, &release.RolledBackTo); err != nil {
		return release, err
	}
	release.Channel = Channel(channel)
	release.Status = ReleaseStatus(status)
	release.ForceUpdate = force != 0
	release.CreatedAt = parseTime(createdAt)
	release.PublishedAt = parseTimePtr(publishedAt)
	release.PausedAt = parseTimePtr(pausedAt)
	release.RolledBackAt = parseTimePtr(rolledBackAt)
	_ = json.Unmarshal([]byte(targeting), &release.Targeting)
	return release, nil
}

func scanReleases(rows *sql.Rows) ([]Release, error) {
	var releases []Release
	for rows.Next() {
		release, err := scanRelease(rows)
		if err != nil {
			return nil, err
		}
		releases = append(releases, release)
	}
	return releases, rows.Err()
}

type packageScanner interface {
	Scan(dest ...any) error
}

func scanPackage(row packageScanner) (ReleasePackage, error) {
	var pkg ReleasePackage
	var kind, createdAt string
	if err := row.Scan(&pkg.ID, &pkg.ReleaseID, &kind, &pkg.Filename, &pkg.Path, &pkg.FileSize, &pkg.SHA256, &createdAt); err != nil {
		return pkg, err
	}
	pkg.Kind = PackageKind(kind)
	pkg.CreatedAt = parseTime(createdAt)
	return pkg, nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return formatTime(*t)
}

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, _ := time.Parse(time.RFC3339Nano, value)
	return t
}

func parseTimePtr(value string) *time.Time {
	t := parseTime(value)
	if t.IsZero() {
		return nil
	}
	return &t
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
