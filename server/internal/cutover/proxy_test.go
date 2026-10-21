package cutover

import (
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/lingqiao/server/internal/config"
)

func TestWrapFallsBackWhenDisabled(t *testing.T) {
	router, err := New(config.CutoverConfig{Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	handler := router.Wrap("exam", func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = io.WriteString(w, "local")
	})
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/admin/api/exam/stats", nil))
	if !called || rec.Body.String() != "local" {
		t.Fatalf("expected local handler, body=%q", rec.Body.String())
	}
}

func TestWrapProxiesMappedGetRoutes(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/java/admin/api/exam/stats" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"status":"ok","source":"java"}`)
	}))
	defer upstream.Close()

	router, err := New(config.CutoverConfig{Enabled: true, BaseURL: upstream.URL, Exam: true})
	if err != nil {
		t.Fatal(err)
	}
	handler := router.Wrap("exam", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("local handler should not run")
	})
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/admin/api/exam/stats", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"source":"java"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestWriteRouteStaysLocalUntilWriteFlagEnabled(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"source":"java"}`)
	}))
	defer upstream.Close()

	router, err := New(config.CutoverConfig{Enabled: true, BaseURL: upstream.URL, Cards: true})
	if err != nil {
		t.Fatal(err)
	}
	local := 0
	handler := router.Wrap("cards", func(w http.ResponseWriter, r *http.Request) {
		local++
		_, _ = io.WriteString(w, "local")
	})
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost, "/admin/api/card/generate", strings.NewReader("{}")))
	if local != 1 || rec.Body.String() != "local" {
		t.Fatalf("write route must stay local: local=%d body=%q", local, rec.Body.String())
	}
}

