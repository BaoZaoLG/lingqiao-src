package main

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	agentsvc "github.com/lingqiao/server/internal/agents"
	auditsvc "github.com/lingqiao/server/internal/audit"
	cardops "github.com/lingqiao/server/internal/cards"
	_ "modernc.org/sqlite"
)

type SQLiteCardManager struct {
	db       *sql.DB
	recorder *auditsvc.Recorder
	mu       sync.Mutex // For protecting recorder, memory-only state if any
}

func NewSQLiteCardManager(dbPath string) (CardManager, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	
	schema := `
	CREATE TABLE IF NOT EXISTS cards (
		code TEXT PRIMARY KEY,
		machine_id TEXT,
		agent_id TEXT,
		created_at DATETIME,
		activated_at DATETIME,
		expires_at DATETIME,
		status TEXT,
		note TEXT,
		max_sessions INTEGER
	);
	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		card_code TEXT,
		machine_id TEXT,
		fingerprint TEXT,
		client_version TEXT,
		created_at DATETIME,
		last_seen DATETIME,
		expires_at DATETIME,
		remote_addr TEXT
	);
	CREATE TABLE IF NOT EXISTS agents (
		id TEXT PRIMARY KEY,
		username TEXT,
		password TEXT,
		prefix TEXT,
		created_at DATETIME,
		disabled BOOLEAN
	);
	CREATE TABLE IF NOT EXISTS blacklist (
		value TEXT PRIMARY KEY,
		type TEXT,
		reason TEXT,
		created_at DATETIME
	);
	CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		time DATETIME,
		action TEXT,
		card TEXT,
		machine TEXT,
		agent_id TEXT,
		detail TEXT,
		addr TEXT
	);
	`
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to init schema: %w", err)
	}

	return &SQLiteCardManager{
		db: db,
	}, nil
}

func (cm *SQLiteCardManager) GetStorage() *JSONStorage {
	return nil
}

func (cm *SQLiteCardManager) ValidateBoundSession(token, machineID, cardCode string) (*Session, error) {
	if token == "" {
		return nil, fmt.Errorf("session_token is required")
	}
	if machineID == "" {
		return nil, fmt.Errorf("machine_id is required")
	}
	now := time.Now()
	
	row := cm.db.QueryRow(`SELECT token, card_code, machine_id, fingerprint, client_version, created_at, last_seen, expires_at, remote_addr FROM sessions WHERE token = ?`, token)
	var session Session
	if err := row.Scan(&session.Token, &session.CardCode, &session.MachineID, &session.Fingerprint, &session.ClientVersion, &session.CreatedAt, &session.LastSeen, &session.ExpiresAt, &session.RemoteAddr); err != nil {
		return nil, fmt.Errorf("invalid or expired session")
	}
	if session.ExpiresAt.Before(now) {
		return nil, fmt.Errorf("invalid or expired session")
	}
	if session.MachineID != machineID {
		return nil, fmt.Errorf("session machine mismatch")
	}
	if cardCode != "" && normalizeCardCode(session.CardCode) != normalizeCardCode(cardCode) {
		return nil, fmt.Errorf("session card mismatch")
	}
	
	crow := cm.db.QueryRow(`SELECT status, expires_at FROM cards WHERE code = ?`, session.CardCode)
	var status string
	var expiresAt time.Time
	if err := crow.Scan(&status, &expiresAt); err != nil {
		return nil, fmt.Errorf("card no longer valid")
	}
	if status == string(CardDisabled) || status == string(CardExpired) || now.After(expiresAt) {
		return nil, fmt.Errorf("card no longer valid")
	}
	return &session, nil
}

func (cm *SQLiteCardManager) SetAuditRecorder(recorder *auditsvc.Recorder) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.recorder = recorder
}

