package main

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/lingqiao/server/internal/storage"
)

// migrateJSONInvitesToSQLite imports legacy invites.json rows when the invites
// table is empty. The import is transactional and idempotent for repeated calls
// once rows already exist.
func migrateJSONInvitesToSQLite(db *sql.DB, store storage.Store) error {
	if db == nil || store == nil {
		return nil
	}

	var inviteCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM invites`).Scan(&inviteCount); err != nil {
		return fmt.Errorf("count invites: %w", err)
	}
	if inviteCount > 0 {
		return nil
	}

	var jsonInvites []struct {
		Code      string `json:"code"`
		CreatedAt string `json:"created_at"`
		CreatedBy string `json:"created_by"`
		UsedBy    string `json:"used_by"`
		UsedAt    string `json:"used_at"`
		MaxUses   int    `json:"max_uses"`
		UseCount  int    `json:"use_count"`
	}
	if err := store.Load("invites", &jsonInvites); err != nil || len(jsonInvites) == 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin invites migration: %w", err)
	}
	defer tx.Rollback()

	for _, iv := range jsonInvites {
		var usedAt interface{}
		if strings.TrimSpace(iv.UsedAt) != "" {
			usedAt = iv.UsedAt
		}
		if _, err := tx.Exec(`
			INSERT INTO invites (code, created_at, created_by, used_by, used_at, max_uses, use_count)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			iv.Code, iv.CreatedAt, iv.CreatedBy, iv.UsedBy, usedAt, iv.MaxUses, iv.UseCount,
		); err != nil {
			return fmt.Errorf("insert invite %s: %w", iv.Code, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit invites migration: %w", err)
	}
	log.Println("[DB] Invites migration completed successfully.")
	return nil
}