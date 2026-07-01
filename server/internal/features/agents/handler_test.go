package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

type mockCardOps struct {
	cards []CardSnapshot
}

func (m *mockCardOps) GenerateCard(duration time.Duration, note string, maxSessions int, agentID string) (CardSnapshot, error) {
	c := CardSnapshot{
		Code:        "GEN123",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(duration),
		Status:      "active",
		Note:        note,
		MaxSessions: maxSessions,
		AgentID:     agentID,
	}
	m.cards = append(m.cards, c)
	return c, nil
}

func (m *mockCardOps) BatchGenerateCards(count int, duration time.Duration, note string, maxSessions int, agentID string) ([]CardSnapshot, error) {
	var res []CardSnapshot
	for i := 0; i < count; i++ {
		c := CardSnapshot{
			Code:        fmt.Sprintf("BATCH%d", i),
			CreatedAt:   time.Now(),
			ExpiresAt:   time.Now().Add(duration),
			Status:      "active",
			Note:        note,
			MaxSessions: maxSessions,
			AgentID:     agentID,
		}
		m.cards = append(m.cards, c)
		res = append(res, c)
	}
	return res, nil
}

func (m *mockCardOps) AllCards() []CardSnapshot {
	return m.cards
}

func (m *mockCardOps) AgentCards(agentID string) []CardSnapshot {
	var res []CardSnapshot
	for _, c := range m.cards {
		if c.AgentID == agentID {
			res = append(res, c)
		}
	}
	return res
}

func (m *mockCardOps) AgentStats(agentID string) (total, active, expired int) {
	for _, c := range m.cards {
		if c.AgentID == agentID {
			total++
			if c.Status == "active" {
				active++
			} else {
				expired++
			}
		}
	}
	return total, active, expired
}

