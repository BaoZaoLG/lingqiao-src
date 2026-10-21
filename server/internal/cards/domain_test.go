package cards

import (
	"errors"
	"testing"
	"time"
)

func TestValidateBulkAction(t *testing.T) {
	for _, action := range []BulkAction{BulkDisable, BulkEnable, BulkExpire, BulkExtend, BulkUnbind, BulkDelete} {
		if err := ValidateBulkAction(action); err != nil {
			t.Fatalf("action %s: %v", action, err)
		}
	}
	if err := ValidateBulkAction("nope"); err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestBulkResultAddItem(t *testing.T) {
	var r BulkResult
	r.AddItem(BulkItemResult{Code: "A", Status: BulkItemUpdated})
	r.AddItem(BulkItemResult{Code: "B", Status: BulkItemSkipped})
	r.AddItem(BulkItemResult{Code: "C", Status: BulkItemFailed})
	if r.Updated != 1 || r.Skipped != 1 || r.Failed != 1 || len(r.Items) != 3 {
		t.Fatalf("unexpected counters: %+v", r)
	}
}

func TestSameCardCode(t *testing.T) {
	if !sameCardCode("ab-cd", "ABCD") {
		t.Fatal("expected normalized match")
	}
	if sameCardCode("AAA", "BBB") {
		t.Fatal("expected mismatch")
	}
}

func TestSessionServiceActivateAndHeartbeat(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	svc := NewSessionService(func() time.Time { return now }, 0) // default 4h

	created := now.Add(-24 * time.Hour)
	expires := created.Add(30 * 24 * time.Hour)
	card := Card{
		Code:        "CARD-001",
		CreatedAt:   created,
		ExpiresAt:   expires,
		Status:      StatusActive,
		MaxSessions: 1,
	}

	// first activation binds machine and starts duration from now
	cardOut, sess, err := svc.Activate(ActivationInput{
		Card:          card,
		Token:         "tok-1",
		MachineID:     "m1",
		Fingerprint:   "fp",
		RemoteAddr:    "1.1.1.1",
		ClientVersion: "1.0",
	})
	if err != nil {
		t.Fatalf("Activate: %v", err)
	}
	if cardOut.MachineID != "m1" || cardOut.ActivatedAt == nil {
		t.Fatalf("binding failed: %+v", cardOut)
	}
	if !cardOut.ExpiresAt.Equal(now.Add(30 * 24 * time.Hour)) {
		t.Fatalf("expires recalculated = %v", cardOut.ExpiresAt)
	}
	if !sess.ExpiresAt.Equal(now.Add(4 * time.Hour)) {
		t.Fatalf("session TTL = %v, want 4h", sess.ExpiresAt)
	}

	// machine mismatch with maxSessions <= 1; cardOut has MaxSessions 1 and MachineID m1
	_, _, err = svc.Activate(ActivationInput{
		Card:      cardOut,
		Token:     "tok-2",
		MachineID: "m2",
	})
	if !errors.Is(err, ErrCardBoundToOtherMachine) {
		t.Fatalf("want bound error, got %v", err)
	}

	// max sessions
	card2 := card
	card2.MaxSessions = 1
	card2.MachineID = ""
	active := []Session{{CardCode: "CARD001", ExpiresAt: now.Add(time.Hour)}}
	_, _, err = svc.Activate(ActivationInput{
		Card:           card2,
		Token:          "tok-3",
		MachineID:      "m1",
		ActiveSessions: active,
	})
	if !errors.Is(err, ErrMaxSessionsReached) {
		t.Fatalf("want max sessions, got %v", err)
	}

	// heartbeat success
	hb, err := svc.Heartbeat(HeartbeatInput{
		Session:       sess,
		Card:          cardOut,
		MachineID:     "m1",
		RemoteAddr:    "2.2.2.2",
		ClientVersion: "1.1",
	})
	if err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}
	if hb.RemoteAddr != "2.2.2.2" || hb.ClientVersion != "1.1" {
		t.Fatalf("heartbeat update failed: %+v", hb)
	}

	// machine mismatch
	_, err = svc.Heartbeat(HeartbeatInput{Session: sess, Card: cardOut, MachineID: "other"})
	if !errors.Is(err, ErrMachineMismatch) {
		t.Fatalf("want machine mismatch, got %v", err)
	}

	// expired session
	expiredSess := sess
	expiredSess.ExpiresAt = now.Add(-time.Second)
	_, err = svc.Heartbeat(HeartbeatInput{Session: expiredSess, Card: cardOut, MachineID: "m1"})
	if !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("want session expired, got %v", err)
	}

	// disabled card
	disabled := cardOut
	disabled.Status = StatusDisabled
	_, err = svc.Heartbeat(HeartbeatInput{Session: sess, Card: disabled, MachineID: "m1"})
	if !errors.Is(err, ErrCardNoLongerValid) {
		t.Fatalf("want card invalid, got %v", err)
	}
}
