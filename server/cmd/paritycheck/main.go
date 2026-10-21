// Command paritycheck compares Go and Java responses for the same admin route.
//
// It reads the shared cutover manifest and, for every read-only route, issues the
// same request to both backends and diffs the decoded JSON. Write routes are never
// called: this tool must be safe to run against production.
//
// Usage:
//
//	paritycheck -go http://127.0.0.1:48901 -java http://127.0.0.1:49001 -token <admin_token>
//
// Exit code 0 means every compared route matched.
package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type manifest struct {
	Version int `json:"version"`
	Routes  []struct {
		Module     string   `json:"module"`
		GoPath     string   `json:"go_path"`
		JavaPath   string   `json:"java_path"`
		Methods    []string `json:"methods"`
		Write      bool     `json:"write"`
		Permission string   `json:"permission"`
		Response   string   `json:"response"`
		ClientAuth bool     `json:"client_auth"`
		ParitySkip bool     `json:"parity_skip"`
	} `json:"routes"`
}

type options struct {
	goBase       string
	javaBase     string
	token        string
	manifestPath string
	module       string
	query        string
	timeout      time.Duration
	ignoreFields string
	insecure     bool
}

func main() {
	var opts options
	flag.StringVar(&opts.goBase, "go", "http://127.0.0.1:48901", "Go backend base URL")
	flag.StringVar(&opts.javaBase, "java", "http://127.0.0.1:49001", "Java sidecar base URL")
	flag.StringVar(&opts.token, "token", os.Getenv("ADMIN_TOKEN"), "admin token (defaults to $ADMIN_TOKEN)")
	flag.StringVar(&opts.manifestPath, "manifest", "docs/api/java-cutover-contracts.json", "shared contract manifest")
	flag.StringVar(&opts.module, "module", "", "only compare this module (empty = all)")
	flag.StringVar(&opts.query, "query", "", "query string appended to every request, e.g. \"page=1&per_page=20\"")
	flag.StringVar(&opts.ignoreFields, "ignore", "", "comma-separated top-level fields to ignore, e.g. \"public_key\"")
	flag.DurationVar(&opts.timeout, "timeout", 15*time.Second, "per-request timeout")
	flag.BoolVar(&opts.insecure, "insecure", false,
		"skip TLS verification (the Go server uses a self-signed certificate by default)")
	flag.Parse()

	if opts.token == "" {
		fmt.Fprintln(os.Stderr, "error: admin token is required (-token or $ADMIN_TOKEN)")
		os.Exit(2)
	}

	m, err := loadManifest(opts.manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}

	client := newClient(opts)
	ignored := parseIgnored(opts.ignoreFields)

	compared, mismatched, skipped := 0, 0, 0
	for _, route := range m.Routes {
		if route.Write || len(route.Methods) == 0 || route.Methods[0] != http.MethodGet {
			continue
		}
		if opts.module != "" && route.Module != opts.module {
			continue
		}
		// The agent portal listens on a different Go port; comparing it needs a
		// separate run with -go pointed at the agent address.
		if route.Module == "agent-portal" {
			skipped++
			continue
		}
		// Authenticated binary downloads need per-client HMAC/session headers and
		// are verified by their dedicated controller/cutover tests instead.
		if route.Response == "binary" || route.ClientAuth || route.ParitySkip {
			skipped++
			continue
		}

		compared++
		goBody, goStatus, goErr := fetch(client, opts.goBase+route.GoPath, opts.query, opts.token)
		javaBody, javaStatus, javaErr := fetch(client, opts.javaBase+route.JavaPath, opts.query, opts.token)

		switch {
		case goErr != nil:
			mismatched++
			fmt.Printf("ERROR %-34s go request failed: %v\n", route.GoPath, goErr)
		case javaErr != nil:
			mismatched++
			fmt.Printf("ERROR %-34s java request failed: %v\n", route.GoPath, javaErr)
		case goStatus != javaStatus:
			mismatched++
			fmt.Printf("DIFF  %-34s status go=%d java=%d\n", route.GoPath, goStatus, javaStatus)
		default:
			diffs := comparePayload(route.Response, goBody, javaBody, ignored)
			if len(diffs) == 0 {
				fmt.Printf("OK    %-34s %s\n", route.GoPath, route.Module)
				continue
			}
			mismatched++
			fmt.Printf("DIFF  %-34s %d difference(s)\n", route.GoPath, len(diffs))
			for _, d := range diffs {
				fmt.Printf("        %s\n", d)
			}
		}
	}

	fmt.Printf("\ncompared=%d matched=%d mismatched=%d skipped=%d\n",
		compared, compared-mismatched, mismatched, skipped)
	if skipped > 0 {
		fmt.Println("note: agent-portal routes need a separate run with -go pointed at the agent port")
	}
	if mismatched > 0 {
		os.Exit(1)
	}
}

func comparePayload(kind string, goBody, javaBody []byte, ignored map[string]bool) []string {
	if kind != "csv" {
		return compareJSON(goBody, javaBody, ignored)
	}
	if bytes.Equal(goBody, javaBody) {
		return nil
	}
	return []string{fmt.Sprintf(
		"$: CSV bytes differ (go=%d bytes java=%d bytes)", len(goBody), len(javaBody))}
}