func TestWriteRouteProxiesWhenWriteFlagEnabled(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/java/admin/api/cards/generate" {
			t.Fatalf("method=%s path=%s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"status":"ok","source":"java"}`)
	}))
	defer upstream.Close()

	router, err := New(config.CutoverConfig{
		Enabled:    true,
		BaseURL:    upstream.URL,
		Cards:      true,
		CardsWrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := router.Wrap("cards", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("local handler should not run")
	})
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost, "/admin/api/card/generate", strings.NewReader("{}")))
	if !strings.Contains(rec.Body.String(), `"source":"java"`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestLegacyPayloadUploadRespectsPayloadWriteCutover(t *testing.T) {
	var received string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Method + " " + r.URL.Path + " " + r.Header.Get("X-Upload-Key")
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	defer upstream.Close()

	readOnly, err := New(config.CutoverConfig{
		Enabled: true, BaseURL: upstream.URL, Payloads: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	local := 0
	readOnly.Wrap("payloads", func(w http.ResponseWriter, r *http.Request) { local++ })(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/admin/api/payload/upload", strings.NewReader("{}")),
	)
	if local != 1 {
		t.Fatalf("legacy upload must stay local without write flag: local=%d", local)
	}

	writeEnabled, err := New(config.CutoverConfig{
		Enabled: true, BaseURL: upstream.URL, Payloads: true, PayloadsWrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(
		http.MethodPost, "/admin/api/payload/upload", strings.NewReader("{}"))
	request.Header.Set("X-Upload-Key", "shared-secret")
	writeEnabled.Wrap("payloads", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("enabled legacy upload must not stay local")
	})(httptest.NewRecorder(), request)

	if received != "POST /java/admin/api/payload/upload shared-secret" {
		t.Fatalf("received = %q", received)
	}
}

func TestCardTransferRoutesRespectReadAndWriteFlags(t *testing.T) {
	var paths []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	defer upstream.Close()

	readOnly, err := New(config.CutoverConfig{
		Enabled: true, BaseURL: upstream.URL, Cards: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	localWrites := 0
	readOnlyHandler := readOnly.Wrap("cards", func(w http.ResponseWriter, r *http.Request) {
		localWrites++
	})
	readOnlyHandler(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/admin/api/cards/export", nil),
	)
	readOnlyHandler(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/admin/api/cards/import", strings.NewReader("{}")),
	)
	if localWrites != 1 {
		t.Fatalf("import must remain local until write cutover: local=%d", localWrites)
	}

	writeEnabled, err := New(config.CutoverConfig{
		Enabled: true, BaseURL: upstream.URL, Cards: true, CardsWrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeHandler := writeEnabled.Wrap("cards", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("enabled card mutation must not stay local")
	})
	writeHandler(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/admin/api/cards/bulk", strings.NewReader("{}")),
	)
	writeHandler(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/admin/api/cards/import", strings.NewReader("{}")),
	)

	if !slices.Equal(paths, []string{
		"GET /java/admin/api/cards/export",
		"POST /java/admin/api/cards/bulk",
		"POST /java/admin/api/cards/import",
	}) {
		t.Fatalf("proxied paths = %#v", paths)
	}
}

func TestAuthorizationRoutesSeparatePreviewFromMutationCutover(t *testing.T) {
	var requests []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.RequestURI())
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	defer upstream.Close()

	readOnly, err := New(config.CutoverConfig{
		Enabled: true, BaseURL: upstream.URL, Cards: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	local := 0
	readHandler := readOnly.Wrap("cards", func(w http.ResponseWriter, r *http.Request) {
		local++
	})
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodGet, "/admin/api/control/authorizations", nil),
		httptest.NewRequest(http.MethodGet, "/admin/api/control/authorizations/AUTH-CODE", nil),
		httptest.NewRequest(http.MethodPost, "/admin/api/control/authorizations/AUTH-CODE/impact", strings.NewReader("{}")),
		httptest.NewRequest(http.MethodPost, "/admin/api/control/authorizations/bulk/impact", strings.NewReader("{}")),
		httptest.NewRequest(http.MethodPost, "/admin/api/control/authorizations/AUTH-CODE/actions", strings.NewReader("{}")),
		httptest.NewRequest(http.MethodPost, "/admin/api/control/authorizations/bulk/actions", strings.NewReader("{}")),
		httptest.NewRequest(http.MethodGet, "/admin/api/control/authorizations/AUTH-CODE/impact", nil),
	} {
		readHandler(httptest.NewRecorder(), request)
	}
	if local != 3 {
		t.Fatalf("mutations/wrong method must stay local without write flag: local=%d", local)
	}

	writeEnabled, err := New(config.CutoverConfig{
		Enabled: true, BaseURL: upstream.URL, Cards: true, CardsWrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeHandler := writeEnabled.Wrap("cards", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("mapped authorization mutation must not stay local")
	})
	writeHandler(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/admin/api/control/authorizations/AUTH-CODE/actions", strings.NewReader("{}")),
	)
	writeHandler(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/admin/api/control/authorizations/bulk/actions", strings.NewReader("{}")),
	)

	if !slices.Equal(requests, []string{
		"GET /java/admin/api/control/authorizations",
		"GET /java/admin/api/control/authorizations/AUTH-CODE",
		"POST /java/admin/api/control/authorizations/AUTH-CODE/impact",
		"POST /java/admin/api/control/authorizations/bulk/impact",
		"POST /java/admin/api/control/authorizations/AUTH-CODE/actions",
		"POST /java/admin/api/control/authorizations/bulk/actions",
	}) {
		t.Fatalf("proxied requests = %#v", requests)
	}
}

func TestClientUpdateRoutesRespectReleaseWriteBoundary(t *testing.T) {
	var requests []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	defer upstream.Close()

	readOnly, err := New(config.CutoverConfig{
		Enabled: true, BaseURL: upstream.URL, Releases: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	local := 0
	handler := readOnly.Wrap("releases", func(w http.ResponseWriter, r *http.Request) {
		local++
	})
	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/update/check", strings.NewReader("{}")))
	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/update/events", strings.NewReader("{}")))
	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/update/package/pkg-1", nil))
	if local != 2 {
		t.Fatalf("POST update routes must stay local without write flag: local=%d", local)
	}

	writeEnabled, err := New(config.CutoverConfig{
		Enabled: true, BaseURL: upstream.URL, Releases: true, ReleasesWrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeHandler := writeEnabled.Wrap("releases", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("mapped client update route must not stay local")
	})
	writeHandler(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/update/check", strings.NewReader("{}")))
	writeHandler(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/v1/update/events", strings.NewReader("{}")))

	if !slices.Equal(requests, []string{
		"GET /java/api/v1/update/package/pkg-1",
		"POST /java/api/v1/update/check",
		"POST /java/api/v1/update/events",
	}) {
		t.Fatalf("proxied requests = %#v", requests)
	}
	if _, ok := lookupRoute("/api/v1/update/package/../secret"); ok {
		t.Fatal("package traversal path must not be mapped")
	}
}

func TestRemainingClientRoutesRespectModuleAndWriteBoundaries(t *testing.T) {
	var requests []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	defer upstream.Close()

	readOnly, err := New(config.CutoverConfig{
		Enabled:      true,
		BaseURL:      upstream.URL,
		Sessions:     true,
		Announcement: true,
		Chat:         true,
		Payloads:     true,
		Scripts:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	local := 0
	handlerByModule := func(module string) http.HandlerFunc {
		return readOnly.Wrap(module, func(w http.ResponseWriter, r *http.Request) { local++ })
	}
	for _, item := range []struct {
		module string
		method string
		path   string
	}{
		{"announcement", http.MethodGet, "/api/v1/announcement"},
		{"payloads", http.MethodGet, "/api/v1/dll"},
		{"scripts", http.MethodGet, "/api/v1/script"},
		{"payloads", http.MethodPost, "/api/v1/key-exchange"},
		{"payloads", http.MethodGet, "/api/v1/payload/payload-1/info"},
		{"payloads", http.MethodGet, "/api/v1/payload/payload-1/chunk/0"},
		{"sessions", http.MethodPost, "/api/v1/activate"},
		{"chat", http.MethodPost, "/api/v1/chat/messages"},
	} {
		handlerByModule(item.module)(
			httptest.NewRecorder(),
			httptest.NewRequest(item.method, item.path, strings.NewReader("{}")),
		)
	}
	if local != 2 {
		t.Fatalf("session/chat mutations must stay local without write flags: local=%d", local)
	}

	writeEnabled, err := New(config.CutoverConfig{
		Enabled:       true,
		BaseURL:       upstream.URL,
		Sessions:      true,
		SessionsWrite: true,
		Chat:          true,
		ChatWrite:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeEnabled.Wrap("sessions", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("session mutation must be proxied")
	})(httptest.NewRecorder(), httptest.NewRequest(
		http.MethodPost, "/api/v1/heartbeat", strings.NewReader("{}")))
	writeEnabled.Wrap("chat", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("chat mutation must be proxied")
	})(httptest.NewRecorder(), httptest.NewRequest(
		http.MethodPost, "/api/v1/chat/send", strings.NewReader("{}")))

	if !slices.Equal(requests, []string{
		"GET /java/api/v1/announcement",
		"GET /java/api/v1/dll",
		"GET /java/api/v1/script",
		"POST /java/api/v1/key-exchange",
		"GET /java/api/v1/payload/payload-1/info",
		"GET /java/api/v1/payload/payload-1/chunk/0",
		"POST /java/api/v1/heartbeat",
		"POST /java/api/v1/chat/send",
	}) {
		t.Fatalf("proxied requests = %#v", requests)
	}
	if _, ok := lookupRoute("/api/v1/payload/../info"); ok {
		t.Fatal("payload traversal path must not be mapped")
	}
	if _, ok := lookupRoute("/api/v1/payload/payload-1/chunk/-1"); ok {
		t.Fatal("negative payload chunk must not be mapped")
	}
}

func TestAdminControlPlaneRoutesRespectReadAndWriteFlags(t *testing.T) {
	var requests []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	defer upstream.Close()

	readOnly, err := New(config.CutoverConfig{
		Enabled: true, BaseURL: upstream.URL, Admin: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	local := 0
	handler := readOnly.Wrap("admin", func(w http.ResponseWriter, r *http.Request) { local++ })
	for _, item := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/admin/api/dashboard"},
		{http.MethodGet, "/admin/api/admin-users"},
		{http.MethodGet, "/admin/api/invites"},
		{http.MethodPost, "/admin/api/login"},
		{http.MethodPost, "/admin/api/admin-users"},
		{http.MethodPost, "/admin/api/invite/create"},
	} {
		handler(httptest.NewRecorder(), httptest.NewRequest(
			item.method, item.path, strings.NewReader("{}")))
	}
	if local != 3 {
		t.Fatalf("admin mutations must stay local without write flag: local=%d", local)
	}

	writeEnabled, err := New(config.CutoverConfig{
		Enabled: true, BaseURL: upstream.URL, Admin: true, AdminWrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeHandler := writeEnabled.Wrap("admin", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("enabled admin route must not stay local")
	})
	for _, path := range []string{
		"/admin/api/login",
		"/admin/api/password",
		"/admin/api/admin-user/update",
		"/admin/api/invite/delete",
	} {
		writeHandler(httptest.NewRecorder(), httptest.NewRequest(
			http.MethodPost, path, strings.NewReader("{}")))
	}

	if !slices.Equal(requests, []string{
		"GET /java/admin/api/dashboard",
		"GET /java/admin/api/admin-users",
		"GET /java/admin/api/invites",
		"POST /java/admin/api/login",
		"POST /java/admin/api/password",
		"POST /java/admin/api/admin-user/update",
		"POST /java/admin/api/invite/delete",
	}) {
		t.Fatalf("proxied requests = %#v", requests)
	}
}

func TestReleaseAdminDynamicRoutesRespectReadAndWriteFlags(t *testing.T) {
	var requests []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	defer upstream.Close()

	readOnly, err := New(config.CutoverConfig{
		Enabled: true, BaseURL: upstream.URL, Releases: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	local := 0
	handler := readOnly.Wrap("releases", func(w http.ResponseWriter, r *http.Request) {
		local++
	})
	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/admin/api/releases/rel-1/events", nil))
	for _, action := range []string{"publish", "pause", "rollback", "packages"} {
		handler(httptest.NewRecorder(), httptest.NewRequest(
			http.MethodPost, "/admin/api/releases/rel-1/"+action, strings.NewReader("{}")))
	}
	if local != 4 {
		t.Fatalf("release mutations must stay local without write flag: local=%d", local)
	}

	writeEnabled, err := New(config.CutoverConfig{
		Enabled: true, BaseURL: upstream.URL, Releases: true, ReleasesWrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeHandler := writeEnabled.Wrap("releases", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("mapped release action must not stay local")
	})
	for _, action := range []string{"publish", "pause", "rollback", "packages"} {
		writeHandler(httptest.NewRecorder(), httptest.NewRequest(
			http.MethodPost, "/admin/api/releases/rel-1/"+action, strings.NewReader("{}")))
	}

	if !slices.Equal(requests, []string{
		"GET /java/admin/api/releases/rel-1/events",
		"POST /java/admin/api/releases/rel-1/publish",
		"POST /java/admin/api/releases/rel-1/pause",
		"POST /java/admin/api/releases/rel-1/rollback",
		"POST /java/admin/api/releases/rel-1/packages",
	}) {
		t.Fatalf("proxied requests = %#v", requests)
	}
	if _, ok := lookupRoute("/admin/api/releases/../publish"); ok {
		t.Fatal("release traversal path must not be mapped")
	}
	if _, ok := lookupRoute("/admin/api/releases/rel-1/unknown"); ok {
		t.Fatal("unknown release action must not be mapped")
	}
}

func TestReadRouteRejectsNonReadMethods(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream must not receive read-route mutations")
	}))
	defer upstream.Close()

	router, err := New(config.CutoverConfig{Enabled: true, BaseURL: upstream.URL, Sessions: true})
	if err != nil {
		t.Fatal(err)
	}
	local := 0
	handler := router.Wrap("sessions", func(w http.ResponseWriter, r *http.Request) {
		local++
	})
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodDelete, "/admin/api/sessions", nil))
	if local != 1 {
		t.Fatalf("local = %d", local)
	}
}

func TestModuleMismatchStaysLocal(t *testing.T) {
	router, err := New(config.CutoverConfig{Enabled: true, BaseURL: "http://127.0.0.1:1", Exam: true})
	if err != nil {
		t.Fatal(err)
	}
	local := 0
	handler := router.Wrap("exam", func(w http.ResponseWriter, r *http.Request) {
		local++
	})
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/admin/api/sessions", nil))
	if local != 1 {
		t.Fatalf("local = %d", local)
	}
}

func TestLookupRouteRejectsScriptMutationPaths(t *testing.T) {
	legacy, ok := lookupRoute("/admin/api/script")
	if !ok || !legacy.dual || legacy.javaPath != "/java/admin/api/script" {
		t.Fatalf("legacy rule=%+v ok=%v", legacy, ok)
	}
	rule, ok := lookupRoute("/admin/api/scripts/create")
	if !ok || !rule.write || rule.javaPath != "/java/admin/api/scripts/create" {
		t.Fatalf("create rule=%+v ok=%v", rule, ok)
	}
	if _, ok := lookupRoute("/admin/api/scripts/../etc"); ok {
		t.Fatal("traversal path must not be mapped")
	}
	activate, ok := lookupRoute("/admin/api/scripts/v1/activate")
	if !ok || activate.method != http.MethodPost || !activate.write {
		t.Fatalf("activate rule=%+v ok=%v", activate, ok)
	}
	del, ok := lookupRoute("/admin/api/scripts/v1/delete")
	if !ok || del.method != http.MethodDelete || !del.write {
		t.Fatalf("delete rule=%+v ok=%v", del, ok)
	}
	read, ok := lookupRoute("/admin/api/scripts/v1")
	if !ok || read.write || read.javaPath != "/java/admin/api/scripts/v1" {
		t.Fatalf("read rule=%+v ok=%v", read, ok)
	}
}

func TestScriptDeleteRequiresDeleteMethodAndWriteFlag(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.Path != "/java/admin/api/scripts/v1/delete" {
			t.Fatalf("method=%s path=%s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"source":"java"}`)
	}))
	defer upstream.Close()

	disabled, err := New(config.CutoverConfig{Enabled: true, BaseURL: upstream.URL, Scripts: true})
	if err != nil {
		t.Fatal(err)
	}
	local := 0
	handler := disabled.Wrap("scripts", func(w http.ResponseWriter, r *http.Request) { local++ })
	handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodDelete, "/admin/api/scripts/v1/delete", nil))
	if local != 1 {
		t.Fatalf("delete must stay local without write flag: local=%d", local)
	}

	enabled, err := New(config.CutoverConfig{
		Enabled:      true,
		BaseURL:      upstream.URL,
		Scripts:      true,
		ScriptsWrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	proxied := enabled.Wrap("scripts", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("local handler should not run")
	})
	rec := httptest.NewRecorder()
	proxied(rec, httptest.NewRequest(http.MethodDelete, "/admin/api/scripts/v1/delete", nil))
	if !strings.Contains(rec.Body.String(), `"source":"java"`) {
		t.Fatalf("body=%s", rec.Body.String())
	}

	postRec := httptest.NewRecorder()
	postLocal := 0
	enabled.Wrap("scripts", func(w http.ResponseWriter, r *http.Request) { postLocal++ })(
		postRec, httptest.NewRequest(http.MethodPost, "/admin/api/scripts/v1/delete", nil))
	if postLocal != 1 {
		t.Fatalf("POST on delete route must stay local: local=%d", postLocal)
	}
}

func TestAgentPortalReadProxiesWhenModuleEnabled(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/java/agent/api/dashboard" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"status":"ok","source":"java"}`)
	}))
	defer upstream.Close()

	router, err := New(config.CutoverConfig{Enabled: true, BaseURL: upstream.URL, AgentPortal: true})
	if err != nil {
		t.Fatal(err)
	}
	handler := router.Wrap("agent-portal", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("local handler should not run")
	})
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/api/dashboard", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"source":"java"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAgentAdminResourceRoutesProxyWithReadFlag(t *testing.T) {
	var paths []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	}))
	defer upstream.Close()

	router, err := New(config.CutoverConfig{
		Enabled: true, BaseURL: upstream.URL, Agents: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := router.Wrap("agents", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("mapped agent read must not stay local")
	})
	for _, path := range []string{
		"/admin/api/agent/cards?agent_id=A1",
		"/admin/api/machines",
		"/admin/api/machine/cards?id=M1",
	} {
		handler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}
	if !slices.Equal(paths, []string{
		"/java/admin/api/agent/cards?agent_id=A1",
		"/java/admin/api/machines",
		"/java/admin/api/machine/cards?id=M1",
	}) {
		t.Fatalf("proxied paths = %#v", paths)
	}
}

func TestAgentPortalLoginStaysLocalUntilWriteFlag(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"source":"java"}`)
	}))
	defer upstream.Close()

	router, err := New(config.CutoverConfig{Enabled: true, BaseURL: upstream.URL, AgentPortal: true})
	if err != nil {
		t.Fatal(err)
	}
	local := 0
	handler := router.Wrap("agent-portal", func(w http.ResponseWriter, r *http.Request) {
		local++
		_, _ = io.WriteString(w, "local")
	})
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader("{}")))
	if local != 1 || rec.Body.String() != "local" {
		t.Fatalf("login must stay local: local=%d body=%q", local, rec.Body.String())
	}
}

func TestAgentPortalLoginProxiesWhenWriteFlagEnabled(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/java/agent/api/login" {
			t.Fatalf("method=%s path=%s", r.Method, r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"status":"ok","source":"java"}`)
	}))
	defer upstream.Close()

	router, err := New(config.CutoverConfig{
		Enabled: true, BaseURL: upstream.URL, AgentPortal: true, AgentPortalWrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := router.Wrap("agent-portal", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("local handler should not run")
	})
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader("{}")))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"source":"java"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAgentPortalUnmappedPathStaysLocal(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream must not be reached")
	}))
	defer upstream.Close()

	router, err := New(config.CutoverConfig{
		Enabled: true, BaseURL: upstream.URL, AgentPortal: true, AgentPortalWrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	local := 0
	handler := router.Wrap("agent-portal", func(w http.ResponseWriter, r *http.Request) {
		local++
	})
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodPost, "/api/unknown", strings.NewReader("{}")))
	handler(rec, httptest.NewRequest(http.MethodGet, "/agent/index.html", nil))
	if local != 2 {
		t.Fatalf("unmapped agent paths must stay local: local=%d", local)
	}
}

