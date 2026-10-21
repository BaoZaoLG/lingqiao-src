package main

import (
	"os"
	"strings"
	"testing"
)

func TestJavaAdminControlPlaneRoutesAreWrappedByCutover(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	required := []string{
		`mux.HandleFunc("/admin/api/login", cutoverRouter.Wrap("admin"`,
		`mux.HandleFunc("/admin/api/password", auth(cutoverRouter.Wrap("admin"`,
		`mux.HandleFunc("/admin/api/dashboard", auth(cutoverRouter.Wrap("admin"`,
		`mux.HandleFunc("/admin/api/stats", auth(cutoverRouter.Wrap("admin"`,
		`mux.HandleFunc("/admin/api/ops/overview", auth(cutoverRouter.Wrap("admin"`,
		`mux.HandleFunc("/admin/api/modules/overview", auth(cutoverRouter.Wrap("admin"`,
		`mux.HandleFunc("/admin/api/admin-users", authP("admins.write", cutoverRouter.Wrap("admin"`,
		`mux.HandleFunc("/admin/api/admin-user/update", authP("admins.write", cutoverRouter.Wrap("admin"`,
		`mux.HandleFunc("/admin/api/invites", authP("invites.write", cutoverRouter.Wrap("admin"`,
		`mux.HandleFunc("/admin/api/invite/create", authP("invites.write", cutoverRouter.Wrap("admin"`,
		`mux.HandleFunc("/admin/api/invite/delete", authP("invites.write", cutoverRouter.Wrap("admin"`,
		`mux.HandleFunc("/admin/api/script", authP("scripts.write", cutoverRouter.Wrap("scripts"`,
		`cutoverRouter.Wrap("announcement", h.HandleAnnouncementSet)`,
	}
	for _, snippet := range required {
		if !strings.Contains(text, snippet) {
			t.Errorf("main.go is missing admin cutover registration %q", snippet)
		}
	}
}
