package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareJSONReportsNoDiffForIdenticalPayloads(t *testing.T) {
	body := []byte(`{"status":"ok","cards":[{"code":"A","max_sessions":1}]}`)
	if diffs := compareJSON(body, body, nil); len(diffs) != 0 {
		t.Fatalf("expected no diffs, got %v", diffs)
	}
}

func TestCompareJSONDetectsMissingAndExtraFields(t *testing.T) {
	goBody := []byte(`{"status":"ok","total":2}`)
	javaBody := []byte(`{"status":"ok","read_only":true}`)

	diffs := compareJSON(goBody, javaBody, nil)
	joined := strings.Join(diffs, "\n")
	if !strings.Contains(joined, "$.total: missing in java") {
		t.Errorf("missing-field diff not reported: %v", diffs)
	}
	if !strings.Contains(joined, "$.read_only: extra in java") {
		t.Errorf("extra-field diff not reported: %v", diffs)
	}
}

func TestCompareJSONDetectsScalarMismatch(t *testing.T) {
	diffs := compareJSON([]byte(`{"total":2}`), []byte(`{"total":3}`), nil)
	if len(diffs) != 1 || !strings.Contains(diffs[0], "go=2 java=3") {
		t.Fatalf("scalar diff = %v", diffs)
	}
}

func TestCompareJSONTreatsEquivalentRfc3339TimestampsAsEqual(t *testing.T) {
	goBody := []byte(`{"at":"2026-07-28T18:05:43.1234+08:00"}`)
	javaBody := []byte(`{"at":"2026-07-28T10:05:43.123400000Z"}`)

	if diffs := compareJSON(goBody, javaBody, nil); len(diffs) != 0 {
		t.Fatalf("equivalent timestamps differ: %v", diffs)
	}
}

func TestCompareJSONDetectsArrayLengthMismatch(t *testing.T) {
	diffs := compareJSON([]byte(`{"a":[1,2]}`), []byte(`{"a":[1]}`), nil)
	if len(diffs) != 1 || !strings.Contains(diffs[0], "length go=2 java=1") {
		t.Fatalf("array diff = %v", diffs)
	}
}

func TestCompareJSONDistinguishesNullFromEmptyString(t *testing.T) {
	diffs := compareJSON([]byte(`{"note":null}`), []byte(`{"note":""}`), nil)
	if len(diffs) != 1 {
		t.Fatalf("expected null-vs-empty diff, got %v", diffs)
	}
}

func TestCompareJSONHonoursIgnoredFields(t *testing.T) {
	goBody := []byte(`{"status":"ok","public_key":"aaa"}`)
	javaBody := []byte(`{"status":"ok","public_key":""}`)

	if diffs := compareJSON(goBody, javaBody, map[string]bool{"public_key": true}); len(diffs) != 0 {
		t.Fatalf("ignored field still reported: %v", diffs)
	}
}

func TestCompareJSONReportsNonJSONBody(t *testing.T) {
	diffs := compareJSON([]byte(`<html>500</html>`), []byte(`{}`), nil)
	if len(diffs) != 1 || !strings.Contains(diffs[0], "go response is not JSON") {
		t.Fatalf("non-JSON diff = %v", diffs)
	}
}

func TestCompareJSONCapsDiffCount(t *testing.T) {
	goItems := make([]map[string]int, 200)
	javaItems := make([]map[string]int, 200)
	for i := range goItems {
		goItems[i] = map[string]int{"v": i}
		javaItems[i] = map[string]int{"v": i + 1}
	}
	goBody, _ := json.Marshal(map[string]any{"items": goItems})
	javaBody, _ := json.Marshal(map[string]any{"items": javaItems})

	if diffs := compareJSON(goBody, javaBody, nil); len(diffs) > 50 {
		t.Fatalf("diff count not capped: %d", len(diffs))
	}
}

func TestComparePayloadComparesCsvAsExactBytes(t *testing.T) {
	body := []byte("\xEF\xBB\xBFcode,status\r\nA,active\r\n")
	if diffs := comparePayload("csv", body, body, nil); len(diffs) != 0 {
		t.Fatalf("identical CSV differs: %v", diffs)
	}
	diffs := comparePayload("csv", body, append(body, '\n'), nil)
	if len(diffs) != 1 || !strings.Contains(diffs[0], "CSV bytes differ") {
		t.Fatalf("CSV mismatch = %v", diffs)
	}
}

func TestFetchSendsTokenAsBearerAndCookie(t *testing.T) {
	var gotAuth, gotCookie string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if c, err := r.Cookie("admin_token"); err == nil {
			gotCookie = c.Value
		}
		if r.URL.RawQuery != "page=1" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	body, status, err := fetch(server.Client(), server.URL+"/admin/api/cards", "page=1", "tok")
	if err != nil || status != http.StatusOK || string(body) != `{"status":"ok"}` {
		t.Fatalf("fetch: body=%s status=%d err=%v", body, status, err)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotCookie != "tok" {
		t.Errorf("cookie = %q", gotCookie)
	}
}

func TestLoadManifestRejectsEmptyRouteList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(path, []byte(`{"version":3,"routes":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadManifest(path); err == nil {
		t.Fatal("expected an error for an empty route list")
	}
}

func TestLoadManifestReadsSharedContract(t *testing.T) {
	m, err := loadManifest(filepath.Join("..", "..", "..", "docs", "api", "java-cutover-contracts.json"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != 14 {
		t.Errorf("version = %d, want 14", m.Version)
	}
	readRoutes := 0
	for _, route := range m.Routes {
		if !route.Write {
			readRoutes++
		}
	}
	if readRoutes == 0 {
		t.Error("shared contract declares no read routes to compare")
	}
}

func TestParseIgnoredSplitsAndTrims(t *testing.T) {
	ignored := parseIgnored(" public_key , total ,")
	if !ignored["public_key"] || !ignored["total"] || len(ignored) != 2 {
		t.Fatalf("parseIgnored = %v", ignored)
	}
}