func TestAgentPortalCardRoutesRespectReadAndWriteFlags(t *testing.T) {
	var paths []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.Path)
		_, _ = io.WriteString(w, `{"status":"ok","source":"java"}`)
	}))
	defer upstream.Close()

	readOnly, err := New(config.CutoverConfig{
		Enabled: true, BaseURL: upstream.URL, AgentPortal: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	localWrites := 0
	readOnlyHandler := readOnly.Wrap("agent-portal", func(w http.ResponseWriter, r *http.Request) {
		localWrites++
	})
	readOnlyHandler(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/cards", nil))
	readOnlyHandler(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/api/card/generate", strings.NewReader("{}")),
	)
	if localWrites != 1 {
		t.Fatalf("card write must stay local until write flag: local=%d", localWrites)
	}

	writeEnabled, err := New(config.CutoverConfig{
		Enabled: true, BaseURL: upstream.URL, AgentPortal: true, AgentPortalWrite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeEnabledHandler := writeEnabled.Wrap("agent-portal", func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("enabled card write must not stay local")
	})
	writeEnabledHandler(
		httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/api/card/batch-generate", strings.NewReader("{}")),
	)

	if !slices.Equal(paths, []string{
		"GET /java/agent/api/cards",
		"POST /java/agent/api/card/batch-generate",
	}) {
		t.Fatalf("proxied paths = %#v", paths)
	}
}
