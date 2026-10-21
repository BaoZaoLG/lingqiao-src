package cutover

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
)

type contractManifest struct {
	Version int `json:"version"`
	Routes  []struct {
		Module     string   `json:"module"`
		GoPath     string   `json:"go_path"`
		JavaPath   string   `json:"java_path"`
		Methods    []string `json:"methods"`
		Write      bool     `json:"write"`
		Dual       bool     `json:"dual"`
		Permission string   `json:"permission"`
		ClientAuth bool     `json:"client_auth"`
		Auth       string   `json:"auth"`
		ParitySkip bool     `json:"parity_skip"`
	} `json:"routes"`
}

func loadManifest(t *testing.T) contractManifest {
	t.Helper()
	data, err := os.ReadFile("../../../docs/api/java-cutover-contracts.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest contractManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Version != 14 || len(manifest.Routes) == 0 {
		t.Fatalf("invalid contract manifest: %+v", manifest)
	}
	return manifest
}

func TestCutoverMapMatchesSharedContractManifest(t *testing.T) {
	manifest := loadManifest(t)
	for _, route := range manifest.Routes {
		rule, ok := lookupRoute(route.GoPath)
		if !ok {
			t.Errorf("route %s is not mapped", route.GoPath)
			continue
		}
		if rule.javaPath != route.JavaPath {
			t.Errorf("route %s maps to %s, want %s", route.GoPath, rule.javaPath, route.JavaPath)
		}
		if rule.module != route.Module {
			t.Errorf("route %s module = %s, want %s", route.GoPath, rule.module, route.Module)
		}
		if rule.write != route.Write {
			t.Errorf("route %s write = %v, want %v", route.GoPath, rule.write, route.Write)
		}
		if route.Dual {
			if !rule.dual || len(route.Methods) != 2 ||
				route.Methods[0] != http.MethodGet || route.Methods[1] != http.MethodPost {
				t.Errorf("dual route %s methods=%v rule=%+v", route.GoPath, route.Methods, rule)
			}
			continue
		}
		if len(route.Methods) != 1 {
			t.Errorf("route %s must declare exactly one method", route.GoPath)
			continue
		}
		wantMethod := rule.method
		if wantMethod == "" {
			if route.Write {
				wantMethod = http.MethodPost
			} else {
				wantMethod = http.MethodGet
			}
		}
		if route.Methods[0] != wantMethod {
			t.Errorf("route %s method = %s, want %s", route.GoPath, route.Methods[0], wantMethod)
		}
	}
}

func TestReadRoutesRemainReadOnlyInLegacyMapper(t *testing.T) {
	manifest := loadManifest(t)
	for _, route := range manifest.Routes {
		path, ok := mapToJavaPath(route.GoPath)
		isLegacyRead := !route.Write &&
			len(route.Methods) >= 1 &&
			route.Methods[0] == http.MethodGet
		if !isLegacyRead {
			if ok {
				t.Errorf("non-GET route %s must not be exposed as read mapping", route.GoPath)
			}
			continue
		}
		if !ok || path != route.JavaPath {
			t.Errorf("read route %s mapping = %q ok=%v", route.GoPath, path, ok)
		}
	}
}

// TestManifestPermissionsMatchGoRoutes pins the permission each admin route is
// registered with in main.go. A Java controller checking a different permission
// would silently widen or narrow access after cutover.
func TestManifestPermissionsMatchGoRoutes(t *testing.T) {
	goPermissions := map[string]string{
		"/admin/api/admin-users":                              "admins.write",
		"/admin/api/admin-user/update":                        "admins.write",
		"/admin/api/invites":                                  "invites.write",
		"/admin/api/invite/create":                            "invites.write",
		"/admin/api/invite/delete":                            "invites.write",
		"/admin/api/exam/records":                             "exam.read",
		"/admin/api/exam/stats":                               "exam.read",
		"/admin/api/exam/export":                              "exam.read",
		"/admin/api/audit":                                    "audit.read",
		"/admin/api/scripts":                                  "scripts.write",
		"/admin/api/script":                                   "scripts.write",
		"/admin/api/cards":                                    "cards.read",
		"/admin/api/sessions":                                 "sessions.read",
		"/admin/api/agents":                                   "agents.read",
		"/admin/api/releases":                                 "releases.write",
		"/admin/api/releases/RELEASE-ID/publish":              "releases.write",
		"/admin/api/releases/RELEASE-ID/pause":                "releases.write",
		"/admin/api/releases/RELEASE-ID/rollback":             "releases.write",
		"/admin/api/releases/RELEASE-ID/events":               "releases.write",
		"/admin/api/releases/RELEASE-ID/packages":             "releases.write",
		"/admin/api/payloads":                                 "payloads.write",
		"/admin/api/chat/messages":                            "chat.read",
		"/admin/api/announcement":                             "announcement.read",
		"/admin/api/blacklist":                                "system.read",
		"/admin/api/card/generate":                            "cards.write",
		"/admin/api/card/batch-generate":                      "cards.write",
		"/admin/api/card/update":                              "cards.write",
		"/admin/api/card/update-details":                      "cards.write",
		"/admin/api/cards/bulk":                               "cards.bulk",
		"/admin/api/cards/export":                             "cards.read",
		"/admin/api/cards/import":                             "cards.write",
		"/admin/api/control/authorizations":                   "cards.read",
		"/admin/api/control/authorizations/AUTH-CODE":         "cards.read",
		"/admin/api/control/authorizations/AUTH-CODE/impact":  "cards.read",
		"/admin/api/control/authorizations/AUTH-CODE/actions": "cards.read",
		"/admin/api/control/authorizations/bulk/impact":       "cards.bulk",
		"/admin/api/control/authorizations/bulk/actions":      "cards.bulk",
		"/admin/api/force-logout":                             "sessions.force_logout",
		"/admin/api/agent/update":                             "agents.write",
		"/admin/api/agent/cards":                              "agents.read",
		"/admin/api/machines":                                 "agents.read",
		"/admin/api/machine/cards":                            "agents.read",
		"/admin/api/scripts/create":                           "scripts.write",
		"/admin/api/scripts/publish":                          "scripts.write",
		"/admin/api/chat/delete":                              "chat.write",
		"/admin/api/chat/system":                              "chat.write",
		"/admin/api/chat/mute":                                "chat.write",
		"/admin/api/payloads/upload-dll":                      "payloads.write",
		"/admin/api/payloads/manage":                          "payloads.write",
		"/admin/api/payload/upload":                           "payloads.write",
	}

	manifest := loadManifest(t)
	seen := map[string]bool{}
	for _, route := range manifest.Routes {
		want, tracked := goPermissions[route.GoPath]
		if !tracked {
			continue
		}
		seen[route.GoPath] = true
		if route.Permission != want {
			t.Errorf("route %s permission = %q, want %q", route.GoPath, route.Permission, want)
		}
	}
	for path := range goPermissions {
		if !seen[path] {
			t.Errorf("route %s is missing from the shared manifest", path)
		}
	}
}
