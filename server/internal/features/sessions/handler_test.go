package sessions_test

import (
	"context"

	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lingqiao/server/internal/features/sessions"
)

// Fake readers and checkers
type fakeReader struct {
	req sessions.ClientRequest
	err error
}

func (f *fakeReader) ReadSignedRequest(r *http.Request) (sessions.ClientRequest, error) {
	return f.req, f.err
}

type fakeVersionCheck struct {
	blocked bool
	data    map[string]interface{}
}

func (f *fakeVersionCheck) CheckVersion(w http.ResponseWriter, clientVersion string) (bool, map[string]interface{}) {
	return f.blocked, f.data
}

func fakeClientIP(r *http.Request) string {
	return "127.0.0.1"
}

func setupHandlerTest() (*sessions.Handler, sessions.Store, *fakeReader, *fakeVersionCheck) {
	repo := &fakeStore{
		cards:       make(map[string]sessions.CardSnapshot),
		sessions:    make(map[string]sessions.Session),
		blacklisted: make(map[string]bool),
	}
	svc := sessions.NewService(repo, time.Hour, func() string { return "test-token" }, nil)
	reader := &fakeReader{}
	vcheck := &fakeVersionCheck{}
	handler := sessions.NewHandler(svc, reader, vcheck, fakeClientIP)
	return handler, repo, reader, vcheck
}

// fakeStore implements sessions.Store
type fakeStore struct {
	cards       map[string]sessions.CardSnapshot
	sessions    map[string]sessions.Session
	blacklisted map[string]bool
}

func (f *fakeStore) RunInTx(ctx context.Context, fn func(sessions.Tx) error) error {
	return fn(&fakeTx{store: f})
}

func (f *fakeStore) GetSession(ctx context.Context, token string) (sessions.Session, error) {
	s, ok := f.sessions[token]
	if !ok {
		return sessions.Session{}, sessions.ErrSessionNotFound
	}
	return s, nil
}

func (f *fakeStore) ListSessions(ctx context.Context, filter sessions.ListFilter, now time.Time) ([]sessions.Session, int, error) {
	var res []sessions.Session
	for _, s := range f.sessions {
		if filter.CardCode != "" && s.CardCode != filter.CardCode {
			continue
		}
		if filter.MachineID != "" && s.MachineID != filter.MachineID {
			continue
		}
		res = append(res, s)
	}
	return res, len(res), nil
}

func (f *fakeStore) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	return 0, nil
}

// fakeTx implements sessions.Tx
type fakeTx struct {
	store *fakeStore
}

func (tx *fakeTx) GetCard(ctx context.Context, code string) (sessions.CardSnapshot, error) {
	c, ok := tx.store.cards[code]
	if !ok {
		return sessions.CardSnapshot{}, sessions.ErrCardNotFound
	}
	return c, nil
}

func (tx *fakeTx) IsCardBlacklisted(ctx context.Context, code string) (bool, error) {
	return tx.store.blacklisted[code], nil
}

func (tx *fakeTx) ListSessionsByCard(ctx context.Context, code string) ([]sessions.Session, error) {
	var res []sessions.Session
	for _, s := range tx.store.sessions {
		if s.CardCode == code {
			res = append(res, s)
		}
	}
	return res, nil
}

func (tx *fakeTx) InsertSession(ctx context.Context, session sessions.Session) error {
	tx.store.sessions[session.Token] = session
	return nil
}

func (tx *fakeTx) GetSession(ctx context.Context, token string) (sessions.Session, error) {
	return tx.store.GetSession(ctx, token)
}

func (tx *fakeTx) UpdateSession(ctx context.Context, session sessions.Session) error {
	tx.store.sessions[session.Token] = session
	return nil
}

func (tx *fakeTx) DeleteSession(ctx context.Context, token string) error {
	delete(tx.store.sessions, token)
	return nil
}

func (tx *fakeTx) UpdateCardActivation(ctx context.Context, card sessions.CardSnapshot) error {
	tx.store.cards[card.Code] = card
	return nil
}

func (tx *fakeTx) UnbindCardPreservingRemaining(ctx context.Context, cardCode string, now time.Time) error {
	return nil
}

func TestHandler_Activate(t *testing.T) {
	handler, repo, reader, _ := setupHandlerTest()
	fs := repo.(*fakeStore)
	fs.cards["TESTCARD"] = sessions.CardSnapshot{
		Code: "TESTCARD",

		Status:      "active",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		MaxSessions: 1,
	}

	reader.req = sessions.ClientRequest{
		Card:          "TESTCARD",
		MachineID:     "mac1",
		Fingerprint:   "fp1",
		ClientVersion: "1.0",
	}

	req := httptest.NewRequest("POST", "/api/v1/activate", nil)
	rr := httptest.NewRecorder()

	handler.HandleActivate(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d. body: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"session_token":"test-token"`) {
		t.Errorf("expected session_token in body")
	}
	if !strings.Contains(rr.Body.String(), `"status":"ok"`) {
		t.Errorf("expected status: ok in body")
	}
}

func TestHandler_BadMethod(t *testing.T) {
	handler, _, _, _ := setupHandlerTest()

	req := httptest.NewRequest("GET", "/api/v1/activate", nil)
	rr := httptest.NewRecorder()
	handler.HandleActivate(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rr.Code)
	}
}

func TestHandler_Activate_MaxSessions(t *testing.T) {
	handler, repo, reader, _ := setupHandlerTest()
	fs := repo.(*fakeStore)
	fs.cards["TESTCARD"] = sessions.CardSnapshot{
		Code:        "TESTCARD",
		Status:      "active",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		MaxSessions: 1,
	}
	fs.sessions["tok_existing"] = sessions.Session{
		Token:     "tok_existing",
		CardCode:  "TESTCARD",
		MachineID: "mac1",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}

	reader.req = sessions.ClientRequest{
		Card:          "TESTCARD",
		MachineID:     "mac2",
		Fingerprint:   "fp1",
		ClientVersion: "1.0",
	}

	req := httptest.NewRequest("POST", "/api/v1/activate", nil)
	rr := httptest.NewRecorder()

	handler.HandleActivate(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for bound to other machine / max sessions, got %d. body: %s", rr.Code, rr.Body.String())
	}
}

func TestHandler_Activate_Blacklisted(t *testing.T) {
	handler, repo, reader, _ := setupHandlerTest()
	fs := repo.(*fakeStore)
	fs.cards["TESTCARD"] = sessions.CardSnapshot{
		Code:        "TESTCARD",
		Status:      "active",
		ExpiresAt:   time.Now().Add(24 * time.Hour),
		MaxSessions: 1,
	}
	fs.blacklisted["TESTCARD"] = true

	reader.req = sessions.ClientRequest{
		Card:          "TESTCARD",
		MachineID:     "mac1",
		Fingerprint:   "fp1",
		ClientVersion: "1.0",
	}

	req := httptest.NewRequest("POST", "/api/v1/activate", nil)
	rr := httptest.NewRecorder()

	handler.HandleActivate(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for blacklisted card, got %d. body: %s", rr.Code, rr.Body.String())
	}
}
