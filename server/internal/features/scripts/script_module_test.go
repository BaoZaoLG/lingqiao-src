package scripts

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lingqiao/server/internal/auth"
	"github.com/lingqiao/server/internal/features/sessions"
)

type testSessionStore struct {
	session sessions.Session
	card    sessions.CardSnapshot
}

func (s *testSessionStore) RunInTx(ctx context.Context, fn func(sessions.Tx) error) error {
	return fn(s)
}

func (s *testSessionStore) GetCard(ctx context.Context, code string) (sessions.CardSnapshot, error) {
	if code != s.card.Code {
		return sessions.CardSnapshot{}, sessions.ErrCardNotFound
	}
	return s.card, nil
}

func (s *testSessionStore) IsCardBlacklisted(ctx context.Context, code string) (bool, error) {
	return false, nil
}

func (s *testSessionStore) ListSessionsByCard(ctx context.Context, code string) ([]sessions.Session, error) {
	return nil, nil
}

func (s *testSessionStore) InsertSession(ctx context.Context, session sessions.Session) error {
	return nil
}

func (s *testSessionStore) GetSession(ctx context.Context, token string) (sessions.Session, error) {
	if token != s.session.Token {
		return sessions.Session{}, sessions.ErrSessionNotFound
	}
	return s.session, nil
}

func (s *testSessionStore) UpdateSession(ctx context.Context, session sessions.Session) error {
	return nil
}

func (s *testSessionStore) DeleteSession(ctx context.Context, token string) error {
	return nil
}

func (s *testSessionStore) UpdateCardActivation(ctx context.Context, card sessions.CardSnapshot) error {
	return nil
}

func (s *testSessionStore) UnbindCardPreservingRemaining(ctx context.Context, cardCode string, now time.Time) error {
	return nil
}

func (s *testSessionStore) ListSessions(ctx context.Context, filter sessions.ListFilter, now time.Time) ([]sessions.Session, int, error) {
	return nil, 0, nil
}

func (s *testSessionStore) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	return 0, nil
}

func newScriptTestHandler(t *testing.T) *Handler {
	t.Helper()
	now := time.Now().UTC()
	store := &testSessionStore{
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
	return NewHandler(nil, sessions.NewService(store, time.Hour, func() string { return "unused" }, func() time.Time { return now }))
}

func signRequestForTest(t *testing.T, req *http.Request, body string) {
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

func TestHandleScriptDownloadRejectsMissingHMAC(t *testing.T) {
	h := newScriptTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/script", nil)
	req.Header.Set("X-Session-Token", "tok-1")
	req.Header.Set("X-Machine-ID", "machine-1")
	req.Header.Set("X-Card-Code", "CARD-1")
	rr := httptest.NewRecorder()

	h.HandleScriptDownload(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestHandleScriptDownloadRejectsQueryToken(t *testing.T) {
	dataDir := t.TempDir()
	Init(dataDir)
	if _, err := SaveActiveScriptModule("1.0.0", "console.log('ok')"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dataDir) })

	h := newScriptTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/script?token=tok-1", nil)
	req.Header.Set("X-Machine-ID", "machine-1")
	req.Header.Set("X-Card-Code", "CARD-1")
	signRequestForTest(t, req, "/api/v1/script")
	rr := httptest.NewRecorder()

	h.HandleScriptDownload(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHandleScriptDownloadJSONAcceptAndSizeHeader(t *testing.T) {
	dataDir := t.TempDir()
	Init(dataDir)
	module, err := SaveActiveScriptModule("1.0.0", "console.log('ok')")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dataDir) })

	h := newScriptTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/script", nil)
	req.Header.Set("X-Session-Token", "tok-1")
	req.Header.Set("X-Machine-ID", "machine-1")
	req.Header.Set("X-Card-Code", "CARD-1")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	signRequestForTest(t, req, "/api/v1/script")
	rr := httptest.NewRecorder()

	h.HandleScriptDownload(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if rr.Header().Get("X-Script-Size") != fmt.Sprintf("%d", module.Size) {
		t.Fatalf("X-Script-Size = %q, want %d", rr.Header().Get("X-Script-Size"), module.Size)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
}
