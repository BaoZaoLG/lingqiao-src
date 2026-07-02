package main

import (
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func getFreePort(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}
	defer l.Close()
	return fmt.Sprintf("%d", l.Addr().(*net.TCPAddr).Port)
}

func TestE2EMigrationAndAPI(t *testing.T) {
	// 1. Setup temp dir
	tempDir, err := os.MkdirTemp("", "lingqiao_e2e_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 2. Create dirty data.json
	dataJSONPath := filepath.Join(tempDir, "data.json")
	dirtyData := `{
	  "agents": {
		"agent1": {"username": "admin", "password": "hash1", "created_at": "2023-01-01T00:00:00Z"},
		"agent2": {"username": "admin", "password": "hash2", "created_at": "2023-01-01T00:00:00Z"},
		"agent3": {"username": "admin", "password": "hash3", "created_at": "2023-01-01T00:00:00Z"},
		"agent4": {"username": "admin_dup1", "password": "hash4", "created_at": "2023-01-01T00:00:00Z"}
	  },
	  "audit": [
		{"time": "2023-01-01T00:00:00Z", "action": "test", "detail": "test event"}
	  ]
	}`
	if err := os.WriteFile(dataJSONPath, []byte(dirtyData), 0644); err != nil {
		t.Fatalf("Failed to write data.json: %v", err)
	}

	// 3. Compile server
	serverExe := filepath.Join(tempDir, "server.exe")
	cmd := exec.Command("go", "build", "-o", serverExe, ".")
	cmd.Dir = "../server"
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to compile server: %v\n%s", err, string(out))
	}

	adminPort := getFreePort(t)
	agentPort := getFreePort(t)

	// 4. Run server to trigger migration
	srvCmd := exec.Command(serverExe)
	srvCmd.Env = append(os.Environ(), "DATA_DIR="+tempDir, "PORT="+adminPort, "AGENT_PORT="+agentPort, "ADMIN_PASSWORD=testpass", "DISABLE_TLS=true")
	srvCmd.Stdout = os.Stdout
	srvCmd.Stderr = os.Stderr

	if err := srvCmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	t.Cleanup(func() {
		if err := srvCmd.Process.Kill(); err != nil {
			t.Logf("Server kill error: %v", err)
		}
		srvCmd.Wait()
	})

	// 5. Test API
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 2 * time.Second,
	}

	adminBaseURL := "https://127.0.0.1:" + adminPort

	// Wait for it to boot and migrate by polling login API
	var loginRes struct {
		Token string `json:"token"`
	}
	booted := false
	for i := 0; i < 20; i++ {
		loginReq := strings.NewReader(`{"password":"testpass"}`)
		resp, err := client.Post(adminBaseURL+"/admin/api/login", "application/json", loginReq)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				if err := json.NewDecoder(resp.Body).Decode(&loginRes); err == nil {
					booted = true
					break
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !booted {
		t.Fatalf("Server failed to boot or login failed after timeout")
	}

	// 5.2 Add blacklist (trigger audit)
	req, _ := http.NewRequest("POST", adminBaseURL+"/admin/api/blacklist", strings.NewReader(`{"action":"add","type":"ip","value":"192.168.1.1","reason":"test"}`))
	req.Header.Set("Authorization", "Bearer "+loginRes.Token)
	req.Header.Set("Content-Type", "application/json")
	resp2, err := client.Do(req)
	if err != nil {
		t.Fatalf("Add blacklist request failed: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp2.Body)
		t.Fatalf("Add blacklist failed with status: %d, body: %s", resp2.StatusCode, string(body))
	}

	// 6. Verify SQLite DB exists
	dbPath := filepath.Join(tempDir, "lingqiao.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatalf("lingqiao.db was not created, migration failed or skipped")
	}

	// 7. Verify SQLite DB content
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open sqlite db: %v", err)
	}
	defer db.Close()

	var agentsCount int
	if err := db.QueryRow("SELECT count(*) FROM agents").Scan(&agentsCount); err != nil {
		t.Fatalf("Failed to count agents: %v", err)
	}
	if agentsCount != 4 {
		t.Errorf("Expected 4 agents, got %d", agentsCount)
	}

	var auditCount int
	if err := db.QueryRow("SELECT count(*) FROM audit_log").Scan(&auditCount); err != nil {
		t.Fatalf("Failed to count audit logs: %v", err)
	}
	// Note: 1 from migration + maybe some from API calls during the test
	if auditCount < 1 {
		t.Errorf("Expected at least 1 audit log, got %d", auditCount)
	}

	// Verify deduplicated usernames
	rows, err := db.Query("SELECT username FROM agents ORDER BY username")
	if err != nil {
		t.Fatalf("Failed to query agents: %v", err)
	}
	defer rows.Close()

	var usernames []string
	for rows.Next() {
		var un string
		if err := rows.Scan(&un); err != nil {
			t.Fatalf("Failed to scan username: %v", err)
		}
		usernames = append(usernames, un)
	}

	// We expect "admin", "admin-agen" (agent2), "admin-agen_dup1" (agent3) and "admin_dup1"
	// But it depends on map iteration order. We just check they are all unique.
	unique := make(map[string]bool)
	for _, un := range usernames {
		if unique[un] {
			t.Errorf("Found duplicate username in database: %s", un)
		}
		unique[un] = true
	}
	if !unique["admin"] || !unique["admin_dup1"] {
		t.Errorf("Expected 'admin' and 'admin_dup1' to exist, got %v", usernames)
	}
}
