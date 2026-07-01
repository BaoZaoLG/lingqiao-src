package agents

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"strings"
	"time"
)

// Repository defines the data operations for the agents module.
type Repository interface {
	CreateAgent(ctx context.Context, username, passwordHash, prefix string) (Account, error)
	CreateAgentTx(ctx context.Context, tx *sql.Tx, username, passwordHash, prefix string) (Account, error)
	GetAgent(ctx context.Context, id string) (*Account, error)
	UsernameExists(ctx context.Context, username string) (bool, error)
	UpdateAgentPassword(ctx context.Context, id, passwordHash string) error
	AllAgents(ctx context.Context) ([]Account, error)
	UpdateAgentStatus(ctx context.Context, id string, disabled bool) error
	DeleteAgent(ctx context.Context, id string) error
	BeginTx(ctx context.Context) (*sql.Tx, error)
}

type SQLiteRepository struct {
	db *sql.DB
}

func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}

func (r *SQLiteRepository) CreateAgent(ctx context.Context, username, passwordHash, prefix string) (Account, error) {
	return r.CreateAgentTx(ctx, nil, username, passwordHash, prefix)
}

func (r *SQLiteRepository) CreateAgentTx(ctx context.Context, tx *sql.Tx, username, passwordHash, prefix string) (Account, error) {
	query := `INSERT INTO agents (id, username, password, prefix, created_at, disabled) VALUES (?, ?, ?, ?, ?, 0)`
	now := time.Now()
	var id string
	var err error

	var runner interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
	} = r.db
	if tx != nil {
		runner = tx
	}

	for i := 0; i < 5; i++ {
		id = "AGT-" + generateShortID()
		_, err = runner.ExecContext(ctx, query, id, username, passwordHash, prefix, now)
		if err == nil {
			return Account{
				ID:        id,
				Username:  username,
				Password:  passwordHash,
				Prefix:    prefix,
				CreatedAt: now,
				Disabled:  false,
			}, nil
		}
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			if strings.Contains(err.Error(), "agents.username") {
				return Account{}, ErrUsernameExists
			}
			if strings.Contains(err.Error(), "agents.id") {
				continue
			}
			return Account{}, ErrUsernameExists
		}
	}
	return Account{}, err
}

func (r *SQLiteRepository) GetAgent(ctx context.Context, id string) (*Account, error) {
	query := `SELECT id, username, password, prefix, created_at, disabled FROM agents WHERE id = ?`
	var a Account
	err := r.db.QueryRowContext(ctx, query, id).Scan(&a.ID, &a.Username, &a.Password, &a.Prefix, &a.CreatedAt, &a.Disabled)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *SQLiteRepository) UsernameExists(ctx context.Context, username string) (bool, error) {
	query := `SELECT 1 FROM agents WHERE username = ? LIMIT 1`
	var dummy int
	err := r.db.QueryRowContext(ctx, query, username).Scan(&dummy)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *SQLiteRepository) UpdateAgentPassword(ctx context.Context, id, passwordHash string) error {
	res, err := r.db.ExecContext(ctx, `UPDATE agents SET password = ? WHERE id = ?`, passwordHash, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrAgentNotFound
	}
	return nil
}

func (r *SQLiteRepository) AllAgents(ctx context.Context) ([]Account, error) {
	query := `SELECT id, username, password, prefix, created_at, disabled FROM agents ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []Account
	for rows.Next() {
		var a Account
		if err := rows.Scan(&a.ID, &a.Username, &a.Password, &a.Prefix, &a.CreatedAt, &a.Disabled); err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, rows.Err()
}

func (r *SQLiteRepository) UpdateAgentStatus(ctx context.Context, id string, disabled bool) error {
	disabledInt := 0
	if disabled {
		disabledInt = 1
	}
	res, err := r.db.ExecContext(ctx, `UPDATE agents SET disabled = ? WHERE id = ?`, disabledInt, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrAgentNotFound
	}
	return nil
}

func (r *SQLiteRepository) DeleteAgent(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM agents WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrAgentNotFound
	}
	return nil
}

var generateShortID = func() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand read failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}