func (cm *SQLiteCardManager) RecordAudit(entry AuditEntry) {
	if entry.Time.IsZero() {
		entry.Time = time.Now()
	}
	_, err := cm.db.Exec(`INSERT INTO audit_log (time, action, card, machine, agent_id, detail, addr) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.Time, entry.Action, entry.Card, entry.Machine, entry.AgentID, entry.Detail, entry.Addr)
	if err != nil {
		fmt.Printf("[ERROR] SQLite RecordAudit: %v\n", err)
	}
	
	cm.mu.Lock()
	if cm.recorder != nil {
		cm.recorder.Append(auditsvc.Event{
			Time:    entry.Time,
			Action:  entry.Action,
			Card:    entry.Card,
			AgentID: entry.AgentID,
			Machine: entry.Machine,
			IP:      entry.Addr,
			Detail:  entry.Detail,
		})
	}
	cm.mu.Unlock()
}

func parseTime(t sql.NullTime) *time.Time {
	if t.Valid {
		return &t.Time
	}
	return nil
}

func (cm *SQLiteCardManager) GetCard(code string) *Card {
	code = normalizeCardCode(code)
	row := cm.db.QueryRow(`SELECT code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions FROM cards WHERE code = ?`, code)
	var c Card
	var activatedAt sql.NullTime
	err := row.Scan(&c.Code, &c.MachineID, &c.AgentID, &c.CreatedAt, &activatedAt, &c.ExpiresAt, &c.Status, &c.Note, &c.MaxSessions)
	if err != nil {
		return nil
	}
	c.ActivatedAt = parseTime(activatedAt)
	return &c
}

func (cm *SQLiteCardManager) ValidateCard(code string) (*Card, error) {
	code = normalizeCardCode(code)
	c := cm.GetCard(code)
	if c == nil {
		return nil, cardops.ErrCardNotFound
	}
	if cm.IsBlacklisted(code) {
		return nil, cardops.ErrCardBlacklisted
	}
	if c.Status == CardDisabled {
		return nil, cardops.ErrCardDisabled
	}
	if c.Status == CardExpired || time.Now().After(c.ExpiresAt) {
		return nil, cardops.ErrCardExpired
	}
	return c, nil
}

func (cm *SQLiteCardManager) GenerateCard(duration time.Duration, note string, maxSessions int, agentID string) (*Card, error) {
	lifecycleCard, err := newCardLifecycleService().Create(duration, note, maxSessions, agentID)
	if err != nil {
		return nil, err
	}
	card := cardFromLifecycleCard(lifecycleCard)
	card.Code = normalizeCardCode(card.Code)

	_, err = cm.db.Exec(`INSERT INTO cards (code, machine_id, agent_id, created_at, expires_at, status, note, max_sessions) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		card.Code, card.MachineID, card.AgentID, card.CreatedAt, card.ExpiresAt, card.Status, card.Note, card.MaxSessions)
	if err != nil {
		return nil, err
	}
	
	cm.RecordAudit(AuditEntry{
		Time:    time.Now(),
		Action:  "card_generated",
		Card:    card.Code,
		AgentID: agentID,
		Detail:  fmt.Sprintf("duration=%s max_sessions=%d", duration, maxSessions),
	})
	return card, nil
}

