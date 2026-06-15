package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestChatSendAndListMessagesForActivatedSession(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "chat", 1, "")
	session, err := cm.ActivateCard(card.Code, "machine-chat", "fp", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}

	api := NewAPIHandler(cm)
	api.chat = NewChatStore(NewJSONStorage(dir), time.Now)

	sendReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/send", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": session.Token,
		"machine_id":    "machine-chat",
		"card":          card.Code,
		"content":       "hello public room",
	})
	sendRR := httptest.NewRecorder()
	api.HandleChatSend(sendRR, sendReq)
	if sendRR.Code != http.StatusOK {
		t.Fatalf("send status = %d, body=%s", sendRR.Code, sendRR.Body.String())
	}

	listReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/messages", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": session.Token,
		"machine_id":    "machine-chat",
		"card":          card.Code,
		"after_id":      int64(0),
	})
	listRR := httptest.NewRecorder()
	api.HandleChatMessages(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listRR.Code, listRR.Body.String())
	}

	var resp struct {
		Status   string        `json:"status"`
		Messages []ChatMessage `json:"messages"`
	}
	if err := json.Unmarshal(listRR.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid list response: %v", err)
	}
	if len(resp.Messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(resp.Messages))
	}
	if resp.Messages[0].Content != "hello public room" {
		t.Fatalf("Content = %q", resp.Messages[0].Content)
	}
	if resp.Messages[0].Author == "" || resp.Messages[0].MachineID != "" || resp.Messages[0].CardCode != "" {
		t.Fatalf("public message should expose pseudonym only, got author=%q machine=%q card=%q",
			resp.Messages[0].Author, resp.Messages[0].MachineID, resp.Messages[0].CardCode)
	}
}

func TestChatStoreAddsSystemMessage(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	api := NewAPIHandler(cm)
	api.chat = NewChatStore(NewJSONStorage(dir), time.Now)

	msg, err := api.chat.AddSystem("维护提醒")
	if err != nil {
		t.Fatalf("AddSystem returned error: %v", err)
	}
	if msg.Type != "system" || msg.Author != "系统" || msg.Content != "维护提醒" {
		t.Fatalf("system message = %#v", msg)
	}

	messages := api.chat.List(0)
	if len(messages) != 1 || messages[0].Type != "system" {
		t.Fatalf("messages = %#v, want one system message", messages)
	}
}

func TestChatStoreDeleteHidesMessageFromList(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "chat", 1, "")
	session, err := cm.ActivateCard(card.Code, "machine-chat", "fp", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}

	api := NewAPIHandler(cm)
	api.chat = NewChatStore(NewJSONStorage(dir), time.Now)

	msg, err := api.chat.Add(session, "hide me")
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if !api.chat.Delete(msg.ID) {
		t.Fatalf("Delete(%d) returned false", msg.ID)
	}
	if messages := api.chat.List(0); len(messages) != 0 {
		t.Fatalf("messages after delete = %#v, want none", messages)
	}
}

