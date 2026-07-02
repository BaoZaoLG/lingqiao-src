package audit

import (
	"context"
	"database/sql"
)

// SQLiteRepository implements the repository interface for SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLite-backed audit repository.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

// Insert inserts an audit event into the legacy audit_log table.
func (r *SQLiteRepository) Insert(ctx context.Context, e Event) error {
	_, err := r.db.ExecContext(ctx, "INSERT INTO audit_log (time, action, card, machine, agent_id, detail, addr) VALUES (?, ?, ?, ?, ?, ?, ?)", e.Time, e.Action, e.Card, e.Machine, e.AgentID, e.Detail, e.IP)
	return err
}

func (r *SQLiteRepository) List(ctx context.Context, limit int) ([]Event, error) {
	query := `SELECT time, action, card, machine, agent_id, detail, addr FROM audit_log ORDER BY time DESC LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var card, machine, agentID, detail, ip sql.NullString
		if err := rows.Scan(
			&e.Time,
			&e.Action,
			&card,
			&machine,
			&agentID,
			&detail,
			&ip,
		); err != nil {
			return nil, err
		}
		if card.Valid {
			e.Card = card.String
		}
		if machine.Valid {
			e.Machine = machine.String
		}
		if agentID.Valid {
			e.AgentID = agentID.String
		}
		if detail.Valid {
			e.Detail = detail.String
		}
		if ip.Valid {
			e.IP = ip.String
		}
		events = append(events, e)
	}
	return events, rows.Err()
}
