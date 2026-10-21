package chat

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/lingqiao/server/internal/features/sessions"
)

type chatIdentityGolden struct {
	Version  int `json:"version"`
	AuthorID struct {
		Cases []struct {
			CardCode   string `json:"card_code"`
			MachineID  string `json:"machine_id"`
			AuthorID   string `json:"author_id"`
			AuthorName string `json:"author_name"`
		} `json:"cases"`
	} `json:"author_id"`
	Nickname struct {
		MaxRunes int `json:"max_runes"`
		Cases    []struct {
			Input      string `json:"input"`
			Normalized string `json:"normalized"`
			Allowed    bool   `json:"allowed"`
		} `json:"cases"`
	} `json:"nickname"`
	Reactions struct {
		Allowed  []string `json:"allowed"`
		Rejected []string `json:"rejected"`
	} `json:"reactions"`
}

func TestChatIdentityGoldenVectors(t *testing.T) {
	data, err := os.ReadFile("../../../../docs/api/chat-identity-golden-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden chatIdentityGolden
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.Version != 1 {
		t.Fatalf("unexpected golden version %d", golden.Version)
	}

	for _, tc := range golden.AuthorID.Cases {
		session := &sessions.Session{CardCode: tc.CardCode, MachineID: tc.MachineID}
		if got := chatAuthorID(session); got != tc.AuthorID {
			t.Errorf("chatAuthorID(%q,%q) = %s, want %s", tc.CardCode, tc.MachineID, got, tc.AuthorID)
		}
		if got := chatAuthor(session); got != tc.AuthorName {
			t.Errorf("chatAuthor(%q,%q) = %s, want %s", tc.CardCode, tc.MachineID, got, tc.AuthorName)
		}
	}

	for _, tc := range golden.Nickname.Cases {
		if got := normalizeChatNickname(tc.Input); got != tc.Normalized {
			t.Errorf("normalizeChatNickname(%q) = %q, want %q", tc.Input, got, tc.Normalized)
		}
		if got := chatNicknameAllowed(tc.Input); got != tc.Allowed {
			t.Errorf("chatNicknameAllowed(%q) = %v, want %v", tc.Input, got, tc.Allowed)
		}
	}

	for _, reaction := range golden.Reactions.Allowed {
		if !validChatReaction(reaction) {
			t.Errorf("reaction %q should be allowed", reaction)
		}
	}
	for _, reaction := range golden.Reactions.Rejected {
		if validChatReaction(reaction) {
			t.Errorf("reaction %q should be rejected", reaction)
		}
	}
}