func TestAgentHandler_EndpointsIntegration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteRepository(db)
	svc := NewService(repo, time.Now)
	inviteSvc := NewInviteService(db)
	cardOps := &mockCardOps{}

	tempDir, err := os.MkdirTemp("", "agent_handler_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	handler := NewAgentHandler(svc, cardOps, cardOps, inviteSvc, 1*time.Hour, tempDir)

	// Create invite code
	invite, err := inviteSvc.Create(1, "admin")
	if err != nil {
		t.Fatalf("failed to create invite: %v", err)
	}

	// 1. Test /api/register
	regReqBody, _ := json.Marshal(map[string]string{
		"username":    "agent_test",
		"password":    "securepassword123",
		"invite_code": invite.Code,
		"prefix":      "TST",
	})
	req := httptest.NewRequest("POST", "/api/register", bytes.NewReader(regReqBody))
	w := httptest.NewRecorder()
	handler.HandleRegister(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected register response 200, got %d. body: %s", w.Code, w.Body.String())
	}

	// Verify invite was used
	invs, _ := inviteSvc.List()
	if len(invs) != 1 || invs[0].UseCount != 1 {
		t.Errorf("expected invite to be used once, got state: %+v", invs)
	}

	// 2. Test /api/login
	loginReqBody, _ := json.Marshal(map[string]string{
		"username": "agent_test",
		"password": "securepassword123",
	})
	req = httptest.NewRequest("POST", "/api/login", bytes.NewReader(loginReqBody))
	w = httptest.NewRecorder()
	handler.HandleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected login response 200, got %d. body: %s", w.Code, w.Body.String())
	}

	var loginRes map[string]string
	json.Unmarshal(w.Body.Bytes(), &loginRes)
	token := loginRes["token"]
	agentID := loginRes["agent_id"]
	if token == "" || agentID == "" {
		t.Fatalf("expected token and agent_id, got token=%s agent_id=%s", token, agentID)
	}

	// Get cookie from login response
	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "agent_token" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected agent_token cookie to be set")
	}

	// 3. Test /api/dashboard (Requires session validation via agentAuth middleware)
	req = httptest.NewRequest("GET", "/api/dashboard", nil)
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()

	// Execute handler via the agentAuth middleware
	handler.agentAuth(handler.HandleDashboard)(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected dashboard response 200, got %d. body: %s", w.Code, w.Body.String())
	}

	var stats map[string]int
	json.Unmarshal(w.Body.Bytes(), &stats)
	if stats["total_cards"] != 0 {
		t.Errorf("expected 0 cards starting out, got: %+v", stats)
	}

	// 4. Test /api/card/generate
	genReqBody, _ := json.Marshal(map[string]interface{}{
		"duration_hours": 24,
		"max_sessions":   2,
		"note":           "Test Card 1",
	})
	req = httptest.NewRequest("POST", "/api/card/generate", bytes.NewReader(genReqBody))
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()

	handler.agentAuth(handler.HandleGenerateCard)(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected card generate response 200, got %d. body: %s", w.Code, w.Body.String())
	}

	var genRes struct {
		Card CardSnapshot `json:"card"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &genRes); err != nil {
		t.Fatalf("failed to parse card generate response: %v", err)
	}
	cardObj := genRes.Card
	diff := cardObj.ExpiresAt.Sub(cardObj.CreatedAt)
	if diff.Round(time.Second) != 24*time.Hour {
		t.Errorf("expected card duration to be 24h, got %v", diff)
	}

	// 5. Test /api/card/batch-generate
	batchReqBody, _ := json.Marshal(map[string]interface{}{
		"count":          3,
		"duration_hours": 168,
		"max_sessions":   1,
		"note":           "Batch test cards",
	})
	req = httptest.NewRequest("POST", "/api/card/batch-generate", bytes.NewReader(batchReqBody))
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()

	handler.agentAuth(handler.HandleBatchGenerate)(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected batch generate response 200, got %d. body: %s", w.Code, w.Body.String())
	}

	var batchRes struct {
		Cards []CardSnapshot `json:"cards"`
		Count int            `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &batchRes); err != nil {
		t.Fatalf("failed to parse batch generate response: %v", err)
	}
	batchCards := batchRes.Cards
	if len(batchCards) != 3 {
		t.Errorf("expected 3 batch cards, got %d", len(batchCards))
	} else {
		diffBatch := batchCards[0].ExpiresAt.Sub(batchCards[0].CreatedAt)
		if diffBatch.Round(time.Second) != 168*time.Hour {
			t.Errorf("expected batch card duration to be 168h, got %v", diffBatch)
		}
	}

	// 6. Verify Dashboard stats after generation
	req = httptest.NewRequest("GET", "/api/dashboard", nil)
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	handler.agentAuth(handler.HandleDashboard)(w, req)
	json.Unmarshal(w.Body.Bytes(), &stats)
	if stats["total_cards"] != 4 { // 1 generated + 3 batch generated
		t.Errorf("expected 4 cards total, got: %+v", stats)
	}

	// 7. Test /api/cards list
	req = httptest.NewRequest("GET", "/api/cards", nil)
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	handler.agentAuth(handler.HandleListCards)(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected list cards response 200, got %d. body: %s", w.Code, w.Body.String())
	}

	var listRes map[string][]CardSnapshot
	json.Unmarshal(w.Body.Bytes(), &listRes)
	if len(listRes["cards"]) != 4 {
		t.Errorf("expected 4 cards in list, got %d", len(listRes["cards"]))
	}

	// 8. Test /api/password (change password)
	pwReqBody, _ := json.Marshal(map[string]string{
		"old_password": "securepassword123",
		"new_password": "newsecurepassword456",
	})
	req = httptest.NewRequest("POST", "/api/password", bytes.NewReader(pwReqBody))
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	handler.agentAuth(handler.HandlePasswordChange)(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected password change response 200, got %d. body: %s", w.Code, w.Body.String())
	}

	// 9. Login with old password should now fail
	req = httptest.NewRequest("POST", "/api/login", bytes.NewReader(loginReqBody))
	w = httptest.NewRecorder()
	handler.HandleLogin(w, req)
	if w.Code == http.StatusOK {
		t.Error("expected login with old password to fail, but it succeeded")
	}

	// Login with new password should succeed
	newLoginReqBody, _ := json.Marshal(map[string]string{
		"username": "agent_test",
		"password": "newsecurepassword456",
	})
	req = httptest.NewRequest("POST", "/api/login", bytes.NewReader(newLoginReqBody))
	w = httptest.NewRecorder()
	handler.HandleLogin(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected login with new password to succeed, got %d. body: %s", w.Code, w.Body.String())
	}
}

