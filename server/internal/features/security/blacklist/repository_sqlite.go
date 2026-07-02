package blacklist

import (
	"context"
	"database/sql"
)

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) Add(ctx context.Context, entry Entry) error {
	query := `INSERT OR REPLACE INTO blacklist (value, type, reason, created_at) VALUES (?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, entry.Value, entry.Type, entry.Reason, entry.CreatedAt)
	return err
}

func (r *SQLiteRepository) Remove(ctx context.Context, value string) error {
	query := `DELETE FROM blacklist WHERE value = ?`
	_, err := r.db.ExecContext(ctx, query, value)
	return err
}

func (r *SQLiteRepository) List(ctx context.Context) ([]Entry, error) {
	query := `SELECT value, type, reason, created_at FROM blacklist ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.Value, &e.Type, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *SQLiteRepository) IsBlacklisted(ctx context.Context, value string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM blacklist WHERE value = ? OR value = 'ALL'`, value).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
