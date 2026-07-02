package audit

import (
	"context"
	"database/sql"
)

// SQLiteRepository implements the repository interface for SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLite-backed audit repository and runs migrations.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	r := &SQLiteRepository{db: db}
	r.migrate()
	return r
}

func (r *SQLiteRepository) migrate() {
	columns := []string{
		"actor_type TEXT",
		"actor_id TEXT",
		"resource_type TEXT",
		"resource_id TEXT",
		"before TEXT",
		"after TEXT",
		"user_agent TEXT",
		"request_id TEXT",
	}
	for _, col := range columns {
		_, _ = r.db.Exec("ALTER TABLE audit_log ADD COLUMN " + col)
	}
}

// Insert inserts an audit event into the audit_log table.
func (r *SQLiteRepository) Insert(ctx context.Context, e Event) error {
	query := `INSERT INTO audit_log (
		time, action, card, machine, agent_id, detail, addr,
		actor_type, actor_id, resource_type, resource_id,
		before, after, user_agent, request_id
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query,
		e.Time, e.Action, e.Card, e.Machine, e.AgentID, e.Detail, e.IP,
		e.ActorType, e.ActorID, e.ResourceType, e.ResourceID,
		e.Before, e.After, e.UserAgent, e.RequestID,
	)
	return err
}

func (r *SQLiteRepository) List(ctx context.Context, filter Filter, limit int) ([]Event, error) {
	query := `SELECT time, action, card, machine, agent_id, detail, addr,
		actor_type, actor_id, resource_type, resource_id,
		before, after, user_agent, request_id
		FROM audit_log WHERE 1=1`
	var args []interface{}

	if filter.Action != "" {
		query += " AND action = ?"
		args = append(args, filter.Action)
	}
	if filter.ActorType != "" {
		query += " AND actor_type = ?"
		args = append(args, filter.ActorType)
	}
	if filter.ActorID != "" {
		query += " AND actor_id = ?"
		args = append(args, filter.ActorID)
	}
	if filter.ResourceType != "" {
		query += " AND resource_type = ?"
		args = append(args, filter.ResourceType)
	}
	if filter.ResourceID != "" {
		query += " AND resource_id = ?"
		args = append(args, filter.ResourceID)
	}
	if filter.Card != "" {
		query += " AND card = ?"
		args = append(args, filter.Card)
	}
	if filter.AgentID != "" {
		query += " AND agent_id = ?"
		args = append(args, filter.AgentID)
	}
	if filter.Machine != "" {
		query += " AND machine = ?"
		args = append(args, filter.Machine)
	}
	if filter.IP != "" {
		query += " AND addr = ?"
		args = append(args, filter.IP)
	}
	if !filter.From.IsZero() {
		query += " AND time >= ?"
		args = append(args, filter.From)
	}
	if !filter.To.IsZero() {
		query += " AND time <= ?"
		args = append(args, filter.To)
	}
	if filter.Query != "" {
		query += " AND (detail LIKE ? OR action LIKE ?)"
		likeQ := "%" + filter.Query + "%"
		args = append(args, likeQ, likeQ)
	}

	query += " ORDER BY time DESC LIMIT ?"
	args = append(args, limit)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var e Event
		var card, machine, agentID, detail, ip sql.NullString
		var actorType, actorID, resType, resID, before, after, ua, reqID sql.NullString
		if err := rows.Scan(
			&e.Time, &e.Action, &card, &machine, &agentID, &detail, &ip,
			&actorType, &actorID, &resType, &resID, &before, &after, &ua, &reqID,
		); err != nil {
			return nil, err
		}
		if card.Valid { e.Card = card.String }
		if machine.Valid { e.Machine = machine.String }
		if agentID.Valid { e.AgentID = agentID.String }
		if detail.Valid { e.Detail = detail.String }
		if ip.Valid { e.IP = ip.String }
		if actorType.Valid { e.ActorType = actorType.String }
		if actorID.Valid { e.ActorID = actorID.String }
		if resType.Valid { e.ResourceType = resType.String }
		if resID.Valid { e.ResourceID = resID.String }
		if before.Valid { e.Before = before.String }
		if after.Valid { e.After = after.String }
		if ua.Valid { e.UserAgent = ua.String }
		if reqID.Valid { e.RequestID = reqID.String }
		events = append(events, e)
	}
	return events, rows.Err()
}
