package agents

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// InviteCode represents an invitation code snapshot.
type InviteCode struct {
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
	UsedBy    string    `json:"used_by,omitempty"`
	UsedAt    time.Time `json:"used_at,omitempty"`
	MaxUses   int       `json:"max_uses"`
	UseCount  int       `json:"use_count"`
}

// InviteService handles invite codes using SQLite.
type InviteService struct {
	db *sql.DB
}

// NewInviteService creates a new InviteService.
func NewInviteService(db *sql.DB) *InviteService {
	return &InviteService{db: db}
}

// List returns all invite codes.
func (s *InviteService) List() ([]InviteCode, error) {
	rows, err := s.db.Query(`SELECT code, created_at, created_by, used_by, used_at, max_uses, use_count FROM invites ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []InviteCode
	for rows.Next() {
		var c InviteCode
		var createdBy sql.NullString
		var usedBy sql.NullString
		var ut sql.NullTime
		var maxUses sql.NullInt64
		var useCount sql.NullInt64
		if err := rows.Scan(&c.Code, &c.CreatedAt, &createdBy, &usedBy, &ut, &maxUses, &useCount); err != nil {
			return nil, err
		}
		c.CreatedAt = c.CreatedAt.UTC()
		if createdBy.Valid {
			c.CreatedBy = createdBy.String
		}
		if usedBy.Valid {
			c.UsedBy = usedBy.String
		}
		if ut.Valid {
			c.UsedAt = ut.Time.UTC()
		}
		if maxUses.Valid {
			c.MaxUses = int(maxUses.Int64)
		}
		if useCount.Valid {
			c.UseCount = int(useCount.Int64)
		}
		res = append(res, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if res == nil {
		res = []InviteCode{}
	}
	return res, nil
}

// Create generates a new invite code.
func (s *InviteService) Create(maxUses int, createdBy string) (InviteCode, error) {
	if maxUses < 0 {
		return InviteCode{}, fmt.Errorf("max_uses must be >= 0")
	}
	code, err := generateInviteCode()
	if err != nil {
		return InviteCode{}, err
	}
	now := time.Now().UTC()
	invite := InviteCode{
		Code:      code,
		CreatedAt: now,
		CreatedBy: createdBy,
		MaxUses:   maxUses,
		UseCount:  0,
	}
	_, err = s.db.Exec(`INSERT INTO invites (code, created_at, created_by, used_by, used_at, max_uses, use_count) VALUES (?, ?, ?, '', NULL, ?, 0)`,
		invite.Code, invite.CreatedAt, invite.CreatedBy, invite.MaxUses)
	if err != nil {
		return InviteCode{}, err
	}
	return invite, nil
}

// Delete removes an invite code.
func (s *InviteService) Delete(code string) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM invites WHERE code = ?`, code)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// ValidateAndUseTx validates and consumes an invite code inside a transaction.
// It uses a single conditional UPDATE to avoid the SELECT/UPDATE race.
func (s *InviteService) ValidateAndUseTx(ctx context.Context, tx *sql.Tx, code string, usedBy string) error {
	if code == "" {
		return fmt.Errorf("邀请码不能为空")
	}

	query := `
		UPDATE invites
		SET use_count = use_count + 1,
			used_by = CASE WHEN used_by = '' OR used_by IS NULL THEN ? ELSE used_by END,
			used_at = ?
		WHERE code = ? AND (max_uses = 0 OR use_count < max_uses)
	`
	res, err := tx.ExecContext(ctx, query, usedBy, time.Now().UTC(), code)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		// Either the code does not exist or its uses have been exhausted.
		var exists bool
		err := tx.QueryRowContext(ctx, `SELECT 1 FROM invites WHERE code = ?`, code).Scan(&exists)
		if err == sql.ErrNoRows {
			return fmt.Errorf("邀请码无效")
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("邀请码已用完")
	}
	return nil
}

// ValidateAndUse validates and consumes an invite code.
func (s *InviteService) ValidateAndUse(code string, usedBy string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.ValidateAndUseTx(context.Background(), tx, code, usedBy); err != nil {
		return err
	}
	return tx.Commit()
}

// RollbackUse is a no-op kept for backwards compatibility (handled by database transactions).
func (s *InviteService) RollbackUse(code string, username string) error {
	return nil
}

func generateInviteCode() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "INV-" + strings.ToUpper(hex.EncodeToString(b)), nil
}