// newClient builds the HTTP client. The Go server serves TLS with a self-signed
// certificate in most deployments, so -insecure is the normal local setting; it is
// never the default so a production run cannot silently ignore a bad certificate.
func newClient(opts options) *http.Client {
	client := &http.Client{Timeout: opts.timeout}
	if opts.insecure {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402 -- opt-in flag
		}
	}
	return client
}

func loadManifest(path string) (manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	if len(m.Routes) == 0 {
		return manifest{}, fmt.Errorf("manifest %s declares no routes", path)
	}
	return m, nil
}

func parseIgnored(raw string) map[string]bool {
	ignored := map[string]bool{}
	for _, field := range strings.Split(raw, ",") {
		if field = strings.TrimSpace(field); field != "" {
			ignored[field] = true
		}
	}
	return ignored
}

func fetch(client *http.Client, url, query, token string) ([]byte, int, error) {
	if query != "" {
		if strings.Contains(url, "?") {
			url += "&" + query
		} else {
			url += "?" + query
		}
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	// Both backends accept the token as a cookie or bearer header.
	req.Header.Set("Authorization", "Bearer "+token)
	req.AddCookie(&http.Cookie{Name: "admin_token", Value: token})

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

// compareJSON decodes both payloads and reports every structural difference as a
// JSON-path style string. Returns nil when the two payloads are equivalent.
func compareJSON(goBody, javaBody []byte, ignored map[string]bool) []string {
	var goValue, javaValue any
	if err := json.Unmarshal(goBody, &goValue); err != nil {
		return []string{fmt.Sprintf("go response is not JSON: %v", err)}
	}
	if err := json.Unmarshal(javaBody, &javaValue); err != nil {
		return []string{fmt.Sprintf("java response is not JSON: %v", err)}
	}
	if goMap, ok := goValue.(map[string]any); ok {
		if javaMap, ok := javaValue.(map[string]any); ok {
			for field := range ignored {
				delete(goMap, field)
				delete(javaMap, field)
			}
		}
	}
	var diffs []string
	walk("$", goValue, javaValue, &diffs)
	sort.Strings(diffs)
	return diffs
}

func walk(path string, goValue, javaValue any, diffs *[]string) {
	// Cap the report so one structural mismatch cannot flood the output.
	if len(*diffs) >= 50 {
		return
	}

	switch expected := goValue.(type) {
	case map[string]any:
		actual, ok := javaValue.(map[string]any)
		if !ok {
			*diffs = append(*diffs, fmt.Sprintf("%s: go=object java=%s", path, kindOf(javaValue)))
			return
		}
		for _, key := range sortedKeys(expected) {
			child, present := actual[key]
			if !present {
				*diffs = append(*diffs, fmt.Sprintf("%s.%s: missing in java", path, key))
				continue
			}
			walk(path+"."+key, expected[key], child, diffs)
		}
		for _, key := range sortedKeys(actual) {
			if _, present := expected[key]; !present {
				*diffs = append(*diffs, fmt.Sprintf("%s.%s: extra in java", path, key))
			}
		}

	case []any:
		actual, ok := javaValue.([]any)
		if !ok {
			*diffs = append(*diffs, fmt.Sprintf("%s: go=array java=%s", path, kindOf(javaValue)))
			return
		}
		if len(expected) != len(actual) {
			*diffs = append(*diffs, fmt.Sprintf("%s: length go=%d java=%d", path, len(expected), len(actual)))
			return
		}
		for i := range expected {
			walk(fmt.Sprintf("%s[%d]", path, i), expected[i], actual[i], diffs)
		}

	default:
		if !equalScalar(goValue, javaValue) {
			*diffs = append(*diffs, fmt.Sprintf("%s: go=%v java=%v", path, format(goValue), format(javaValue)))
		}
	}
}

// equalScalar treats Go's nil and an absent-but-empty Java value as different,
// but tolerates the numeric round-tripping both encoders perform.
func equalScalar(goValue, javaValue any) bool {
	if goValue == nil || javaValue == nil {
		return goValue == nil && javaValue == nil
	}
	goNum, goIsNum := goValue.(float64)
	javaNum, javaIsNum := javaValue.(float64)
	if goIsNum && javaIsNum {
		return goNum == javaNum
	}
	goText, goIsText := goValue.(string)
	javaText, javaIsText := javaValue.(string)
	if goIsText && javaIsText {
		goTime, goTimeErr := time.Parse(time.RFC3339Nano, goText)
		javaTime, javaTimeErr := time.Parse(time.RFC3339Nano, javaText)
		if goTimeErr == nil && javaTimeErr == nil {
			return goTime.Equal(javaTime)
		}
	}
	return goValue == javaValue
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func kindOf(value any) string {
	switch value.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case nil:
		return "null"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func format(value any) string {
	if str, ok := value.(string); ok {
		return fmt.Sprintf("%q", str)
	}
	return fmt.Sprintf("%v", value)
}
