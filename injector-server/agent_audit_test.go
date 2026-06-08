package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	auditsvc "github.com/lingqiao/server/internal/audit"
)

func TestAgentGenerateCardUsesUnifiedAuditRecorder(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)
	recorder := auditsvc.NewRecorder()
	cm.SetAuditRecorder(recorder)
	agent, err := cm.CreateAgent("agent-audit", hashPassword("pass123"), "")
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAgentHandler(cm)

	req := httptest.NewRequest(http.MethodPost, "/api/card/generate", strings.NewReader(`{"duration_hours":24,"max_sessions":1,"note":"audit"}`))
	req.Header.Set("X-Agent-ID", agent.ID)
	rr := httptest.NewRecorder()

	handler.HandleGenerateCard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	events := recorder.Query(auditsvc.Filter{Action: "agent_card_generated"})
	if len(events) != 1 {
		t.Fatalf("agent_card_generated recorder events = %#v, want one", events)
	}
	if events[0].AgentID != agent.ID {
		t.Fatalf("event AgentID = %q, want %q", events[0].AgentID, agent.ID)
	}
}
