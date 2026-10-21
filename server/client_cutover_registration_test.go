package main

import (
	"os"
	"strings"
	"testing"
)

// Java client controllers are not cutover-capable merely because the proxy map
// knows their paths: the public Go listener must also wrap each registration.
func TestJavaClientRoutesAreActuallyWrappedByCutover(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	required := []string{
		`activateHandler := cutoverRouter.Wrap("sessions", api.HandleActivate)`,
		`mux.HandleFunc("/api/v1/heartbeat", cutoverRouter.Wrap("sessions", api.HandleHeartbeat))`,
		`mux.HandleFunc("/api/v1/deactivate", cutoverRouter.Wrap("sessions", api.HandleDeactivate))`,
		`mux.HandleFunc("/api/v1/announcement", cutoverRouter.Wrap("announcement", api.HandleAnnouncement))`,
		`mux.HandleFunc("/api/v1/chat/profile", cutoverRouter.Wrap("chat", chatHandler.HandleChatProfile))`,
		`mux.HandleFunc("/api/v1/chat/messages", cutoverRouter.Wrap("chat", chatHandler.HandleChatMessages))`,
		`mux.HandleFunc("/api/v1/chat/presence", cutoverRouter.Wrap("chat", chatHandler.HandleChatPresence))`,
		`mux.HandleFunc("/api/v1/chat/react", cutoverRouter.Wrap("chat", chatHandler.HandleChatReact))`,
		`mux.HandleFunc("/api/v1/chat/send", cutoverRouter.Wrap("chat", chatHandler.HandleChatSend))`,
		`mux.HandleFunc("/api/v1/dll", cutoverRouter.Wrap("payloads", api.HandleDllDownload))`,
		`mux.HandleFunc("/api/v1/script", cutoverRouter.Wrap("scripts", scriptsHandler.HandleScriptDownload))`,
		`mux.HandleFunc("/api/v1/key-exchange", cutoverRouter.Wrap("payloads"`,
		`mux.HandleFunc("/api/v1/payload/", cutoverRouter.Wrap("payloads"`,
	}
	for _, snippet := range required {
		if !strings.Contains(text, snippet) {
			t.Errorf("main.go is missing client cutover registration %q", snippet)
		}
	}
}