func TestAgentHandler_NegativeScenarios(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewSQLiteRepository(db)
	svc := NewService(repo, time.Now)
	inviteSvc := NewInviteService(db)
	cardOps := &mockCardOps{}

	tempDir, err := os.MkdirTemp("", "agent_handler_neg_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	handler := NewAgentHandler(svc, cardOps, cardOps, inviteSvc, 1*time.Hour, tempDir)

	// Create invite
	invite, _ := inviteSvc.Create(1, "admin")

	// 1. Register with invalid invite code
	regReqBody, _ := json.Marshal(map[string]string{
		"username":    "agent_neg",
		"password":    "password123",
		"invite_code": "INVALID-CODE",
		"prefix":      "TST",
	})
	req := httptest.NewRequest("POST", "/api/register", bytes.NewReader(regReqBody))
	w := httptest.NewRecorder()
	handler.HandleRegister(w, req)
	if w.Code == http.StatusOK {
		t.Error("expected registration with invalid invite code to fail")
	}

	// 2. Register valid agent
	regReqBody, _ = json.Marshal(map[string]string{
		"username":    "agent_neg",
		"password":    "password123",
		"invite_code": invite.Code,
		"prefix":      "TST",
	})
	req = httptest.NewRequest("POST", "/api/register", bytes.NewReader(regReqBody))
	w = httptest.NewRecorder()
	handler.HandleRegister(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected registration to succeed, got %d. body: %s", w.Code, w.Body.String())
	}

	// 3. Register duplicate username
	invite2, _ := inviteSvc.Create(1, "admin")
	regReqBody, _ = json.Marshal(map[string]string{
		"username":    "agent_neg",
		"password":    "password123",
		"invite_code": invite2.Code,
		"prefix":      "TST",
	})
	req = httptest.NewRequest("POST", "/api/register", bytes.NewReader(regReqBody))
	w = httptest.NewRecorder()
	handler.HandleRegister(w, req)
	if w.Code == http.StatusOK {
		t.Error("expected duplicate username registration to fail")
	}

	// 4. Access protected endpoint without login
	req = httptest.NewRequest("GET", "/api/dashboard", nil)
	w = httptest.NewRecorder()
	handler.agentAuth(handler.HandleDashboard)(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for dashboard, got %d", w.Code)
	}

	req = httptest.NewRequest("POST", "/api/card/generate", bytes.NewReader([]byte("{}")))
	w = httptest.NewRecorder()
	handler.agentAuth(handler.HandleGenerateCard)(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for generate card, got %d", w.Code)
	}

	// 5. Login to get session cookie
	loginReqBody, _ := json.Marshal(map[string]string{
		"username": "agent_neg",
		"password": "password123",
	})
	req = httptest.NewRequest("POST", "/api/login", bytes.NewReader(loginReqBody))
	w = httptest.NewRecorder()
	handler.HandleLogin(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %s", w.Body.String())
	}
	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "agent_token" {
			sessionCookie = c
			break
		}
	}

	// Change password with wrong old password
	pwReqBody, _ := json.Marshal(map[string]string{
		"old_password": "wrong_password",
		"new_password": "newsecurepassword123",
	})
	req = httptest.NewRequest("POST", "/api/password", bytes.NewReader(pwReqBody))
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	handler.agentAuth(handler.HandlePasswordChange)(w, req)
	if w.Code == http.StatusOK {
		t.Error("expected password change with wrong old password to fail")
	}

	// 6. Disable agent
	all, _ := repo.AllAgents(context.Background())
	var testAgentID string
	for _, a := range all {
		if a.Username == "agent_neg" {
			testAgentID = a.ID
			break
		}
	}
	if testAgentID == "" {
		t.Fatal("could not find registered agent in database")
	}

	err = svc.SetAgentStatus(context.Background(), testAgentID, true)
	if err != nil {
		t.Fatalf("failed to disable agent: %v", err)
	}

	// Try login with disabled account
	req = httptest.NewRequest("POST", "/api/login", bytes.NewReader(loginReqBody))
	w = httptest.NewRecorder()
	handler.HandleLogin(w, req)
	if w.Code == http.StatusOK {
		t.Error("expected login with disabled account to fail")
	}

	// Try accessing dashboard with existing session cookie after being disabled (should fail due to db disabled status check)
	req = httptest.NewRequest("GET", "/api/dashboard", nil)
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	handler.agentAuth(handler.HandleDashboard)(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized for disabled agent session access, got %d", w.Code)
	}
}
