package sessions

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) RunInTx(ctx context.Context, fn func(Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	sqlTx := &sqliteTx{tx: tx}
	if err := fn(sqlTx); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) GetSession(ctx context.Context, token string) (Session, error) {
	row := r.db.QueryRowContext(ctx, "SELECT token, card_code, machine_id, fingerprint, client_version, created_at, last_seen, expires_at, remote_addr FROM sessions WHERE token = ?", token)
	return scanSession(row)
}

func (r *SQLiteRepository) ListSessions(ctx context.Context, filter ListFilter, now time.Time) ([]Session, int, error) {
	var sessions []Session
	var total int

	query := "SELECT token, card_code, machine_id, fingerprint, client_version, created_at, last_seen, expires_at, remote_addr FROM sessions"
	countQuery := "SELECT COUNT(*) FROM sessions"

	var conditions []string
	var args []interface{}

	if filter.ActiveOnly {
		conditions = append(conditions, "expires_at > ?")
		args = append(args, now.Format(time.RFC3339))
	}
	if filter.CardCode != "" {
		conditions = append(conditions, "card_code = ?")
		args = append(args, filter.CardCode)
	}
	if filter.MachineID != "" {
		conditions = append(conditions, "machine_id = ?")
		args = append(args, filter.MachineID)
	}

	if len(conditions) > 0 {
		whereClause := " WHERE " + strings.Join(conditions, " AND ")
		query += whereClause
		countQuery += whereClause
	}

	err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query += " ORDER BY last_seen DESC"
	if filter.PerPage > 0 {
		query += " LIMIT ? OFFSET ?"
		args = append(args, filter.PerPage, (filter.Page-1)*filter.PerPage)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		s, err := scanSessionRows(rows)
		if err != nil {
			return nil, 0, err
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return sessions, total, nil
}

func (r *SQLiteRepository) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	res, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at < ?", now.Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return int(affected), nil
}

type sqliteTx struct {
	tx *sql.Tx
}

func (t *sqliteTx) GetCard(ctx context.Context, code string) (CardSnapshot, error) {
	row := t.tx.QueryRowContext(ctx, "SELECT code, status, note, machine_id, agent_id, created_at, activated_at, expires_at, max_sessions FROM cards WHERE code = ?", code)

	var c CardSnapshot
	var createdStr, expiresStr, activatedStr, machineID, agentID, note sql.NullString

	err := row.Scan(
		&c.Code,
		&c.Status,
		&note,
		&machineID,
		&agentID,
		&createdStr,
		&activatedStr,
		&expiresStr,
		&c.MaxSessions,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return CardSnapshot{}, ErrCardNotFound
		}
		return CardSnapshot{}, err
	}

	c.MachineID = machineID.String
	c.AgentID = agentID.String
	c.Note = note.String
	c.CreatedAt, _ = time.Parse(time.RFC3339, createdStr.String)
	c.ExpiresAt, _ = time.Parse(time.RFC3339, expiresStr.String)
	if activatedStr.Valid && activatedStr.String != "" {
		activated, _ := time.Parse(time.RFC3339, activatedStr.String)
		c.ActivatedAt = &activated
	}
	return c, nil
}

func (t *sqliteTx) IsCardBlacklisted(ctx context.Context, code string) (bool, error) {
	var count int
	err := t.tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM blacklist WHERE value = ? OR value = 'ALL'", code).Scan(&count)
	return count > 0, err
}

func (t *sqliteTx) ListSessionsByCard(ctx context.Context, code string) ([]Session, error) {
	rows, err := t.tx.QueryContext(ctx, "SELECT token, card_code, machine_id, fingerprint, client_version, created_at, last_seen, expires_at, remote_addr FROM sessions WHERE card_code = ?", code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		s, err := scanSessionRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (t *sqliteTx) InsertSession(ctx context.Context, session Session) error {
	_, err := t.tx.ExecContext(ctx,
		"INSERT INTO sessions (token, card_code, machine_id, fingerprint, client_version, created_at, last_seen, expires_at, remote_addr) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		session.Token, session.CardCode, session.MachineID, session.Fingerprint, session.ClientVersion,
		session.CreatedAt.Format(time.RFC3339), session.LastSeen.Format(time.RFC3339), session.ExpiresAt.Format(time.RFC3339), session.RemoteAddr,
	)
	return err
}

func (t *sqliteTx) GetSession(ctx context.Context, token string) (Session, error) {
	row := t.tx.QueryRowContext(ctx, "SELECT token, card_code, machine_id, fingerprint, client_version, created_at, last_seen, expires_at, remote_addr FROM sessions WHERE token = ?", token)
	return scanSession(row)
}

func (t *sqliteTx) UpdateSession(ctx context.Context, session Session) error {
	_, err := t.tx.ExecContext(ctx,
		"UPDATE sessions SET client_version = ?, last_seen = ?, expires_at = ?, remote_addr = ? WHERE token = ?",
		session.ClientVersion, session.LastSeen.Format(time.RFC3339), session.ExpiresAt.Format(time.RFC3339), session.RemoteAddr, session.Token,
	)
	return err
}

func (t *sqliteTx) DeleteSession(ctx context.Context, token string) error {
	_, err := t.tx.ExecContext(ctx, "DELETE FROM sessions WHERE token = ?", token)
	return err
}

func (t *sqliteTx) UpdateCardActivation(ctx context.Context, card CardSnapshot) error {
	var activatedStr string
	if card.ActivatedAt != nil {
		activatedStr = card.ActivatedAt.Format(time.RFC3339)
	}
	_, err := t.tx.ExecContext(ctx,
		"UPDATE cards SET machine_id = ?, activated_at = ?, expires_at = ? WHERE code = ?",
		card.MachineID, activatedStr, card.ExpiresAt.Format(time.RFC3339), card.Code,
	)
	return err
}

func (t *sqliteTx) UnbindCardPreservingRemaining(ctx context.Context, cardCode string, now time.Time) error {
	c, err := t.GetCard(ctx, cardCode)
	if err != nil {
		return err
	}
	if c.ActivatedAt != nil {
		duration := c.ExpiresAt.Sub(*c.ActivatedAt)
		c.ExpiresAt = now.Add(duration)
	}
	_, err = t.tx.ExecContext(ctx,
		"UPDATE cards SET machine_id = '', activated_at = NULL, expires_at = ? WHERE code = ?",
		c.ExpiresAt.Format(time.RFC3339), cardCode,
	)
	return err
}

// Helper interfaces for sharing scan logic
type scanner interface {
	Scan(dest ...interface{}) error
}

func scanSession(row scanner) (Session, error) {
	var s Session
	var createdStr, lastSeenStr, expiresStr, machineID, fingerprint, clientVersion, remoteAddr sql.NullString
	err := row.Scan(&s.Token, &s.CardCode, &machineID, &fingerprint, &clientVersion, &createdStr, &lastSeenStr, &expiresStr, &remoteAddr)
	if err != nil {
		if err == sql.ErrNoRows {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, err
	}
	s.MachineID = machineID.String
	s.Fingerprint = fingerprint.String
	s.ClientVersion = clientVersion.String
	s.RemoteAddr = remoteAddr.String
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdStr.String)
	s.LastSeen, _ = time.Parse(time.RFC3339, lastSeenStr.String)
	s.ExpiresAt, _ = time.Parse(time.RFC3339, expiresStr.String)
	return s, nil
}

func scanSessionRows(rows *sql.Rows) (Session, error) {
	return scanSession(rows)
}
