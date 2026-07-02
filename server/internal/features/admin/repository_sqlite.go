package admin

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrAdminNotFound = errors.New("admin not found")
)

type RepositorySQLite struct {
	db *sql.DB
}

func NewRepositorySQLite(db *sql.DB) (*RepositorySQLite, error) {
	r := &RepositorySQLite{db: db}
	if err := r.migrate(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *RepositorySQLite) migrate() error {
	ctx := context.Background()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS admin_users (
			id TEXT PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL,
			disabled INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			last_login_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS admin_sessions (
			token_hash TEXT PRIMARY KEY,
			admin_id TEXT NOT NULL,
			created_at TEXT NOT NULL,
			expires_at TEXT NOT NULL,
			last_seen_at TEXT,
			remote_addr TEXT,
			user_agent TEXT,
			FOREIGN KEY(admin_id) REFERENCES admin_users(id) ON DELETE CASCADE
		)`,
	}
	for _, stmt := range stmts {
		if _, err := r.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (r *RepositorySQLite) CountAdmins(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_users`).Scan(&count)
	return count, err
}

func (r *RepositorySQLite) CreateAdmin(ctx context.Context, admin *AdminUser) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO admin_users (
		id, username, password_hash, role, disabled, created_at, last_login_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		admin.ID, admin.Username, admin.PasswordHash, admin.Role, admin.Disabled,
		admin.CreatedAt.Format(time.RFC3339),
		formatTimePtr(admin.LastLoginAt))
	return err
}

func (r *RepositorySQLite) GetAdmin(ctx context.Context, id string) (*AdminUser, error) {
	var a AdminUser
	var disabled int
	var createdAt string
	var lastLoginAt sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT id, username, password_hash, role, disabled, created_at, last_login_at
		FROM admin_users WHERE id = ?`, id).Scan(
		&a.ID, &a.Username, &a.PasswordHash, &a.Role, &disabled, &createdAt, &lastLoginAt)
	if err == sql.ErrNoRows {
		return nil, ErrAdminNotFound
	}
	if err != nil {
		return nil, err
	}
	a.Disabled = disabled == 1
	a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if lastLoginAt.Valid {
		t, _ := time.Parse(time.RFC3339, lastLoginAt.String)
		a.LastLoginAt = &t
	}
	return &a, nil
}

func (r *RepositorySQLite) GetAdminByUsername(ctx context.Context, username string) (*AdminUser, error) {
	var a AdminUser
	var disabled int
	var createdAt string
	var lastLoginAt sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT id, username, password_hash, role, disabled, created_at, last_login_at
		FROM admin_users WHERE username = ?`, username).Scan(
		&a.ID, &a.Username, &a.PasswordHash, &a.Role, &disabled, &createdAt, &lastLoginAt)
	if err == sql.ErrNoRows {
		return nil, ErrAdminNotFound
	}
	if err != nil {
		return nil, err
	}
	a.Disabled = disabled == 1
	a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if lastLoginAt.Valid {
		t, _ := time.Parse(time.RFC3339, lastLoginAt.String)
		a.LastLoginAt = &t
	}
	return &a, nil
}

func (r *RepositorySQLite) UpdateAdmin(ctx context.Context, admin *AdminUser) error {
	_, err := r.db.ExecContext(ctx, `UPDATE admin_users SET
		username = ?, password_hash = ?, role = ?, disabled = ?, last_login_at = ?
		WHERE id = ?`,
		admin.Username, admin.PasswordHash, admin.Role, admin.Disabled,
		formatTimePtr(admin.LastLoginAt), admin.ID)
	return err
}

func (r *RepositorySQLite) ListAdmins(ctx context.Context) ([]AdminUser, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, username, role, disabled, created_at, last_login_at FROM admin_users ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var admins []AdminUser
	for rows.Next() {
		var a AdminUser
		var disabled int
		var createdAt string
		var lastLoginAt sql.NullString
		if err := rows.Scan(&a.ID, &a.Username, &a.Role, &disabled, &createdAt, &lastLoginAt); err != nil {
			return nil, err
		}
		a.Disabled = disabled == 1
		a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if lastLoginAt.Valid {
			t, _ := time.Parse(time.RFC3339, lastLoginAt.String)
			a.LastLoginAt = &t
		}
		admins = append(admins, a)
	}
	return admins, nil
}

func (r *RepositorySQLite) CreateSession(ctx context.Context, session *AdminSession) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO admin_sessions (
		token_hash, admin_id, created_at, expires_at, last_seen_at, remote_addr, user_agent
	) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		session.TokenHash, session.AdminID, session.CreatedAt.Format(time.RFC3339),
		session.ExpiresAt.Format(time.RFC3339), formatTimePtr(session.LastSeenAt),
		session.RemoteAddr, session.UserAgent)
	return err
}

func (r *RepositorySQLite) GetSession(ctx context.Context, tokenHash string) (*AdminSession, error) {
	var s AdminSession
	var createdAt, expiresAt string
	var lastSeenAt sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT token_hash, admin_id, created_at, expires_at, last_seen_at, remote_addr, user_agent
		FROM admin_sessions WHERE token_hash = ?`, tokenHash).Scan(
		&s.TokenHash, &s.AdminID, &createdAt, &expiresAt, &lastSeenAt, &s.RemoteAddr, &s.UserAgent)
	if err == sql.ErrNoRows {
		return nil, errors.New("session not found")
	}
	if err != nil {
		return nil, err
	}
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	s.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	if lastSeenAt.Valid {
		t, _ := time.Parse(time.RFC3339, lastSeenAt.String)
		s.LastSeenAt = &t
	}
	return &s, nil
}

func (r *RepositorySQLite) UpdateSession(ctx context.Context, session *AdminSession) error {
	_, err := r.db.ExecContext(ctx, `UPDATE admin_sessions SET
		expires_at = ?, last_seen_at = ? WHERE token_hash = ?`,
		session.ExpiresAt.Format(time.RFC3339), formatTimePtr(session.LastSeenAt), session.TokenHash)
	return err
}

func (r *RepositorySQLite) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (r *RepositorySQLite) DeleteSessionsByAdmin(ctx context.Context, adminID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM admin_sessions WHERE admin_id = ?`, adminID)
	return err
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.RFC3339)
	return &s
}

func parseTimePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}
