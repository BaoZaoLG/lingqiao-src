package com.lingqiao.server.chat;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

/**
 * Client-facing chat service mirroring Go chat.Service:
 * Add/List/Profile/SetProfile/React/TouchPresence with mute checks,
 * send rate limiting, retention pruning and public message projection.
 */
public class ClientChatService {
    static final Duration SEND_INTERVAL = Duration.ofSeconds(5);
    static final Duration PRESENCE_AGE = Duration.ofSeconds(60);

    private final ChatStateStore store;
    private final Map<String, Instant> lastSent = new ConcurrentHashMap<>();
    private final Map<String, Instant> presence = new ConcurrentHashMap<>();

    public ClientChatService(ChatStateStore store) {
        this.store = store;
    }

    public synchronized ChatMessageView add(ClientIdentity identity, String content, long replyToId) {
        String normalized = ChatWriteService.normalizeContent(content);
        if (normalized.isEmpty()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "message is required");
        }
        Instant now = Instant.now();
        ChatState state = store.read();
        requireNotMuted(state, identity.authorId(), now);

        Instant last = lastSent.get(identity.sessionToken());
        if (last != null && Duration.between(last, now).compareTo(SEND_INTERVAL) < 0) {
            throw new SidecarException(ErrorCode.TOO_MANY_REQUESTS, "发送过于频繁，请稍后再试");
        }

        long nextId = state.getNextId() <= 0 ? 1 : state.getNextId();
        ChatDocument message = new ChatDocument();
        message.setId(nextId);
        message.setType("user");
        message.setAuthorId(identity.authorId());
        message.setAuthor(nicknameOf(state, identity));
        message.setContent(normalized);
        message.setCreatedAt(now);
        message.setReplyToId(replyToId > 0 ? replyToId : 0);

        List<ChatDocument> messages = new ArrayList<>(state.getMessages());
        messages.add(message);
        state.setMessages(ChatWriteService.prune(messages, now));
        state.setNextId(nextId + 1);
        store.write(state);

