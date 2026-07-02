package releases

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lingqiao/server/internal/auth"
	"github.com/lingqiao/server/internal/features/sessions"
	releasesvc "github.com/lingqiao/server/internal/releases"
)

type releaseTestSessionStore struct {
	session sessions.Session
	card    sessions.CardSnapshot
}

func (s *releaseTestSessionStore) RunInTx(ctx context.Context, fn func(sessions.Tx) error) error {
	return fn(s)
}

func (s *releaseTestSessionStore) GetCard(ctx context.Context, code string) (sessions.CardSnapshot, error) {
	if code != s.card.Code {
		return sessions.CardSnapshot{}, sessions.ErrCardNotFound
	}
	return s.card, nil
}

func (s *releaseTestSessionStore) IsCardBlacklisted(ctx context.Context, code string) (bool, error) {
	return false, nil
}

func (s *releaseTestSessionStore) ListSessionsByCard(ctx context.Context, code string) ([]sessions.Session, error) {
	return nil, nil
}

func (s *releaseTestSessionStore) InsertSession(ctx context.Context, session sessions.Session) error {
	return nil
}

func (s *releaseTestSessionStore) GetSession(ctx context.Context, token string) (sessions.Session, error) {
	if token != s.session.Token {
		return sessions.Session{}, sessions.ErrSessionNotFound
	}
	return s.session, nil
}

func (s *releaseTestSessionStore) UpdateSession(ctx context.Context, session sessions.Session) error {
	return nil
}

func (s *releaseTestSessionStore) DeleteSession(ctx context.Context, token string) error {
	return nil
}

func (s *releaseTestSessionStore) UpdateCardActivation(ctx context.Context, card sessions.CardSnapshot) error {
	return nil
}

func (s *releaseTestSessionStore) UnbindCardPreservingRemaining(ctx context.Context, cardCode string, now time.Time) error {
	return nil
}

func (s *releaseTestSessionStore) ListSessions(ctx context.Context, filter sessions.ListFilter, now time.Time) ([]sessions.Session, int, error) {
	return nil, 0, nil
}

func (s *releaseTestSessionStore) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	return 0, nil
}

func newReleaseTestSessionService(now time.Time) *sessions.Service {
	store := &releaseTestSessionStore{
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
	return sessions.NewService(store, time.Hour, func() string { return "unused" }, func() time.Time { return now })
}

func signReleaseRequestForTest(t *testing.T, req *http.Request, body string) {
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

func TestHandleUpdateEventRejectsUnboundSession(t *testing.T) {
	now := time.Now().UTC()
	store, err := releasesvc.OpenSQLiteStore(t.TempDir() + "/releases.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	h := NewHandler(&releaseService{store: store}, newReleaseTestSessionService(now), nil)
	body := `{"release_id":"rel-1","version":"1.0.0","machine_id":"machine-1","card_code":"CARD-1","type":"download_started"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/update/events", strings.NewReader(body))
	signReleaseRequestForTest(t, req, body)
	rr := httptest.NewRecorder()

	h.HandleUpdateEvent(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
	}
}

func TestPublishReleaseRequiresPackage(t *testing.T) {
	store, err := releasesvc.OpenSQLiteStore(t.TempDir() + "/releases.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	release := releasesvc.Release{
		ID:             "rel-1",
		Version:        "2.0.0",
		Channel:        releasesvc.ChannelStable,
		Status:         releasesvc.StatusDraft,
		RolloutPercent: 100,
		CreatedAt:      time.Now().UTC(),
	}
	if err := store.SaveRelease(context.Background(), release); err != nil {
		t.Fatal(err)
	}
	dataDir := t.TempDir()
	scriptDir := filepath.Join(dataDir, "scripts")
	if err := os.MkdirAll(scriptDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(scriptDir, "active.json"), []byte(`{"id":"script-1","version":"1.0.0","sha256":"abc123"}`), 0600); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(&releaseService{store: store, dataDir: dataDir}, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/admin/api/releases/rel-1/publish", nil)
	rr := httptest.NewRecorder()

	h.updateReleaseStatus(rr, req, h.svc, "rel-1", releasesvc.StatusPublished)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}