func (cm *SQLiteCardManager) GenerateCardWithCode(code string, duration time.Duration, note string, maxSessions int, agentID string) (*Card, error) {
	normalized := normalizeCardCode(code)
	if len(normalized) != 18 {
		return nil, fmt.Errorf("card code must contain 18 base32 characters")
	}
	for i := 0; i < len(normalized); i++ {
		if crockfordDecode[normalized[i]] < 0 {
			return nil, fmt.Errorf("card code contains invalid character %q", normalized[i])
		}
	}
	if duration <= 0 {
		return nil, fmt.Errorf("duration must be positive")
	}
	if maxSessions < 1 {
		maxSessions = 1
	}

	now := time.Now()
	card := &Card{
		Code:        FormatCardCode(normalized),
		AgentID:     agentID,
		CreatedAt:   now,
		ExpiresAt:   now.Add(duration),
		Status:      CardActive,
		Note:        note,
		MaxSessions: maxSessions,
	}

	_, err := cm.db.Exec(`INSERT INTO cards (code, machine_id, agent_id, created_at, expires_at, status, note, max_sessions) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		normalized, card.MachineID, card.AgentID, card.CreatedAt, card.ExpiresAt, card.Status, card.Note, card.MaxSessions)
	if err != nil {
		return nil, fmt.Errorf("card already exists or db error: %v", err)
	}

	cm.RecordAudit(AuditEntry{
		Time:    now,
		Action:  "card_imported",
		Card:    card.Code,
		AgentID: agentID,
		Detail:  fmt.Sprintf("duration=%s max_sessions=%d", duration, maxSessions),
	})
	return card, nil
}

func (cm *SQLiteCardManager) getActiveSessionsForCard(code string) []cardops.Session {
	rows, err := cm.db.Query(`SELECT token, card_code, machine_id, fingerprint, client_version, created_at, last_seen, expires_at, remote_addr FROM sessions WHERE card_code = ?`, code)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var sessions []cardops.Session
	for rows.Next() {
		var s cardops.Session
		if err := rows.Scan(&s.Token, &s.CardCode, &s.MachineID, &s.Fingerprint, &s.ClientVersion, &s.CreatedAt, &s.LastSeen, &s.ExpiresAt, &s.RemoteAddr); err == nil {
			sessions = append(sessions, s)
		}
	}
	return sessions
}

func (cm *SQLiteCardManager) GetSession(token string) *Session {
	row := cm.db.QueryRow(`SELECT token, card_code, machine_id, fingerprint, client_version, created_at, last_seen, expires_at, remote_addr FROM sessions WHERE token = ?`, token)
	var s Session
	err := row.Scan(&s.Token, &s.CardCode, &s.MachineID, &s.Fingerprint, &s.ClientVersion, &s.CreatedAt, &s.LastSeen, &s.ExpiresAt, &s.RemoteAddr)
	if err != nil {
		return nil
	}
	return &s
}

func (cm *SQLiteCardManager) ActivateCard(code, machineID, fingerprint, addr, clientVersion string) (*Session, error) {
	code = normalizeCardCode(code)

	tx, err := cm.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Get Card inside tx
	row := tx.QueryRow(`SELECT code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions FROM cards WHERE code = ?`, code)
	var c Card
	var activatedAt sql.NullTime
	if err := row.Scan(&c.Code, &c.MachineID, &c.AgentID, &c.CreatedAt, &activatedAt, &c.ExpiresAt, &c.Status, &c.Note, &c.MaxSessions); err != nil {
		return nil, cardops.ErrCardNotFound
	}
	c.ActivatedAt = parseTime(activatedAt)
	
	// Validate logic again inside tx to avoid race
	if cm.IsBlacklisted(code) {
		return nil, cardops.ErrCardBlacklisted
	}
	if c.Status == CardDisabled {
		return nil, cardops.ErrCardDisabled
	}
	if c.Status == CardExpired || time.Now().After(c.ExpiresAt) {
		return nil, cardops.ErrCardExpired
	}

	rows, err := tx.Query(`SELECT token, card_code, machine_id, fingerprint, client_version, created_at, last_seen, expires_at, remote_addr FROM sessions WHERE card_code = ?`, code)
	if err != nil {
		return nil, err
	}
	var activeSessions []cardops.Session
	for rows.Next() {
		var s cardops.Session
		if err := rows.Scan(&s.Token, &s.CardCode, &s.MachineID, &s.Fingerprint, &s.ClientVersion, &s.CreatedAt, &s.LastSeen, &s.ExpiresAt, &s.RemoteAddr); err == nil {
			activeSessions = append(activeSessions, s)
		}
	}
	rows.Close()

	token := generateSessionToken()
	updatedCard, lifecycleSession, err := newCardSessionService().Activate(cardops.ActivationInput{
		Card:           lifecycleCardFromCard(&c),
		Token:          token,
		MachineID:      machineID,
		Fingerprint:    fingerprint,
		RemoteAddr:     addr,
		ClientVersion:  clientVersion,
		ActiveSessions: activeSessions,
	})
	if err != nil {
		return nil, err
	}
	
	lifecycleSession.CardCode = code
	applyLifecycleCard(&c, updatedCard)
	session := sessionFromLifecycleSession(lifecycleSession)
	
	// Update card
	_, err = tx.Exec(`UPDATE cards SET machine_id = ?, activated_at = ?, expires_at = ? WHERE code = ?`,
		c.MachineID, c.ActivatedAt, c.ExpiresAt, c.Code)
	if err != nil {
		return nil, err
	}

	// Insert session
	_, err = tx.Exec(`INSERT INTO sessions (token, card_code, machine_id, fingerprint, client_version, created_at, last_seen, expires_at, remote_addr) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.Token, session.CardCode, session.MachineID, session.Fingerprint, session.ClientVersion, session.CreatedAt, session.LastSeen, session.ExpiresAt, session.RemoteAddr)
	if err != nil {
		return nil, err
	}

	tx.Commit()

	cm.RecordAudit(AuditEntry{
		Time:    time.Now(),
		Action:  "card_activated",
		Card:    code,
		Machine: machineID,
		Addr:    addr,
	})

	return session, nil
}

func (cm *SQLiteCardManager) Heartbeat(token, machineID, addr, clientVersion string) (*Session, error) {
	tx, err := cm.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row := tx.QueryRow(`SELECT token, card_code, machine_id, fingerprint, client_version, created_at, last_seen, expires_at, remote_addr FROM sessions WHERE token = ?`, token)
	var session Session
	if err := row.Scan(&session.Token, &session.CardCode, &session.MachineID, &session.Fingerprint, &session.ClientVersion, &session.CreatedAt, &session.LastSeen, &session.ExpiresAt, &session.RemoteAddr); err != nil {
		return nil, fmt.Errorf("session not found")
	}

	crow := tx.QueryRow(`SELECT code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions FROM cards WHERE code = ?`, session.CardCode)
	var card Card
	var activatedAt sql.NullTime
	if err := crow.Scan(&card.Code, &card.MachineID, &card.AgentID, &card.CreatedAt, &activatedAt, &card.ExpiresAt, &card.Status, &card.Note, &card.MaxSessions); err != nil {
		tx.Exec(`DELETE FROM sessions WHERE token = ?`, token)
		tx.Commit()
		return nil, fmt.Errorf("card no longer valid")
	}
	card.ActivatedAt = parseTime(activatedAt)

	lifecycleSession, err := newCardSessionService().Heartbeat(cardops.HeartbeatInput{
		Session:       lifecycleSessionFromSession(&session),
		Card:          lifecycleCardFromCard(&card),
		MachineID:     machineID,
		RemoteAddr:    addr,
		ClientVersion: clientVersion,
	})
	if err != nil {
		if errors.Is(err, cardops.ErrSessionExpired) || errors.Is(err, cardops.ErrCardNoLongerValid) {
			tx.Exec(`DELETE FROM sessions WHERE token = ?`, token)
			tx.Commit()
		}
		return nil, err
	}

	applyLifecycleSession(&session, lifecycleSession)
	tx.Exec(`UPDATE sessions SET last_seen = ?, expires_at = ?, client_version = ?, remote_addr = ? WHERE token = ?`,
		session.LastSeen, session.ExpiresAt, session.ClientVersion, session.RemoteAddr, session.Token)
	tx.Commit()

	return &session, nil
}