        lastSent.put(identity.sessionToken(), now);
        return project(state, message, identity.authorId());
    }

    public synchronized List<ChatMessageView> list(ClientIdentity identity, long afterId) {
        Instant now = Instant.now();
        ChatState state = store.read();
        state.setMessages(ChatWriteService.prune(new ArrayList<>(state.getMessages()), now));

        List<ChatMessageView> views = new ArrayList<>();
        for (ChatDocument message : state.getMessages()) {
            if (message.isDeleted() || message.getId() <= afterId) {
                continue;
            }
            views.add(project(state, message, identity.authorId()));
        }
        return views;
    }

    public synchronized ChatProfileView profile(ClientIdentity identity) {
        ChatState state = store.read();
        Object stored = state.getProfiles().get(identity.authorId());
        String nickname = nicknameFromProfile(stored);
        if (nickname.isEmpty()) {
            nickname = ChatIdentity.defaultAuthorName(identity.authorId());
        }
        return new ChatProfileView(identity.authorId(), nickname);
    }

    public synchronized ChatProfileView setProfile(ClientIdentity identity, String nickname) {
        String normalized = ChatIdentity.normalizeNickname(nickname);
        if (normalized.isEmpty()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "nickname is required");
        }
        if (!ChatIdentity.nicknameAllowed(normalized)) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "nickname is not allowed");
        }
        Instant now = Instant.now();
        ChatState state = store.read();
        requireNotMuted(state, identity.authorId(), now);

        Map<String, Object> profile = new LinkedHashMap<>();
        profile.put("author_id", identity.authorId());
        profile.put("nickname", normalized);
        profile.put("updated_at", now.toString());

        Map<String, Object> profiles = new LinkedHashMap<>(state.getProfiles());
        profiles.put(identity.authorId(), profile);
        state.setProfiles(profiles);
        store.write(state);
        return new ChatProfileView(identity.authorId(), normalized);
    }

    public synchronized ChatMessageView react(ClientIdentity identity, long messageId, String reaction) {
        if (messageId <= 0) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "message_id is required");
        }
        if (!ChatIdentity.validReaction(reaction)) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "reaction is not allowed");
        }
        Instant now = Instant.now();
        ChatState state = store.read();
        requireNotMuted(state, identity.authorId(), now);

        for (ChatDocument message : state.getMessages()) {
            if (message.getId() != messageId || message.isDeleted()) {
                continue;
            }
            Map<String, List<String>> users = new LinkedHashMap<>(message.getReactionUsers());
            List<String> reacted = new ArrayList<>(users.getOrDefault(reaction, List.of()));
            if (reacted.contains(identity.authorId())) {
                reacted.remove(identity.authorId());
            } else {
                reacted.add(identity.authorId());
            }
            users.put(reaction, reacted);
            message.setReactionUsers(users);
            store.write(state);
            return project(state, message, identity.authorId());
        }
        throw new SidecarException(ErrorCode.NOT_FOUND, "message not found");
    }

    public synchronized int touchPresence(ClientIdentity identity) {
        Instant now = Instant.now();
        presence.put(identity.authorId(), now);
        Instant cutoff = now.minus(PRESENCE_AGE);
        presence.entrySet().removeIf(entry -> entry.getValue().isBefore(cutoff));
        return presence.size();
    }

    private void requireNotMuted(ChatState state, String authorId, Instant now) {
        ChatMuteDocument mute = state.getMutes().get(authorId);
        if (mute != null && mute.getExpires() != null && mute.getExpires().isAfter(now)) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "你已被禁言，暂时无法发送消息");
        }
    }

    private String nicknameOf(ChatState state, ClientIdentity identity) {
        String nickname = nicknameFromProfile(state.getProfiles().get(identity.authorId()));
        return nickname.isEmpty() ? ChatIdentity.defaultAuthorName(identity.authorId()) : nickname;
    }

    @SuppressWarnings("unchecked")
    private static String nicknameFromProfile(Object stored) {
        if (stored instanceof Map<?, ?> map) {
            Object nickname = ((Map<String, Object>) map).get("nickname");
            return nickname == null ? "" : String.valueOf(nickname);
        }
        return "";
    }

    ChatMessageView project(ChatState state, ChatDocument message, String currentAuthorId) {
        ChatReplyPreview preview = null;
        if (message.getReplyToId() > 0) {
            for (ChatDocument source : state.getMessages()) {
                if (source.getId() == message.getReplyToId() && !source.isDeleted()) {
                    preview = new ChatReplyPreview(
                        source.getAuthor(), ChatIdentity.replyPreview(source.getContent()));
                    break;
                }
            }
        }

        Map<String, Integer> counts = new LinkedHashMap<>();
        List<String> reacted = new ArrayList<>();
        message.getReactionUsers().forEach((reaction, users) -> {
            if (users != null && !users.isEmpty()) {
                counts.put(reaction, users.size());
                if (!currentAuthorId.isEmpty() && users.contains(currentAuthorId)) {
                    reacted.add(reaction);
                }
            }
        });

        String type = message.getType() == null || message.getType().isEmpty() ? "user" : message.getType();
        return new ChatMessageView(
            message.getId(), type, message.getAuthorId(), message.getAuthor(), message.getContent(),
            message.getCreatedAt(), message.getReplyToId(), preview, counts,
            currentAuthorId.isEmpty() ? null : reacted);
    }

    public record ClientIdentity(String sessionToken, String authorId) {
    }

    public record ChatProfileView(String authorId, String nickname) {
    }

    public record ChatReplyPreview(String author, String content) {
    }

    public record ChatMessageView(
        long id,
        String type,
        String authorId,
        String author,
        String content,
        Instant createdAt,
        long replyToId,
        ChatReplyPreview replyPreview,
        Map<String, Integer> reactions,
        List<String> reacted
    ) {
    }
}