func TestAdminChatMessagesIncludesDeletedMessages(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "chat", 1, "")
	session, err := cm.ActivateCard(card.Code, "machine-admin-chat", "fp", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}

	api := NewAPIHandler(cm)
	api.chat = NewChatStore(NewJSONStorage(dir), time.Now)
	admin := NewAdminHandler(cm)
	admin.chat = api.chat

	msg, err := api.chat.Add(session, "admin visible")
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}
	if !api.chat.Delete(msg.ID) {
		t.Fatalf("Delete(%d) returned false", msg.ID)
	}
	if messages := api.chat.List(0); len(messages) != 0 {
		t.Fatalf("public messages after delete = %#v, want none", messages)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/api/chat/messages", nil)
	rr := httptest.NewRecorder()
	admin.HandleChatMessages(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("admin list status = %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		Messages []struct {
			ID      int64  `json:"id"`
			Content string `json:"content"`
			Deleted bool   `json:"deleted"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid admin list response: %v", err)
	}
	if len(resp.Messages) != 1 || resp.Messages[0].ID != msg.ID || !resp.Messages[0].Deleted {
		t.Fatalf("admin messages = %#v, want deleted message visible", resp.Messages)
	}
}

func TestAdminChatDeleteSoftDeletesMessage(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "chat", 1, "")
	session, err := cm.ActivateCard(card.Code, "machine-admin-delete", "fp", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}

	api := NewAPIHandler(cm)
	api.chat = NewChatStore(NewJSONStorage(dir), time.Now)
	admin := NewAdminHandler(cm)
	admin.chat = api.chat

	msg, err := api.chat.Add(session, "delete through admin api")
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	body, _ := json.Marshal(map[string]interface{}{"id": msg.ID})
	req := httptest.NewRequest(http.MethodPost, "/admin/api/chat/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	admin.HandleChatDelete(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body=%s", rr.Code, rr.Body.String())
	}

	if messages := api.chat.List(0); len(messages) != 0 {
		t.Fatalf("public messages after admin delete = %#v, want none", messages)
	}
	adminMessages := api.chat.AdminList()
	if len(adminMessages) != 1 || !adminMessages[0].Deleted {
		t.Fatalf("admin messages after delete = %#v, want deleted message visible", adminMessages)
	}
}

func TestAdminChatSystemCreatesSystemMessage(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "chat", 1, "")
	session, err := cm.ActivateCard(card.Code, "machine-admin-system", "fp", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}

	api := NewAPIHandler(cm)
	api.chat = NewChatStore(NewJSONStorage(dir), time.Now)
	admin := NewAdminHandler(cm)
	admin.chat = api.chat

	body, _ := json.Marshal(map[string]interface{}{"content": "维护通知"})
	req := httptest.NewRequest(http.MethodPost, "/admin/api/chat/system", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	admin.HandleChatSystem(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("system status = %d, body=%s", rr.Code, rr.Body.String())
	}

	messages := api.chat.List(0, session)
	if len(messages) != 1 {
		t.Fatalf("len(messages) = %d, want 1", len(messages))
	}
	if messages[0].Type != "system" || messages[0].Author != "系统" || messages[0].Content != "维护通知" {
		t.Fatalf("system message = %#v", messages[0])
	}
}

func TestAdminChatMuteBlocksSendingMessages(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "chat", 1, "")
	session, err := cm.ActivateCard(card.Code, "machine-admin-mute", "fp", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}

	api := NewAPIHandler(cm)
	api.chat = NewChatStore(NewJSONStorage(dir), time.Now)
	admin := NewAdminHandler(cm)
	admin.chat = api.chat

	body, _ := json.Marshal(map[string]interface{}{
		"author_id":        chatAuthorID(session),
		"duration_minutes": 30,
		"reason":           "spam",
	})
	muteReq := httptest.NewRequest(http.MethodPost, "/admin/api/chat/mute", bytes.NewReader(body))
	muteReq.Header.Set("Content-Type", "application/json")
	muteRR := httptest.NewRecorder()
	admin.HandleChatMute(muteRR, muteReq)
	if muteRR.Code != http.StatusOK {
		t.Fatalf("mute status = %d, body=%s", muteRR.Code, muteRR.Body.String())
	}

	sendReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/send", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": session.Token,
		"machine_id":    "machine-admin-mute",
		"card":          card.Code,
		"content":       "blocked",
	})
	sendRR := httptest.NewRecorder()
	api.HandleChatSend(sendRR, sendReq)
	if sendRR.Code != http.StatusForbidden {
		t.Fatalf("muted send status = %d, want %d, body=%s", sendRR.Code, http.StatusForbidden, sendRR.Body.String())
	}
}

func TestAdminChatMuteBlocksProfileUpdates(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "chat", 1, "")
	session, err := cm.ActivateCard(card.Code, "machine-admin-mute-profile", "fp", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}

	api := NewAPIHandler(cm)
	api.chat = NewChatStore(NewJSONStorage(dir), time.Now)
	admin := NewAdminHandler(cm)
	admin.chat = api.chat

	body, _ := json.Marshal(map[string]interface{}{
		"author_id":        chatAuthorID(session),
		"duration_minutes": 30,
		"reason":           "spam",
	})
	muteReq := httptest.NewRequest(http.MethodPost, "/admin/api/chat/mute", bytes.NewReader(body))
	muteReq.Header.Set("Content-Type", "application/json")
	muteRR := httptest.NewRecorder()
	admin.HandleChatMute(muteRR, muteReq)
	if muteRR.Code != http.StatusOK {
		t.Fatalf("mute status = %d, body=%s", muteRR.Code, muteRR.Body.String())
	}

	profileReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/profile", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": session.Token,
		"machine_id":    "machine-admin-mute-profile",
		"card":          card.Code,
		"nickname":      "新昵称",
	})
	profileRR := httptest.NewRecorder()
	api.HandleChatProfile(profileRR, profileReq)
	if profileRR.Code != http.StatusForbidden {
		t.Fatalf("muted profile status = %d, want %d, body=%s", profileRR.Code, http.StatusForbidden, profileRR.Body.String())
	}
}

func TestChatRejectsInvalidSession(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	api := NewAPIHandler(cm)
	api.chat = NewChatStore(NewJSONStorage(dir), time.Now)

	req := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/send", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": "missing",
		"machine_id":    "machine-chat",
		"card":          "ABCDEF-GHJKMN-PQRSTV",
		"content":       "hello",
	})
	rr := httptest.NewRecorder()
	api.HandleChatSend(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusUnauthorized, rr.Body.String())
	}
}

func TestChatSendRateLimit(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "chat", 1, "")
	session, err := cm.ActivateCard(card.Code, "machine-chat", "fp", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}

	api := NewAPIHandler(cm)
	api.chat = NewChatStore(NewJSONStorage(dir), time.Now)

	first := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/send", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": session.Token,
		"machine_id":    "machine-chat",
		"card":          card.Code,
		"content":       "first",
	})
	firstRR := httptest.NewRecorder()
	api.HandleChatSend(firstRR, first)
	if firstRR.Code != http.StatusOK {
		t.Fatalf("first status = %d, body=%s", firstRR.Code, firstRR.Body.String())
	}

	time.Sleep(time.Millisecond)
	second := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/send", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": session.Token,
		"machine_id":    "machine-chat",
		"card":          card.Code,
		"content":       "second",
	})
	secondRR := httptest.NewRecorder()
	api.HandleChatSend(secondRR, second)
	if secondRR.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want %d, body=%s", secondRR.Code, http.StatusTooManyRequests, secondRR.Body.String())
	}
}

func TestChatProfileUpdatesNicknameForFutureMessages(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "chat", 1, "")
	session, err := cm.ActivateCard(card.Code, "machine-profile", "fp", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}

	api := NewAPIHandler(cm)
	api.chat = NewChatStore(NewJSONStorage(dir), time.Now)

	profileReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/profile", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": session.Token,
		"machine_id":    "machine-profile",
		"card":          card.Code,
		"nickname":      "桥友",
	})
	profileRR := httptest.NewRecorder()
	api.HandleChatProfile(profileRR, profileReq)
	if profileRR.Code != http.StatusOK {
		t.Fatalf("profile status = %d, body=%s", profileRR.Code, profileRR.Body.String())
	}
	var profileResp struct {
		Status  string      `json:"status"`
		Profile ChatProfile `json:"profile"`
	}
	if err := json.Unmarshal(profileRR.Body.Bytes(), &profileResp); err != nil {
		t.Fatalf("invalid profile response: %v", err)
	}
	if profileResp.Profile.Nickname != "桥友" || profileResp.Profile.AuthorID == "" {
		t.Fatalf("profile = %#v, want nickname and author id", profileResp.Profile)
	}

	listReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/messages", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": session.Token,
		"machine_id":    "machine-profile",
		"card":          card.Code,
		"content":       "hello",
	})
	sendRR := httptest.NewRecorder()
	api.HandleChatSend(sendRR, listReq)
	if sendRR.Code != http.StatusOK {
		t.Fatalf("send status = %d, body=%s", sendRR.Code, sendRR.Body.String())
	}

	messageReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/messages", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": session.Token,
		"machine_id":    "machine-profile",
		"card":          card.Code,
		"after_id":      int64(0),
	})
	listRR := httptest.NewRecorder()
	api.HandleChatMessages(listRR, messageReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listRR.Code, listRR.Body.String())
	}

	var resp struct {
		Status   string        `json:"status"`
		Messages []ChatMessage `json:"messages"`
	}
	if err := json.Unmarshal(listRR.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid list response: %v", err)
	}
	if len(resp.Messages) != 1 || resp.Messages[0].Author != "桥友" {
		t.Fatalf("messages = %#v, want one message from updated nickname", resp.Messages)
	}
}

func TestChatProfileRejectsSensitiveNickname(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "chat", 1, "")
	session, err := cm.ActivateCard(card.Code, "machine-profile-sensitive", "fp", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}

	api := NewAPIHandler(cm)
	api.chat = NewChatStore(NewJSONStorage(dir), time.Now)

	profileReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/profile", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": session.Token,
		"machine_id":    "machine-profile-sensitive",
		"card":          card.Code,
		"nickname":      "管理员",
	})
	profileRR := httptest.NewRecorder()
	api.HandleChatProfile(profileRR, profileReq)
	if profileRR.Code != http.StatusBadRequest {
		t.Fatalf("profile status = %d, want %d, body=%s", profileRR.Code, http.StatusBadRequest, profileRR.Body.String())
	}
}

func TestChatSendWithReplyReturnsReplyPreview(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "chat", 1, "")
	session, err := cm.ActivateCard(card.Code, "machine-reply", "fp", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}

	api := NewAPIHandler(cm)
	api.chat = NewChatStore(NewJSONStorage(dir), time.Now)

	firstReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/send", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": session.Token,
		"machine_id":    "machine-reply",
		"card":          card.Code,
		"content":       "first message",
	})
	firstRR := httptest.NewRecorder()
	api.HandleChatSend(firstRR, firstReq)
	if firstRR.Code != http.StatusOK {
		t.Fatalf("first send status = %d, body=%s", firstRR.Code, firstRR.Body.String())
	}
	var firstResp struct {
		Message ChatMessage `json:"message"`
	}
	if err := json.Unmarshal(firstRR.Body.Bytes(), &firstResp); err != nil {
		t.Fatalf("invalid first send response: %v", err)
	}

	api.chat.lastSent = map[string]time.Time{}
	replyReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/send", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": session.Token,
		"machine_id":    "machine-reply",
		"card":          card.Code,
		"content":       "reply message",
		"reply_to_id":   firstResp.Message.ID,
	})
	replyRR := httptest.NewRecorder()
	api.HandleChatSend(replyRR, replyReq)
	if replyRR.Code != http.StatusOK {
		t.Fatalf("reply send status = %d, body=%s", replyRR.Code, replyRR.Body.String())
	}

	listReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/messages", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": session.Token,
		"machine_id":    "machine-reply",
		"card":          card.Code,
		"after_id":      int64(0),
	})
	listRR := httptest.NewRecorder()
	api.HandleChatMessages(listRR, listReq)
	if listRR.Code != http.StatusOK {
		t.Fatalf("list status = %d, body=%s", listRR.Code, listRR.Body.String())
	}

	var resp struct {
		Messages []ChatMessage `json:"messages"`
	}
	if err := json.Unmarshal(listRR.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid list response: %v", err)
	}
	if len(resp.Messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(resp.Messages))
	}
	reply := resp.Messages[1]
	if reply.ReplyToID != firstResp.Message.ID || reply.ReplyPreview == nil {
		t.Fatalf("reply fields = %#v, want reply_to_id and preview", reply)
	}
	if reply.ReplyPreview.Author != resp.Messages[0].Author || reply.ReplyPreview.Content != "first message" {
		t.Fatalf("reply preview = %#v, want original author and content", reply.ReplyPreview)
	}
}

func TestChatReactTogglesReactionForCurrentUser(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "chat", 1, "")
	session, err := cm.ActivateCard(card.Code, "machine-react", "fp", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}

	api := NewAPIHandler(cm)
	api.chat = NewChatStore(NewJSONStorage(dir), time.Now)

	msg, err := api.chat.Add(session, "reactable")
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	reactReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/react", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": session.Token,
		"machine_id":    "machine-react",
		"card":          card.Code,
		"message_id":    msg.ID,
		"reaction":      "👍",
	})
	reactRR := httptest.NewRecorder()
	api.HandleChatReact(reactRR, reactReq)
	if reactRR.Code != http.StatusOK {
		t.Fatalf("react status = %d, body=%s", reactRR.Code, reactRR.Body.String())
	}

	messages := api.chat.List(0, session)
	if len(messages) != 1 || messages[0].Reactions["👍"] != 1 || len(messages[0].Reacted) != 1 {
		t.Fatalf("messages after react = %#v, want reaction count and reacted flag", messages)
	}

	cancelReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/react", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": session.Token,
		"machine_id":    "machine-react",
		"card":          card.Code,
		"message_id":    msg.ID,
		"reaction":      "👍",
	})
	cancelRR := httptest.NewRecorder()
	api.HandleChatReact(cancelRR, cancelReq)
	if cancelRR.Code != http.StatusOK {
		t.Fatalf("cancel status = %d, body=%s", cancelRR.Code, cancelRR.Body.String())
	}

	messages = api.chat.List(0, session)
	if len(messages) != 1 || messages[0].Reactions["👍"] != 0 || len(messages[0].Reacted) != 0 {
		t.Fatalf("messages after cancel = %#v, want reaction removed", messages)
	}
}

func TestAdminChatMuteBlocksReactions(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "chat", 1, "")
	session, err := cm.ActivateCard(card.Code, "machine-admin-mute-react", "fp", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}

	api := NewAPIHandler(cm)
	api.chat = NewChatStore(NewJSONStorage(dir), time.Now)
	admin := NewAdminHandler(cm)
	admin.chat = api.chat

	msg, err := api.chat.Add(session, "react blocked")
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"author_id":        chatAuthorID(session),
		"duration_minutes": 30,
		"reason":           "spam",
	})
	muteReq := httptest.NewRequest(http.MethodPost, "/admin/api/chat/mute", bytes.NewReader(body))
	muteReq.Header.Set("Content-Type", "application/json")
	muteRR := httptest.NewRecorder()
	admin.HandleChatMute(muteRR, muteReq)
	if muteRR.Code != http.StatusOK {
		t.Fatalf("mute status = %d, body=%s", muteRR.Code, muteRR.Body.String())
	}

	reactReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/react", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": session.Token,
		"machine_id":    "machine-admin-mute-react",
		"card":          card.Code,
		"message_id":    msg.ID,
		"reaction":      "👍",
	})
	reactRR := httptest.NewRecorder()
	api.HandleChatReact(reactRR, reactReq)
	if reactRR.Code != http.StatusForbidden {
		t.Fatalf("muted react status = %d, want %d, body=%s", reactRR.Code, http.StatusForbidden, reactRR.Body.String())
	}
}

func TestChatMutePersistsAcrossStoreReload(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	card, _ := cm.GenerateCard(24*time.Hour, "chat", 1, "")
	session, err := cm.ActivateCard(card.Code, "machine-mute-persist", "fp", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("ActivateCard returned error: %v", err)
	}

	storage := NewJSONStorage(dir)
	chat := NewChatStore(storage, time.Now)
	if _, err := chat.Mute(chatAuthorID(session), 30*time.Minute, "spam"); err != nil {
		t.Fatalf("Mute returned error: %v", err)
	}

	reloaded := NewChatStore(storage, time.Now)
	api := NewAPIHandler(cm)
	api.chat = reloaded

	sendReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/send", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": session.Token,
		"machine_id":    "machine-mute-persist",
		"card":          card.Code,
		"content":       "still blocked",
	})
	sendRR := httptest.NewRecorder()
	api.HandleChatSend(sendRR, sendReq)
	if sendRR.Code != http.StatusForbidden {
		t.Fatalf("send after reload status = %d, want %d, body=%s", sendRR.Code, http.StatusForbidden, sendRR.Body.String())
	}
}

func TestChatPresenceCountsRecentlyActiveUsers(t *testing.T) {
	cm, dir := setupTestCM(t)
	defer teardownTestCM(dir)

	now := time.Date(2026, 6, 15, 10, 30, 0, 0, time.UTC)
	cardA, _ := cm.GenerateCard(24*time.Hour, "chat", 1, "")
	cardB, _ := cm.GenerateCard(24*time.Hour, "chat", 1, "")
	sessionA, err := cm.ActivateCard(cardA.Code, "machine-presence-a", "fp", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("ActivateCard A returned error: %v", err)
	}
	sessionB, err := cm.ActivateCard(cardB.Code, "machine-presence-b", "fp", "127.0.0.1", "2.0.0")
	if err != nil {
		t.Fatalf("ActivateCard B returned error: %v", err)
	}

	api := NewAPIHandler(cm)
	api.chat = NewChatStore(NewJSONStorage(dir), func() time.Time { return now })

	firstRR := httptest.NewRecorder()
	firstReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/presence", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": sessionA.Token,
		"machine_id":    "machine-presence-a",
		"card":          cardA.Code,
	})
	api.HandleChatPresence(firstRR, firstReq)
	if firstRR.Code != http.StatusOK {
		t.Fatalf("first presence status = %d, body=%s", firstRR.Code, firstRR.Body.String())
	}

	time.Sleep(time.Millisecond)
	secondRR := httptest.NewRecorder()
	secondReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/presence", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": sessionB.Token,
		"machine_id":    "machine-presence-b",
		"card":          cardB.Code,
	})
	api.HandleChatPresence(secondRR, secondReq)
	if secondRR.Code != http.StatusOK {
		t.Fatalf("second presence status = %d, body=%s", secondRR.Code, secondRR.Body.String())
	}

	var resp struct {
		Online int `json:"online"`
	}
	if err := json.Unmarshal(secondRR.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid presence response: %v", err)
	}
	if resp.Online != 2 {
		t.Fatalf("online = %d, want 2", resp.Online)
	}

	now = now.Add(61 * time.Second)
	time.Sleep(time.Millisecond)
	thirdRR := httptest.NewRecorder()
	thirdReq := signedJSONRequest(t, http.MethodPost, "/api/v1/chat/presence", map[string]interface{}{
		"client_id":     "injector_v1",
		"session_token": sessionB.Token,
		"machine_id":    "machine-presence-b",
		"card":          cardB.Code,
	})
	api.HandleChatPresence(thirdRR, thirdReq)
	if thirdRR.Code != http.StatusOK {
		t.Fatalf("third presence status = %d, body=%s", thirdRR.Code, thirdRR.Body.String())
	}
	if err := json.Unmarshal(thirdRR.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid third presence response: %v", err)
	}
	if resp.Online != 1 {
		t.Fatalf("online after expiry = %d, want 1", resp.Online)
	}
}

func signedJSONRequest(t *testing.T, method, path string, payload map[string]interface{}) *http.Request {
	t.Helper()
	time.Sleep(time.Millisecond)
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	addTestHMACHeaders(req, string(body))
	return req
}