func (cm *SQLiteCardManager) DeactivateSession(token string, unbind bool) error {
	tx, err := cm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	row := tx.QueryRow(`SELECT card_code FROM sessions WHERE token = ?`, token)
	var cardCode string
	if err := row.Scan(&cardCode); err != nil {
		return fmt.Errorf("session not found")
	}

	tx.Exec(`DELETE FROM sessions WHERE token = ?`, token)

	if unbind {
		crow := tx.QueryRow(`SELECT created_at, activated_at, expires_at FROM cards WHERE code = ?`, cardCode)
		var createdAt, expiresAt time.Time
		var activatedAt sql.NullTime
		if err := crow.Scan(&createdAt, &activatedAt, &expiresAt); err == nil {
			if activatedAt.Valid {
				remaining := time.Until(expiresAt)
				if remaining < 0 {
					remaining = 0
				}
				newExpires := time.Now().Add(remaining)
				tx.Exec(`UPDATE cards SET machine_id = '', activated_at = NULL, expires_at = ? WHERE code = ?`, newExpires, cardCode)
			} else {
				tx.Exec(`UPDATE cards SET machine_id = '', activated_at = NULL WHERE code = ?`, cardCode)
			}
		}
	}
	tx.Commit()

	cm.RecordAudit(AuditEntry{
		Time:   time.Now(),
		Action: "session_deactivated",
		Card:   cardCode,
	})
	return nil
}

func (cm *SQLiteCardManager) AllCards() []*Card {
	rows, err := cm.db.Query(`SELECT code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions FROM cards`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var res []*Card
	for rows.Next() {
		var c Card
		var act sql.NullTime
		if err := rows.Scan(&c.Code, &c.MachineID, &c.AgentID, &c.CreatedAt, &act, &c.ExpiresAt, &c.Status, &c.Note, &c.MaxSessions); err == nil {
			c.ActivatedAt = parseTime(act)
			res = append(res, &c)
		}
	}
	return res
}

func (cm *SQLiteCardManager) AllSessions() []*Session {
	rows, err := cm.db.Query(`SELECT token, card_code, machine_id, fingerprint, client_version, created_at, last_seen, expires_at, remote_addr FROM sessions`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var res []*Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.Token, &s.CardCode, &s.MachineID, &s.Fingerprint, &s.ClientVersion, &s.CreatedAt, &s.LastSeen, &s.ExpiresAt, &s.RemoteAddr); err == nil {
			res = append(res, &s)
		}
	}
	return res
}

