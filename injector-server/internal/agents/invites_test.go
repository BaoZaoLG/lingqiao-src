package agents

import (
	"testing"

	"github.com/lingqiao/server/internal/storage"
)

func TestInviteServiceCreateUseAndExhaustion(t *testing.T) {
	svc := NewInviteService(storage.NewJSONStore(t.TempDir()))

	invite, err := svc.Create(1, "admin")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if invite.Code == "" {
		t.Fatal("invite code is empty")
	}

	if err := svc.ValidateAndUse(invite.Code, "agent-1"); err != nil {
		t.Fatalf("ValidateAndUse returned error: %v", err)
	}
	if err := svc.ValidateAndUse(invite.Code, "agent-2"); err == nil {
		t.Fatal("ValidateAndUse should reject exhausted invite")
	}

	codes, err := svc.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if codes[0].UseCount != 1 {
		t.Fatalf("UseCount = %d, want 1", codes[0].UseCount)
	}
	if codes[0].UsedBy != "agent-1" {
		t.Fatalf("UsedBy = %q, want agent-1", codes[0].UsedBy)
	}
}

func TestInviteServiceDelete(t *testing.T) {
	svc := NewInviteService(storage.NewJSONStore(t.TempDir()))
	invite, err := svc.Create(0, "admin")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	deleted, err := svc.Delete(invite.Code)
	if err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if !deleted {
		t.Fatal("Delete should report true for existing invite")
	}

	codes, err := svc.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(codes) != 0 {
		t.Fatalf("len(codes) = %d, want 0", len(codes))
	}
}

func TestInviteServiceReplace(t *testing.T) {
	svc := NewInviteService(storage.NewJSONStore(t.TempDir()))

	err := svc.Replace([]InviteCode{
		{Code: "LQ-EXISTING", CreatedBy: "admin", MaxUses: 2},
	})
	if err != nil {
		t.Fatalf("Replace returned error: %v", err)
	}

	codes, err := svc.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(codes) != 1 {
		t.Fatalf("len(codes) = %d, want 1", len(codes))
	}
	if codes[0].Code != "LQ-EXISTING" {
		t.Fatalf("Code = %q, want LQ-EXISTING", codes[0].Code)
	}
}
