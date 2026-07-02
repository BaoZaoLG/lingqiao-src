package cards

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNotFound = errors.New("card not found")

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) Get(ctx context.Context, code string) (Card, error) {
	row := r.db.QueryRowContext(ctx, "SELECT code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions FROM cards WHERE code = ?", code)
	var card Card
	var machineID, agentID, note, createdAt, activatedAt, expiresAt sql.NullString
	err := row.Scan(
		&card.Code,
		&machineID,
		&agentID,
		&createdAt,
		&activatedAt,
		&expiresAt,
		&card.Status,
		&note,
		&card.MaxSessions,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Card{}, ErrNotFound
		}
		return Card{}, err
	}
	card.MachineID = machineID.String
	card.AgentID = agentID.String
	card.Note = note.String
	card.CreatedAt = parseDBTime(createdAt.String)
	card.ExpiresAt = parseDBTime(expiresAt.String)
	if activatedAt.Valid && activatedAt.String != "" {
		t := parseDBTime(activatedAt.String)
		card.ActivatedAt = &t
	}
	return card, nil
}

func (r *SQLiteRepository) Insert(ctx context.Context, card Card) error {
	var activatedAt interface{}
	if card.ActivatedAt != nil {
		activatedAt = formatDBTime(*card.ActivatedAt)
	}
	query := `
		INSERT INTO cards (code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		card.Code,
		card.MachineID,
		card.AgentID,
		formatDBTime(card.CreatedAt),
		activatedAt,
		formatDBTime(card.ExpiresAt),
		card.Status,
		card.Note,
		card.MaxSessions,
	)
	return err
}

func (r *SQLiteRepository) Save(ctx context.Context, card Card) error {
	var activatedAt interface{}
	if card.ActivatedAt != nil {
		activatedAt = formatDBTime(*card.ActivatedAt)
	}
	query := `
		INSERT INTO cards (code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(code) DO UPDATE SET
			machine_id = excluded.machine_id,
			agent_id = excluded.agent_id,
			activated_at = excluded.activated_at,
			expires_at = excluded.expires_at,
			status = excluded.status,
			note = excluded.note,
			max_sessions = excluded.max_sessions
	`
	_, err := r.db.ExecContext(ctx, query,
		card.Code,
		card.MachineID,
		card.AgentID,
		formatDBTime(card.CreatedAt),
		activatedAt,
		formatDBTime(card.ExpiresAt),
		card.Status,
		card.Note,
		card.MaxSessions,
	)
	return err
}

func (r *SQLiteRepository) List(ctx context.Context, filter ListFilter) ([]Card, error) {
	query := "SELECT code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions FROM cards WHERE 1=1"
	var args []interface{}

	if filter.Query != "" {
		query += " AND (code LIKE ? OR note LIKE ?)"
		likeQ := "%" + filter.Query + "%"
		args = append(args, likeQ, likeQ)
	}
	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.MachineID != "" {
		query += " AND machine_id = ?"
		args = append(args, filter.MachineID)
	}
	if filter.AgentID != "" {
		query += " AND agent_id = ?"
		args = append(args, filter.AgentID)
	}

	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cards []Card
	for rows.Next() {
		var card Card
		var machineID, agentID, note, createdAt, activatedAt, expiresAt sql.NullString
		if err := rows.Scan(
			&card.Code,
			&machineID,
			&agentID,
			&createdAt,
			&activatedAt,
			&expiresAt,
			&card.Status,
			&note,
			&card.MaxSessions,
		); err != nil {
			return nil, err
		}
		card.MachineID = machineID.String
		card.AgentID = agentID.String
		card.Note = note.String
		card.CreatedAt = parseDBTime(createdAt.String)
		card.ExpiresAt = parseDBTime(expiresAt.String)
		if activatedAt.Valid && activatedAt.String != "" {
			t := parseDBTime(activatedAt.String)
			card.ActivatedAt = &t
		}
		cards = append(cards, card)
	}
	return cards, rows.Err()
}

func (r *SQLiteRepository) BatchSave(ctx context.Context, cards []Card) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO cards (code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(code) DO UPDATE SET
			machine_id = excluded.machine_id,
			agent_id = excluded.agent_id,
			activated_at = excluded.activated_at,
			expires_at = excluded.expires_at,
			status = excluded.status,
			note = excluded.note,
			max_sessions = excluded.max_sessions
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, card := range cards {
		var activatedAt interface{}
		if card.ActivatedAt != nil {
			activatedAt = formatDBTime(*card.ActivatedAt)
		}
		_, err := stmt.ExecContext(ctx,
			card.Code,
			card.MachineID,
			card.AgentID,
			formatDBTime(card.CreatedAt),
			activatedAt,
			formatDBTime(card.ExpiresAt),
			card.Status,
			card.Note,
			card.MaxSessions,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *SQLiteRepository) UpdateStatusBulk(ctx context.Context, codes []string, status Status) (int, error) {
	if len(codes) == 0 {
		return 0, nil
	}
	placeholders := strings.Repeat("?,", len(codes)-1) + "?"
	query := fmt.Sprintf("UPDATE cards SET status = ? WHERE code IN (%s)", placeholders)

	args := []interface{}{status}
	for _, c := range codes {
		args = append(args, c)
	}

	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return int(affected), nil
}

func (r *SQLiteRepository) ExtendBulk(ctx context.Context, codes []string, extendHours int) (int, error) {
	if len(codes) == 0 {
		return 0, nil
	}
	placeholders := strings.Repeat("?,", len(codes)-1) + "?"
	query := fmt.Sprintf("UPDATE cards SET expires_at = datetime(expires_at, '+%d hours'), status = CASE WHEN status = 'expired' THEN 'active' ELSE status END WHERE code IN (%s)", extendHours, placeholders)

	args := []interface{}{}
	for _, c := range codes {
		args = append(args, c)
	}

	res, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	affected, _ := res.RowsAffected()
	return int(affected), nil
}

func (r *SQLiteRepository) Delete(ctx context.Context, code string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM cards WHERE code = ?", code)
	return err
}

func (r *SQLiteRepository) Count(ctx context.Context, filter ListFilter) (int, error) {
	query := "SELECT COUNT(*) FROM cards WHERE 1=1"
	var args []interface{}

	if filter.Status != "" {
		query += " AND status = ?"
		args = append(args, filter.Status)
	}
	if filter.AgentID != "" {
		query += " AND agent_id = ?"
		args = append(args, filter.AgentID)
	}

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func formatDBTime(t time.Time) string {
	return t.Round(0).Format(time.RFC3339Nano)
}

func parseDBTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	if i := strings.Index(value, " m="); i >= 0 {
		value = value[:i]
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, value); err == nil {
			return t
		}
	}
	return time.Time{}
}
