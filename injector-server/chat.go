package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	chatMaxMessages  = 200
	chatMaxAge       = 24 * time.Hour
	chatMaxRunes     = 300
	chatSendInterval = 5 * time.Second
	chatPresenceAge  = 60 * time.Second
)

type ChatMessage struct {
	ID            int64               `json:"id"`
	Type          string              `json:"type"`
	AuthorID      string              `json:"author_id,omitempty"`
	Author        string              `json:"author"`
	Content       string              `json:"content"`
	CreatedAt     time.Time           `json:"created_at"`
	ReplyToID     int64               `json:"reply_to_id,omitempty"`
	ReplyPreview  *ChatReplyPreview   `json:"reply_preview,omitempty"`
	Reactions     map[string]int      `json:"reactions,omitempty"`
	Reacted       []string            `json:"reacted,omitempty"`
	ReactionUsers map[string][]string `json:"reaction_users,omitempty"`
	MachineID     string              `json:"-"`
	CardCode      string              `json:"-"`
	Deleted       bool                `json:"deleted,omitempty"`
}

type ChatReplyPreview struct {
	Author  string `json:"author"`
	Content string `json:"content"`
}

type ChatProfile struct {
	AuthorID  string    `json:"author_id"`
	Nickname  string    `json:"nickname"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ChatStore struct {
	mu       sync.Mutex
	storage  *JSONStorage
	now      func() time.Time
	nextID   int64
	messages []ChatMessage
	profiles map[string]ChatProfile
	presence map[string]time.Time
	mutes    map[string]ChatMute
	lastSent map[string]time.Time
}

type ChatMute struct {
	AuthorID string    `json:"author_id"`
	Reason   string    `json:"reason,omitempty"`
	Expires  time.Time `json:"expires_at"`
}

func NewChatStore(storage *JSONStorage, now func() time.Time) *ChatStore {
	if now == nil {
		now = time.Now
	}
	s := &ChatStore{
		storage:  storage,
		now:      now,
		nextID:   1,
		messages: make([]ChatMessage, 0),
		profiles: make(map[string]ChatProfile),
		presence: make(map[string]time.Time),
		mutes:    make(map[string]ChatMute),
		lastSent: make(map[string]time.Time),
	}
	s.load()
	return s
}

func (s *ChatStore) Add(session *Session, content string, replyToID ...int64) (ChatMessage, error) {
	if session == nil {
		return ChatMessage{}, fmt.Errorf("session is required")
	}
	content = normalizeChatContent(content)
	if content == "" {
		return ChatMessage{}, fmt.Errorf("message is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	if s.isMutedLocked(chatAuthorID(session), now) {
		return ChatMessage{}, fmt.Errorf("muted")
	}
	if last, ok := s.lastSent[session.Token]; ok && now.Sub(last) < chatSendInterval {
		return ChatMessage{}, fmt.Errorf("rate limited")
	}
	replyID := int64(0)
	if len(replyToID) > 0 {
		replyID = replyToID[0]
	}
	msg := ChatMessage{
		ID:        s.nextID,
		Type:      "user",
		AuthorID:  chatAuthorID(session),
		Author:    s.nicknameLocked(session),
		Content:   content,
		CreatedAt: now,
		ReplyToID: replyID,
		MachineID: session.MachineID,
		CardCode:  session.CardCode,
	}
	s.nextID++
	s.messages = append(s.messages, msg)
	s.lastSent[session.Token] = now
	s.pruneLocked(now)
	s.saveLocked()
	return s.publicChatMessageLocked(msg), nil
}

func (s *ChatStore) Profile(session *Session) (ChatProfile, error) {
	if session == nil {
		return ChatProfile{}, fmt.Errorf("session is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	authorID := chatAuthorID(session)
	profile, ok := s.profiles[authorID]
	if !ok {
		profile = ChatProfile{
			AuthorID:  authorID,
			Nickname:  chatAuthor(session),
			UpdatedAt: s.now(),
		}
	}
	return profile, nil
}

func (s *ChatStore) SetProfile(session *Session, nickname string) (ChatProfile, error) {
	if session == nil {
		return ChatProfile{}, fmt.Errorf("session is required")
	}
	nickname = normalizeChatNickname(nickname)
	if nickname == "" {
		return ChatProfile{}, fmt.Errorf("nickname is required")
	}
	if !chatNicknameAllowed(nickname) {
		return ChatProfile{}, fmt.Errorf("nickname is not allowed")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isMutedLocked(chatAuthorID(session), s.now()) {
		return ChatProfile{}, fmt.Errorf("muted")
	}
	profile := ChatProfile{
		AuthorID:  chatAuthorID(session),
		Nickname:  nickname,
		UpdatedAt: s.now(),
	}
	s.profiles[profile.AuthorID] = profile
	s.saveLocked()
	return profile, nil
}

func (s *ChatStore) AddSystem(content string) (ChatMessage, error) {
	content = normalizeChatContent(content)
	if content == "" {
		return ChatMessage{}, fmt.Errorf("message is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	msg := ChatMessage{
		ID:        s.nextID,
		Type:      "system",
		Author:    "系统",
		Content:   content,
		CreatedAt: s.now(),
	}
	s.nextID++
	s.messages = append(s.messages, msg)
	s.pruneLocked(msg.CreatedAt)
	s.saveLocked()
	return s.publicChatMessageLocked(msg), nil
}

func (s *ChatStore) Delete(id int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.messages {
		if s.messages[i].ID == id && !s.messages[i].Deleted {
			s.messages[i].Deleted = true
			s.saveLocked()
			return true
		}
	}
	return false
}

func (s *ChatStore) React(session *Session, messageID int64, reaction string) (ChatMessage, error) {
	if session == nil {
		return ChatMessage{}, fmt.Errorf("session is required")
	}
	if messageID <= 0 {
		return ChatMessage{}, fmt.Errorf("message_id is required")
	}
	if !validChatReaction(reaction) {
		return ChatMessage{}, fmt.Errorf("reaction is not allowed")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	authorID := chatAuthorID(session)
	if s.isMutedLocked(authorID, s.now()) {
		return ChatMessage{}, fmt.Errorf("muted")
	}
	for i := range s.messages {
		if s.messages[i].ID != messageID || s.messages[i].Deleted {
			continue
		}
		if s.messages[i].ReactionUsers == nil {
			s.messages[i].ReactionUsers = make(map[string][]string)
		}
		users := s.messages[i].ReactionUsers[reaction]
		if containsString(users, authorID) {
			s.messages[i].ReactionUsers[reaction] = removeString(users, authorID)
		} else {
			s.messages[i].ReactionUsers[reaction] = append(users, authorID)
		}
		s.saveLocked()
		return s.publicChatMessageForAuthorLocked(s.messages[i], authorID), nil
	}
	return ChatMessage{}, fmt.Errorf("message not found")
}

func (s *ChatStore) TouchPresence(session *Session) (int, error) {
	if session == nil {
		return 0, fmt.Errorf("session is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	s.presence[chatAuthorID(session)] = now
	return s.onlineLocked(now), nil
}

func (s *ChatStore) Mute(authorID string, duration time.Duration, reason string) (ChatMute, error) {
	authorID = strings.TrimSpace(authorID)
	if authorID == "" {
		return ChatMute{}, fmt.Errorf("author_id is required")
	}
	if duration <= 0 {
		duration = time.Hour
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	mute := ChatMute{
		AuthorID: authorID,
		Reason:   strings.TrimSpace(reason),
		Expires:  s.now().Add(duration),
	}
	s.mutes[authorID] = mute
	s.saveLocked()
	return mute, nil
}

func (s *ChatStore) List(afterID int64, currentSession ...*Session) []ChatMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked(s.now())
	currentAuthorID := ""
	if len(currentSession) > 0 && currentSession[0] != nil {
		currentAuthorID = chatAuthorID(currentSession[0])
	}
	result := make([]ChatMessage, 0, len(s.messages))
	for _, msg := range s.messages {
		if msg.Deleted {
			continue
		}
		if msg.ID > afterID {
			result = append(result, s.publicChatMessageForAuthorLocked(msg, currentAuthorID))
		}
	}
	return result
}

func (s *ChatStore) AdminList() []ChatMessage {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked(s.now())
	result := make([]ChatMessage, 0, len(s.messages))
	for _, msg := range s.messages {
		public := s.publicChatMessageLocked(msg)
		public.Deleted = msg.Deleted
		result = append(result, public)
	}
	return result
}

func (s *ChatStore) onlineLocked(now time.Time) int {
	cutoff := now.Add(-chatPresenceAge)
	online := 0
	for authorID, seenAt := range s.presence {
		if seenAt.Before(cutoff) {
			delete(s.presence, authorID)
			continue
		}
		online++
	}
	return online
}

func (s *ChatStore) isMutedLocked(authorID string, now time.Time) bool {
	mute, ok := s.mutes[authorID]
	if !ok {
		return false
	}
	if !mute.Expires.After(now) {
		delete(s.mutes, authorID)
		return false
	}
	return true
}

func (s *ChatStore) load() {
	if s.storage == nil {
		return
	}
	var data struct {
		NextID   int64                  `json:"next_id"`
		Messages []ChatMessage          `json:"messages"`
		Profiles map[string]ChatProfile `json:"profiles"`
		Mutes    map[string]ChatMute    `json:"mutes"`
	}
	if err := s.storage.Load("chat", &data); err != nil {
		return
	}
	if data.NextID > 0 {
		s.nextID = data.NextID
	}
	if data.Messages != nil {
		s.messages = data.Messages
		for _, msg := range data.Messages {
			if msg.ID >= s.nextID {
				s.nextID = msg.ID + 1
			}
		}
	}
	if data.Profiles != nil {
		s.profiles = data.Profiles
	}
	if data.Mutes != nil {
		now := s.now()
		for authorID, mute := range data.Mutes {
			if mute.Expires.After(now) {
				s.mutes[authorID] = mute
			}
		}
	}
}

func (s *ChatStore) saveLocked() {
	if s.storage == nil {
		return
	}
	data := struct {
		NextID   int64                  `json:"next_id"`
		Messages []ChatMessage          `json:"messages"`
		Profiles map[string]ChatProfile `json:"profiles"`
		Mutes    map[string]ChatMute    `json:"mutes"`
	}{
		NextID:   s.nextID,
		Messages: append([]ChatMessage(nil), s.messages...),
		Profiles: cloneChatProfiles(s.profiles),
		Mutes:    cloneActiveChatMutes(s.mutes, s.now()),
	}
	if err := s.storage.Save("chat", &data); err != nil {
		fmt.Printf("[CHAT] save failed: %v\n", err)
	}
}

func (s *ChatStore) pruneLocked(now time.Time) {
	cutoff := now.Add(-chatMaxAge)
	start := 0
	for start < len(s.messages) && s.messages[start].CreatedAt.Before(cutoff) {
		start++
	}
	if start > 0 {
		s.messages = append([]ChatMessage(nil), s.messages[start:]...)
	}
	if len(s.messages) > chatMaxMessages {
		s.messages = append([]ChatMessage(nil), s.messages[len(s.messages)-chatMaxMessages:]...)
	}
}

func normalizeChatContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	runes := []rune(content)
	if len(runes) > chatMaxRunes {
		content = string(runes[:chatMaxRunes])
	}
	if !utf8.ValidString(content) {
		return ""
	}
	return content
}

func normalizeChatNickname(nickname string) string {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" || !utf8.ValidString(nickname) {
		return ""
	}
	runes := []rune(nickname)
	if len(runes) > 16 {
		return ""
	}
	return nickname
}

func chatNicknameAllowed(nickname string) bool {
	lower := strings.ToLower(nickname)
	for _, word := range []string{"admin", "管理员", "系统", "官方"} {
		if strings.Contains(lower, word) {
			return false
		}
	}
	return true
}

func replyPreviewContent(content string) string {
	runes := []rune(content)
	if len(runes) > 60 {
		return string(runes[:60])
	}
	return content
}

func validChatReaction(reaction string) bool {
	switch reaction {
	case "👍", "❤️", "😂", "？", "收到":
		return true
	default:
		return false
	}
}

func (s *ChatStore) nicknameLocked(session *Session) string {
	if profile, ok := s.profiles[chatAuthorID(session)]; ok && profile.Nickname != "" {
		return profile.Nickname
	}
	return chatAuthor(session)
}

func chatAuthor(session *Session) string {
	return "用户-" + strings.ToUpper(chatAuthorID(session)[:4])
}

func chatAuthorID(session *Session) string {
	sum := sha256.Sum256([]byte(session.CardCode + "|" + session.MachineID))
	return hex.EncodeToString(sum[:])[:16]
}

func cloneChatProfiles(profiles map[string]ChatProfile) map[string]ChatProfile {
	cloned := make(map[string]ChatProfile, len(profiles))
	for k, v := range profiles {
		cloned[k] = v
	}
	return cloned
}

func cloneActiveChatMutes(mutes map[string]ChatMute, now time.Time) map[string]ChatMute {
	cloned := make(map[string]ChatMute, len(mutes))
	for authorID, mute := range mutes {
		if mute.Expires.After(now) {
			cloned[authorID] = mute
		}
	}
	return cloned
}

func (s *ChatStore) publicChatMessageLocked(msg ChatMessage) ChatMessage {
	return s.publicChatMessageForAuthorLocked(msg, "")
}

func (s *ChatStore) publicChatMessageForAuthorLocked(msg ChatMessage, currentAuthorID string) ChatMessage {
	if msg.Type == "" {
		msg.Type = "user"
	}
	msg.ReplyPreview = nil
	if msg.ReplyToID > 0 {
		for _, source := range s.messages {
			if source.ID == msg.ReplyToID && !source.Deleted {
				msg.ReplyPreview = &ChatReplyPreview{
					Author:  source.Author,
					Content: replyPreviewContent(source.Content),
				}
				break
			}
		}
	}
	msg.Reactions = reactionCounts(msg.ReactionUsers)
	msg.Reacted = reactedList(msg.ReactionUsers, currentAuthorID)
	msg.ReactionUsers = nil
	msg.MachineID = ""
	msg.CardCode = ""
	msg.Deleted = false
	return msg
}

func reactionCounts(users map[string][]string) map[string]int {
	counts := make(map[string]int)
	for reaction, ids := range users {
		if len(ids) > 0 {
			counts[reaction] = len(ids)
		}
	}
	return counts
}

func reactedList(users map[string][]string, authorID string) []string {
	if authorID == "" {
		return nil
	}
	reactions := make([]string, 0)
	for reaction, ids := range users {
		if containsString(ids, authorID) {
			reactions = append(reactions, reaction)
		}
	}
	return reactions
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeString(values []string, target string) []string {
	result := values[:0]
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func (h *APIHandler) HandleChatSend(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	req, err := h.readSignedRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	session, err := h.validateBoundSession(req.SessionToken, req.MachineID, req.Card)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	msg, err := h.chat.Add(session, req.Content, req.ReplyToID)
	if err != nil {
		if err.Error() == "muted" {
			writeError(w, http.StatusForbidden, "你已被禁言，暂时无法发送消息")
			return
		}
		if err.Error() == "rate limited" {
			writeError(w, http.StatusTooManyRequests, "发送过于频繁，请稍后再试")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"message": msg})
}

func (h *APIHandler) HandleChatProfile(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	req, err := h.readSignedRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	session, err := h.validateBoundSession(req.SessionToken, req.MachineID, req.Card)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var profile ChatProfile
	if req.Nickname != "" {
		profile, err = h.chat.SetProfile(session, req.Nickname)
	} else {
		profile, err = h.chat.Profile(session)
	}
	if err != nil {
		if err.Error() == "muted" {
			writeError(w, http.StatusForbidden, "你已被禁言，暂时无法修改昵称")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"profile": profile})
}

func (h *APIHandler) HandleChatReact(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	req, err := h.readSignedRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	session, err := h.validateBoundSession(req.SessionToken, req.MachineID, req.Card)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	msg, err := h.chat.React(session, req.MessageID, req.Reaction)
	if err != nil {
		if err.Error() == "muted" {
			writeError(w, http.StatusForbidden, "你已被禁言，暂时无法添加表情")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"message": msg})
}

func (h *APIHandler) HandleChatPresence(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	req, err := h.readSignedRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	session, err := h.validateBoundSession(req.SessionToken, req.MachineID, req.Card)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	online, err := h.chat.TouchPresence(session)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"online": online})
}

func (h *APIHandler) HandleChatMessages(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	req, err := h.readSignedRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	session, err := h.validateBoundSession(req.SessionToken, req.MachineID, req.Card)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	writeOK(w, map[string]interface{}{"messages": h.chat.List(req.AfterID, session)})
}
