package admin

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func newTestService(t *testing.T) (*Service, *RepositorySQLite) {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	repo, err := NewRepositorySQLite(db)
	if err != nil {
		t.Fatalf("create repository: %v", err)
	}
	svc, err := NewService(repo, time.Hour, t.TempDir())
	if err != nil {
		t.Fatalf("create service: %v", err)
	}
	return svc, repo
}

func TestServiceCreateAdminRejectsUnknownRole(t *testing.T) {
	svc, repo := newTestService(t)

	err := svc.CreateAdmin(context.Background(), &AdminUser{
		ID:           "adm_invalid",
		Username:     "invalid-role",
		PasswordHash: "secret1",
		Role:         Role("unknown"),
		CreatedAt:    time.Now().UTC(),
	})
	if !errors.Is(err, ErrInvalidAdminRole) {
		t.Fatalf("CreateAdmin error = %v, want %v", err, ErrInvalidAdminRole)
	}

	count, err := repo.CountAdmins(context.Background())
	if err != nil {
		t.Fatalf("CountAdmins: %v", err)
	}
	if count != 1 {
		t.Fatalf("admin count = %d, want seeded owner only", count)
	}
}

func TestServiceUpdateAdminProtectsLastActiveOwner(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	owner, err := svc.GetAdmin(ctx, "adm_owner")
	if err != nil {
		t.Fatalf("GetAdmin owner: %v", err)
	}
	owner.Disabled = true
	if err := svc.UpdateAdmin(ctx, owner); !errors.Is(err, ErrLastActiveOwner) {
		t.Fatalf("UpdateAdmin error = %v, want %v", err, ErrLastActiveOwner)
	}

	if err := svc.CreateAdmin(ctx, &AdminUser{
		ID:           "adm_owner_2",
		Username:     "owner-2",
		PasswordHash: "secret2",
		Role:         RoleOwner,
		CreatedAt:    time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateAdmin second owner: %v", err)
	}

	if err := svc.UpdateAdmin(ctx, owner); err != nil {
		t.Fatalf("UpdateAdmin with second owner: %v", err)
	}
}

func TestServiceLoginSessionAndPassword(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	// seed owner password is random; reset via Create path: update owner hash
	owner, err := svc.GetAdmin(ctx, "adm_owner")
	if err != nil {
		t.Fatal(err)
	}
	hash, err := svc.HashPassword("owner-pass")
	if err != nil {
		t.Fatal(err)
	}
	owner.PasswordHash = hash
	if err := svc.UpdateAdmin(ctx, owner); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.VerifyLogin(ctx, "admin", "wrong"); err == nil {
		t.Fatal("expected wrong password")
	}
	admin, err := svc.VerifyLogin(ctx, "admin", "owner-pass")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	token, maxAge, err := svc.CreateSession(ctx, admin.ID, "127.0.0.1", "test-agent")
	if err != nil || token == "" || maxAge <= 0 {
		t.Fatalf("CreateSession: %v token=%q maxAge=%d", err, token, maxAge)
	}
	got, ok := svc.CheckSession(ctx, token)
	if !ok || got.ID != admin.ID {
		t.Fatalf("CheckSession failed: ok=%v got=%+v", ok, got)
	}
	if _, ok := svc.CheckSession(ctx, ""); ok {
		t.Fatal("empty token")
	}
	if err := svc.ChangePassword(ctx, admin.ID, "owner-pass", "new-pass1"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}
	if err := svc.ChangePassword(ctx, admin.ID, "bad", "new-pass2"); !errors.Is(err, ErrWrongOldPassword) {
		t.Fatalf("want ErrWrongOldPassword, got %v", err)
	}
	if _, err := svc.VerifyLogin(ctx, "admin", "new-pass1"); err != nil {
		t.Fatalf("relogin: %v", err)
	}
	admins, err := svc.ListAdmins(ctx)
	if err != nil || len(admins) < 1 {
		t.Fatalf("ListAdmins: %v %d", err, len(admins))
	}
}

func TestRepositorySessionCRUD(t *testing.T) {
	_, repo := newTestService(t)
	ctx := context.Background()
	now := time.Now().UTC()
	sess := &AdminSession{
		TokenHash:  "hash1",
		AdminID:    "adm_owner",
		CreatedAt:  now,
		ExpiresAt:  now.Add(time.Hour),
		RemoteAddr: "1.1.1.1",
		UserAgent:  "ua",
	}
	if err := repo.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetSession(ctx, "hash1")
	if err != nil || got.AdminID != "adm_owner" {
		t.Fatalf("GetSession: %v %+v", err, got)
	}
	seen := now.Add(time.Minute)
	got.LastSeenAt = &seen
	got.ExpiresAt = now.Add(2 * time.Hour)
	if err := repo.UpdateSession(ctx, got); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteSession(ctx, "hash1"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.GetSession(ctx, "hash1"); err == nil {
		t.Fatal("expected missing session")
	}
}
