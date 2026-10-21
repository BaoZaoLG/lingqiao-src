package exam

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lingqiao/server/internal/auth"
	"github.com/lingqiao/server/internal/features/sessions"
	_ "modernc.org/sqlite"
)

type examTestSessionStore struct {
	session sessions.Session
	card    sessions.CardSnapshot
}

func (s *examTestSessionStore) RunInTx(ctx context.Context, fn func(sessions.Tx) error) error {
	return fn(s)
}

func (s *examTestSessionStore) GetCard(ctx context.Context, code string) (sessions.CardSnapshot, error) {
	if code != s.card.Code {
		return sessions.CardSnapshot{}, sessions.ErrCardNotFound
	}
	return s.card, nil
}

func (s *examTestSessionStore) IsCardBlacklisted(ctx context.Context, code string) (bool, error) {
	return false, nil
}

func (s *examTestSessionStore) ListSessionsByCard(ctx context.Context, code string) ([]sessions.Session, error) {
	return nil, nil
}

func (s *examTestSessionStore) InsertSession(ctx context.Context, session sessions.Session) error {
	return nil
}

func (s *examTestSessionStore) GetSession(ctx context.Context, token string) (sessions.Session, error) {
	if token != s.session.Token {
		return sessions.Session{}, sessions.ErrSessionNotFound
	}
	return s.session, nil
}

func (s *examTestSessionStore) UpdateSession(ctx context.Context, session sessions.Session) error {
	return nil
}

func (s *examTestSessionStore) DeleteSession(ctx context.Context, token string) error {
	return nil
}

func (s *examTestSessionStore) UpdateCardActivation(ctx context.Context, card sessions.CardSnapshot) error {
	return nil
}

func (s *examTestSessionStore) UnbindCardPreservingRemaining(ctx context.Context, cardCode string, now time.Time) error {
	return nil
}

func (s *examTestSessionStore) ListSessions(ctx context.Context, filter sessions.ListFilter, now time.Time) ([]sessions.Session, int, error) {
	return nil, 0, nil
}

func (s *examTestSessionStore) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	return 0, nil
}

func newExamTestHandlerWithClosedDB(t *testing.T) *Handler {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close sqlite: %v", err)
	}

	now := time.Now().UTC()
	store := &examTestSessionStore{
		session: sessions.Session{
			Token:     "tok-1",
			CardCode:  "CARD-1",
			MachineID: "machine-1",
			ExpiresAt: now.Add(time.Hour),
		},
		card: sessions.CardSnapshot{
			Code:      "CARD-1",
			Status:    "active",
			ExpiresAt: now.Add(time.Hour),
		},
	}
	svc := sessions.NewService(store, time.Hour, func() string { return "unused" }, func() time.Time { return now })
	return NewHandler(NewRepository(db), svc)
}

func signExamRequestForTest(t *testing.T, req *http.Request, body string) {
	t.Helper()
	t.Setenv("HMAC_SECRET", "test-secret")
	auth.Init(t.TempDir())
	auth.ConfigureClients()
	key, err := auth.GetDerivedKey("injector_v1")
	if err != nil {
		t.Fatal(err)
	}
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce := fmt.Sprintf("nonce-%d", time.Now().UnixNano())
	req.Header.Set("X-Client-ID", "injector_v1")
	req.Header.Set("X-Timestamp", ts)
	req.Header.Set("X-Nonce", nonce)
	req.Header.Set("X-HMAC-Signature", auth.SignHMAC(string(key), ts+"|"+nonce+"|"+body))
}

func TestInsertBatchRollsBackOnFailure(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE exam_records (
		id INTEGER PRIMARY KEY, card_code TEXT NOT NULL, machine_id TEXT NOT NULL,
		exam_id TEXT NOT NULL, course_id TEXT NOT NULL, class_id TEXT NOT NULL,
		question_id TEXT NOT NULL UNIQUE, question_text TEXT NOT NULL, question_type TEXT NOT NULL,
		options_json TEXT NOT NULL, provided_answer TEXT NOT NULL, correct_answer TEXT NOT NULL,
		model_name TEXT NOT NULL, confidence REAL NOT NULL, answer_time_ms INTEGER NOT NULL,
		fill_ok INTEGER NOT NULL, created_at DATETIME NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	records := []Record{
		{CardCode: "C", MachineID: "M", ExamID: "E", QuestionID: "duplicate", CreatedAt: now},
		{CardCode: "C", MachineID: "M", ExamID: "E", QuestionID: "duplicate", CreatedAt: now},
	}
	if err := NewRepository(db).InsertBatch(context.Background(), records); err == nil {
		t.Fatal("InsertBatch should fail on duplicate question_id")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM exam_records`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("batch was partially committed: count=%d", count)
	}
}

func TestHandleCollectReturnsServerErrorWhenInsertFails(t *testing.T) {
	h := newExamTestHandlerWithClosedDB(t)
	body := `{"session_token":"tok-1","exam_id":"exam-1","questions":[{"question_id":"q1","provided_answer":"A","fill_ok":true}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/exam/collect", strings.NewReader(body))
	rr := httptest.NewRecorder()

	h.HandleCollect(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusInternalServerError, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "database is closed") {
		t.Fatalf("response leaked database error: %s", rr.Body.String())
	}
}

func TestHandleReportReturnsServerErrorWhenInsertFails(t *testing.T) {
	h := newExamTestHandlerWithClosedDB(t)
	body := `{"session_token":"tok-1","exam_id":"exam-1","questions":[{"question_id":"q1","provided_answer":"A","fill_ok":true}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/exam/report", strings.NewReader(body))
	signExamRequestForTest(t, req, body)
	rr := httptest.NewRecorder()

	h.HandleReport(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusInternalServerError, rr.Body.String())
	}
	if strings.Contains(rr.Body.String(), "database is closed") {
		t.Fatalf("response leaked database error: %s", rr.Body.String())
	}
}

func TestSpreadsheetSafeProtectsExportCells(t *testing.T) {
	for _, input := range []string{"=1+1", "+cmd", "-2", "@name"} {
		if got := spreadsheetSafe(input); got != "'"+input {
			t.Fatalf("spreadsheetSafe(%q) = %q", input, got)
		}
	}
	if got := spreadsheetSafe("normal"); got != "normal" {
		t.Fatalf("normal value changed: %q", got)
	}
}
