package agents

import "testing"

func TestAccountServiceCreateRejectsDuplicateUsername(t *testing.T) {
	service := NewAccountService(func() (string, error) { return "abc123", nil })
	existing := []Account{{ID: "AGT-existing", Username: "agent1"}}

	_, err := service.Create(existing, "agent1", "hash", "")
	if err == nil {
		t.Fatal("Create should reject duplicate username")
	}
}

func TestAccountServiceCreateBuildsAccount(t *testing.T) {
	service := NewAccountService(func() (string, error) { return "abc123", nil })

	account, err := service.Create(nil, "agent1", "hash", "PX")
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if account.ID != "AGT-abc123" {
		t.Fatalf("ID = %q, want AGT-abc123", account.ID)
	}
	if account.Username != "agent1" {
		t.Fatalf("Username = %q, want agent1", account.Username)
	}
	if account.Password != "hash" {
		t.Fatalf("Password = %q, want hash", account.Password)
	}
	if account.Prefix != "PX" {
		t.Fatalf("Prefix = %q, want PX", account.Prefix)
	}
	if account.CreatedAt.IsZero() {
		t.Fatal("CreatedAt should be set")
	}
}

func TestAccountServiceCreateRejectsEmptyUsername(t *testing.T) {
	service := NewAccountService(func() (string, error) { return "abc123", nil })

	_, err := service.Create(nil, "", "hash", "")
	if err == nil {
		t.Fatal("Create should reject empty username")
	}
}

func TestAccountServiceUpdatePassword(t *testing.T) {
	service := NewAccountService(func() (string, error) { return "abc123", nil })
	account := Account{ID: "AGT-1", Username: "agent1", Password: "old"}

	updated, err := service.UpdatePassword(account, "new")
	if err != nil {
		t.Fatalf("UpdatePassword returned error: %v", err)
	}
	if updated.Password != "new" {
		t.Fatalf("Password = %q, want new", updated.Password)
	}
}

func TestAccountServiceUpdateStatus(t *testing.T) {
	service := NewAccountService(func() (string, error) { return "abc123", nil })
	account := Account{ID: "AGT-1", Username: "agent1"}

	updated := service.UpdateStatus(account, true)
	if !updated.Disabled {
		t.Fatal("Disabled should be true")
	}
}

func TestAccountServiceEnsureCanDelete(t *testing.T) {
	service := NewAccountService(func() (string, error) { return "abc123", nil })

	if err := service.EnsureCanDelete(nil); err == nil {
		t.Fatal("EnsureCanDelete should reject missing account")
	}
	if err := service.EnsureCanDelete(&Account{ID: "AGT-1"}); err != nil {
		t.Fatalf("EnsureCanDelete returned error: %v", err)
	}
}