func (cm *SQLiteCardManager) AllBlacklist() []*BlacklistEntry {
	rows, err := cm.db.Query(`SELECT value, type, reason, created_at FROM blacklist`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var res []*BlacklistEntry
	for rows.Next() {
		var b BlacklistEntry
		if err := rows.Scan(&b.Value, &b.Type, &b.Reason, &b.CreatedAt); err == nil {
			res = append(res, &b)
		}
	}
	return res
}

func (cm *SQLiteCardManager) AuditLog() []AuditEntry {
	rows, err := cm.db.Query(`SELECT time, action, card, machine, agent_id, detail, addr FROM audit_log ORDER BY id DESC LIMIT 10000`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var res []AuditEntry
	for rows.Next() {
		var a AuditEntry
		var card, machine, agent, detail, addr sql.NullString
		if err := rows.Scan(&a.Time, &a.Action, &card, &machine, &agent, &detail, &addr); err == nil {
			a.Card = card.String
			a.Machine = machine.String
			a.AgentID = agent.String
			a.Detail = detail.String
			a.Addr = addr.String
			res = append(res, a)
		}
	}
	// Reverse to keep chronological order if needed, but returning desc is fine for list.
	// Actually original was chronological, let's reverse it to match original order.
	for i, j := 0, len(res)-1; i < j; i, j = i+1, j-1 {
		res[i], res[j] = res[j], res[i]
	}
	return res
}

func (cm *SQLiteCardManager) AddBlacklist(typ, value, reason string) {
	cm.db.Exec(`INSERT OR REPLACE INTO blacklist (value, type, reason, created_at) VALUES (?, ?, ?, ?)`, value, typ, reason, time.Now())
	cm.RecordAudit(AuditEntry{
		Time:   time.Now(),
		Action: "blacklist_added",
		Detail: fmt.Sprintf("type=%s value=%s reason=%s", typ, value, reason),
	})
}

func (cm *SQLiteCardManager) RemoveBlacklist(value string) {
	cm.db.Exec(`DELETE FROM blacklist WHERE value = ?`, value)
	cm.RecordAudit(AuditEntry{
		Time:   time.Now(),
		Action: "blacklist_removed",
		Detail: fmt.Sprintf("value=%s", value),
	})
}

func (cm *SQLiteCardManager) UpdateCardStatus(code string, status CardStatus) error {
	code = normalizeCardCode(code)
	c := cm.GetCard(code)
	if c == nil {
		return fmt.Errorf("card not found")
	}
	updated := newCardLifecycleService().UpdateStatus(lifecycleCardFromCard(c), cardops.Status(status))
	_, err := cm.db.Exec(`UPDATE cards SET status = ? WHERE code = ?`, updated.Status, code)
	if err != nil {
		return err
	}
	cm.RecordAudit(AuditEntry{
		Time:   time.Now(),
		Action: "card_status_changed",
		Card:   code,
		Detail: fmt.Sprintf("new_status=%s", status),
	})
	return nil
}

func (cm *SQLiteCardManager) ExtendCard(code string, duration time.Duration) error {
	code = normalizeCardCode(code)
	c := cm.GetCard(code)
	if c == nil {
		return fmt.Errorf("card not found")
	}
	updated := newCardLifecycleService().Extend(lifecycleCardFromCard(c), duration)
	_, err := cm.db.Exec(`UPDATE cards SET expires_at = ?, status = ? WHERE code = ?`, updated.ExpiresAt, updated.Status, code)
	if err != nil {
		return err
	}
	cm.RecordAudit(AuditEntry{
		Time:   time.Now(),
		Action: "card_extended",
		Card:   code,
		Detail: fmt.Sprintf("added_duration=%s new_expires=%s", duration, updated.ExpiresAt.Format(time.RFC3339)),
	})
	return nil
}

func (cm *SQLiteCardManager) UnbindCard(code string) error {
	code = normalizeCardCode(code)
	c := cm.GetCard(code)
	if c == nil {
		return fmt.Errorf("card not found")
	}
	
	if c.ActivatedAt != nil {
		originalDuration := c.ExpiresAt.Sub(*c.ActivatedAt)
		c.ExpiresAt = c.CreatedAt.Add(originalDuration)
	}
	
	_, err := cm.db.Exec(`UPDATE cards SET machine_id = '', activated_at = NULL, expires_at = ? WHERE code = ?`, c.ExpiresAt, code)
	if err != nil {
		return err
	}
	cm.RecordAudit(AuditEntry{
		Time:   time.Now(),
		Action: "card_unbound",
		Card:   code,
	})
	return nil
}

func (cm *SQLiteCardManager) CleanupExpired() {
	cm.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now())
}

func (cm *SQLiteCardManager) IsBlacklisted(value string) bool {
	var v string
	err := cm.db.QueryRow(`SELECT value FROM blacklist WHERE value = ?`, value).Scan(&v)
	return err == nil
}

func (cm *SQLiteCardManager) BatchGenerateCards(count int, duration time.Duration, note string, maxSessions int, agentID string) ([]*Card, error) {
	if count < 1 {
		count = 1
	}
	if count > 100 {
		count = 100
	}
	cards := make([]*Card, 0, count)
	
	tx, err := cm.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	
	stmt, err := tx.Prepare(`INSERT INTO cards (code, machine_id, agent_id, created_at, expires_at, status, note, max_sessions) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	now := time.Now()
	for i := 0; i < count; i++ {
		code, err := generateCardCode()
		if err != nil {
			return nil, err
		}
		card := &Card{
			Code:        code,
			AgentID:     agentID,
			CreatedAt:   now,
			ExpiresAt:   now.Add(duration),
			Status:      CardActive,
			Note:        note,
			MaxSessions: maxSessions,
		}
		normalized := normalizeCardCode(code)
		if _, err := stmt.Exec(normalized, card.MachineID, card.AgentID, card.CreatedAt, card.ExpiresAt, card.Status, card.Note, card.MaxSessions); err != nil {
			return nil, err
		}
		cards = append(cards, card)
	}
	tx.Commit()

	cm.RecordAudit(AuditEntry{
		Time:    now,
		Action:  "cards_batch_generated",
		AgentID: agentID,
		Detail:  fmt.Sprintf("count=%d duration=%s max_sessions=%d note=%s", count, duration, maxSessions, note),
	})
	return cards, nil
}

func (cm *SQLiteCardManager) UpdateCardDetails(code string, note *string, maxSessions *int) error {
	code = normalizeCardCode(code)
	c := cm.GetCard(code)
	if c == nil {
		return fmt.Errorf("card not found")
	}
	updated := newCardLifecycleService().UpdateDetails(lifecycleCardFromCard(c), note, maxSessions)
	
	_, err := cm.db.Exec(`UPDATE cards SET note = ?, max_sessions = ? WHERE code = ?`, updated.Note, updated.MaxSessions, code)
	if err != nil {
		return err
	}
	changes := []string{}
	if note != nil {
		changes = append(changes, fmt.Sprintf("note=%s", *note))
	}
	if maxSessions != nil {
		changes = append(changes, fmt.Sprintf("max_sessions=%d", *maxSessions))
	}
	cm.RecordAudit(AuditEntry{
		Time:   time.Now(),
		Action: "card_details_updated",
		Card:   code,
		Detail: strings.Join(changes, " "),
	})
	return nil
}

func (cm *SQLiteCardManager) SearchCards(query, status, machineID string) []*Card {
	queryStr := "SELECT code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions FROM cards WHERE 1=1"
	args := []interface{}{}
	if status != "" {
		queryStr += " AND status = ?"
		args = append(args, status)
	}
	if machineID != "" {
		queryStr += " AND machine_id = ?"
		args = append(args, machineID)
	}
	if query != "" {
		query = strings.ToUpper(query)
		queryStr += " AND (UPPER(code) LIKE ? OR UPPER(note) LIKE ?)"
		likeQ := "%" + query + "%"
		args = append(args, likeQ, likeQ)
	}
	rows, err := cm.db.Query(queryStr, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var res []*Card
	for rows.Next() {
		var c Card
		var act sql.NullTime
		if err := rows.Scan(&c.Code, &c.MachineID, &c.AgentID, &c.CreatedAt, &act, &c.ExpiresAt, &c.Status, &c.Note, &c.MaxSessions); err == nil {
			c.ActivatedAt = parseTime(act)
			res = append(res, &c)
		}
	}
	return res
}

func (cm *SQLiteCardManager) GetUniqueMachines() []MachineInfo {
	cards := cm.AllCards()
	sessions := cm.AllSessions()
	blacklist := cm.AllBlacklist()
	
	machineMap := make(map[string]*MachineInfo)
	for _, c := range cards {
		if c.MachineID == "" {
			continue
		}
		info, exists := machineMap[c.MachineID]
		if !exists {
			info = &MachineInfo{MachineID: c.MachineID}
			machineMap[c.MachineID] = info
		}
		info.CardCount++
	}
	for _, s := range sessions {
		if info, ok := machineMap[s.MachineID]; ok {
			if info.LastSeen == nil || s.LastSeen.After(*info.LastSeen) {
				t := s.LastSeen
				info.LastSeen = &t
			}
		}
	}
	for _, entry := range blacklist {
		if entry.Type == "machine" {
			if info, ok := machineMap[entry.Value]; ok {
				info.IsBlacklisted = true
			}
		}
	}
	
	result := make([]MachineInfo, 0, len(machineMap))
	for _, info := range machineMap {
		result = append(result, *info)
	}
	return result
}

func (cm *SQLiteCardManager) GetCardsByMachine(machineID string) []*Card {
	return cm.SearchCards("", "", machineID)
}

func (cm *SQLiteCardManager) bulkUpdateCardsDetailed(codes []string, action string, extendHours int, dryRun bool) (cardops.BulkResult, error) {
	var result cardops.BulkResult
	bulkAction := cardops.BulkAction(action)
	if err := cardops.ValidateBulkAction(bulkAction); err != nil {
		for _, code := range codes {
			result.AddItem(cardops.BulkItemResult{
				Code:    code,
				Status:  cardops.BulkItemFailed,
				Message: err.Error(),
			})
		}
		return result, err
	}

	tx, err := cm.db.Begin()
	if err != nil && !dryRun {
		return result, err
	}
	if !dryRun {
		defer tx.Rollback()
	}

	changed := false
	for _, code := range codes {
		normalized := normalizeCardCode(code)
		c := cm.GetCard(normalized)
		if c == nil {
			result.AddItem(cardops.BulkItemResult{
				Code:    code,
				Status:  cardops.BulkItemSkipped,
				Message: "card not found",
			})
			continue
		}
		if !dryRun {
			switch bulkAction {
			case cardops.BulkDisable:
				tx.Exec(`UPDATE cards SET status = ? WHERE code = ?`, CardDisabled, normalized)
			case cardops.BulkEnable:
				tx.Exec(`UPDATE cards SET status = ? WHERE code = ?`, CardActive, normalized)
			case cardops.BulkExpire:
				tx.Exec(`UPDATE cards SET status = ? WHERE code = ?`, CardExpired, normalized)
			case cardops.BulkExtend:
				expiresAt := c.ExpiresAt.Add(time.Duration(extendHours) * time.Hour)
				status := c.Status
				if status == CardExpired {
					status = CardActive
				}
				tx.Exec(`UPDATE cards SET expires_at = ?, status = ? WHERE code = ?`, expiresAt, status, normalized)
			case cardops.BulkUnbind:
				expiresAt := c.ExpiresAt
				if c.ActivatedAt != nil {
					originalDuration := c.ExpiresAt.Sub(*c.ActivatedAt)
					expiresAt = c.CreatedAt.Add(originalDuration)
				}
				tx.Exec(`UPDATE cards SET machine_id = '', activated_at = NULL, expires_at = ? WHERE code = ?`, expiresAt, normalized)
			}
			changed = true
		}
		result.AddItem(cardops.BulkItemResult{
			Code:   code,
			Status: cardops.BulkItemUpdated,
		})
	}
	
	if !dryRun && changed {
		tx.Commit()
		cm.RecordAudit(AuditEntry{
			Time:   time.Now(),
			Action: "bulk_" + action,
			Detail: fmt.Sprintf("updated=%d skipped=%d failed=%d cards=%s", result.Updated, result.Skipped, result.Failed, strings.Join(codes, ",")),
		})
	}
	return result, nil
}

func (cm *SQLiteCardManager) BulkUpdateCardsDetailed(codes []string, action string, extendHours int) (cardops.BulkResult, error) {
	return cm.bulkUpdateCardsDetailed(codes, action, extendHours, false)
}

func (cm *SQLiteCardManager) PreviewBulkUpdateCardsDetailed(codes []string, action string, extendHours int) (cardops.BulkResult, error) {
	return cm.bulkUpdateCardsDetailed(codes, action, extendHours, true)
}

func (cm *SQLiteCardManager) BulkUpdateCards(codes []string, action string, extendHours int) (int, error) {
	result, err := cm.BulkUpdateCardsDetailed(codes, action, extendHours)
	return result.Updated, err
}

func (cm *SQLiteCardManager) CreateAgent(username, passwordHash, prefix string) (*Agent, error) {
	agents := cm.AllAgents()
	existing := make([]agentsvc.Account, 0, len(agents))
	for _, a := range agents {
		existing = append(existing, accountFromAgent(a))
	}
	account, err := newAccountService().Create(existing, username, passwordHash, prefix)
	if err != nil {
		return nil, err
	}
	agent := agentFromAccount(account)
	_, err = cm.db.Exec(`INSERT INTO agents (id, username, password, prefix, created_at, disabled) VALUES (?, ?, ?, ?, ?, ?)`,
		agent.ID, agent.Username, agent.Password, agent.Prefix, agent.CreatedAt, agent.Disabled)
	if err != nil {
		return nil, err
	}
	cm.RecordAudit(AuditEntry{
		Time:   time.Now(),
		Action: "agent_created",
		Detail: fmt.Sprintf("agent_id=%s username=%s", agent.ID, username),
	})
	return agent, nil
}

func (cm *SQLiteCardManager) UsernameExists(username string) bool {
	var u string
	err := cm.db.QueryRow(`SELECT username FROM agents WHERE username = ?`, username).Scan(&u)
	return err == nil
}

func (cm *SQLiteCardManager) GetAgent(id string) *Agent {
	var a Agent
	err := cm.db.QueryRow(`SELECT id, username, password, prefix, created_at, disabled FROM agents WHERE id = ?`, id).
		Scan(&a.ID, &a.Username, &a.Password, &a.Prefix, &a.CreatedAt, &a.Disabled)
	if err != nil {
		return nil
	}
	return &a
}

func (cm *SQLiteCardManager) UpdateAgentPassword(id, passwordHash string) error {
	agent := cm.GetAgent(id)
	if agent == nil {
		return fmt.Errorf("agent not found")
	}
	account, err := newAccountService().UpdatePassword(accountFromAgent(agent), passwordHash)
	if err != nil {
		return err
	}
	_, err = cm.db.Exec(`UPDATE agents SET password = ? WHERE id = ?`, account.Password, id)
	if err != nil {
		return err
	}
	cm.RecordAudit(AuditEntry{
		Time:    time.Now(),
		Action:  "agent_password_updated",
		AgentID: id,
	})
	return nil
}

func (cm *SQLiteCardManager) AllAgents() []*Agent {
	rows, err := cm.db.Query(`SELECT id, username, password, prefix, created_at, disabled FROM agents`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var res []*Agent
	for rows.Next() {
		var a Agent
		if err := rows.Scan(&a.ID, &a.Username, &a.Password, &a.Prefix, &a.CreatedAt, &a.Disabled); err == nil {
			res = append(res, &a)
		}
	}
	return res
}

func (cm *SQLiteCardManager) UpdateAgentStatus(id string, disabled bool) error {
	agent := cm.GetAgent(id)
	if agent == nil {
		return fmt.Errorf("agent not found")
	}
	account := newAccountService().UpdateStatus(accountFromAgent(agent), disabled)
	_, err := cm.db.Exec(`UPDATE agents SET disabled = ? WHERE id = ?`, account.Disabled, id)
	if err != nil {
		return err
	}
	cm.RecordAudit(AuditEntry{
		Time:   time.Now(),
		Action: "agent_status_changed",
		Detail: fmt.Sprintf("agent_id=%s disabled=%v", id, disabled),
	})
	return nil
}

func (cm *SQLiteCardManager) DeleteAgent(id string) error {
	agent := cm.GetAgent(id)
	if agent == nil {
		return fmt.Errorf("agent not found")
	}
	account := accountFromAgent(agent)
	if err := newAccountService().EnsureCanDelete(&account); err != nil {
		return err
	}
	_, err := cm.db.Exec(`DELETE FROM agents WHERE id = ?`, id)
	if err != nil {
		return err
	}
	cm.RecordAudit(AuditEntry{
		Time:   time.Now(),
		Action: "agent_deleted",
		Detail: fmt.Sprintf("agent_id=%s", id),
	})
	return nil
}

func (cm *SQLiteCardManager) AgentCards(agentID string) []*Card {
	rows, err := cm.db.Query(`SELECT code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions FROM cards WHERE agent_id = ?`, agentID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var res []*Card
	for rows.Next() {
		var c Card
		var act sql.NullTime
		if err := rows.Scan(&c.Code, &c.MachineID, &c.AgentID, &c.CreatedAt, &act, &c.ExpiresAt, &c.Status, &c.Note, &c.MaxSessions); err == nil {
			c.ActivatedAt = parseTime(act)
			res = append(res, &c)
		}
	}
	return res
}

func (cm *SQLiteCardManager) AgentStats(agentID string) (total, active, expired int) {
	rows, err := cm.db.Query(`SELECT expires_at, status FROM cards WHERE agent_id = ?`, agentID)
	if err != nil {
		return 0, 0, 0
	}
	defer rows.Close()
	now := time.Now()
	for rows.Next() {
		var expiresAt time.Time
		var status string
		if err := rows.Scan(&expiresAt, &status); err == nil {
			total++
			if CardStatus(status) == CardActive && now.Before(expiresAt) {
				active++
			}
			if now.After(expiresAt) || CardStatus(status) == CardExpired {
				expired++
			}
		}
	}
	return
}

func (cm *SQLiteCardManager) ImportJSONData(old *JSONCardManager) error {
	tx, err := cm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, c := range old.AllCards() {
		_, err := tx.Exec(`INSERT OR IGNORE INTO cards (code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			c.Code, c.MachineID, c.AgentID, c.CreatedAt, c.ActivatedAt, c.ExpiresAt, c.Status, c.Note, c.MaxSessions)
		if err != nil {
			return err
		}
	}
	for _, s := range old.AllSessions() {
		_, err := tx.Exec(`INSERT OR IGNORE INTO sessions (token, card_code, machine_id, fingerprint, client_version, created_at, last_seen, expires_at, remote_addr) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			s.Token, s.CardCode, s.MachineID, s.Fingerprint, s.ClientVersion, s.CreatedAt, s.LastSeen, s.ExpiresAt, s.RemoteAddr)
		if err != nil {
			return err
		}
	}
	for _, a := range old.AllAgents() {
		_, err := tx.Exec(`INSERT OR IGNORE INTO agents (id, username, password, prefix, created_at, disabled) VALUES (?, ?, ?, ?, ?, ?)`,
			a.ID, a.Username, a.Password, a.Prefix, a.CreatedAt, a.Disabled)
		if err != nil {
			return err
		}
	}
	for _, b := range old.AllBlacklist() {
		_, err := tx.Exec(`INSERT OR IGNORE INTO blacklist (value, type, reason, created_at) VALUES (?, ?, ?, ?)`,
			b.Value, b.Type, b.Reason, b.CreatedAt)
		if err != nil {
			return err
		}
	}
	for _, a := range old.AuditLog() {
		_, err := tx.Exec(`INSERT OR IGNORE INTO audit_log (time, action, card, machine, agent_id, detail, addr) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			a.Time, a.Action, a.Card, a.Machine, a.AgentID, a.Detail, a.Addr)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
