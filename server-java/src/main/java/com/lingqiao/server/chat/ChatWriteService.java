package com.lingqiao.server.chat;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.time.Duration;
import java.time.Instant;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Java write path for community chat moderation.
 * Mirrors Go chat.Service AddSystem/Delete/Mute plus retention pruning.
 */
public class ChatWriteService {
    static final int MAX_CHAT_RUNES = 300;
    static final Duration MAX_AGE = Duration.ofHours(24);
    static final Duration DEFAULT_MUTE = Duration.ofHours(1);

    private final Path dataDir;
    private final ObjectMapper mapper;

    public ChatWriteService(Path dataDir) {
        this.dataDir = dataDir.toAbsolutePath().normalize();
        this.mapper = new ObjectMapper()
            .registerModule(new JavaTimeModule())
            .disable(SerializationFeature.WRITE_DATES_AS_TIMESTAMPS)
            .enable(SerializationFeature.INDENT_OUTPUT);
    }

    public synchronized ChatDocument addSystemMessage(String content) {
        String normalized = normalizeContent(content);
        if (normalized.isEmpty()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "message is required");
        }
        ChatState state = readState();
        Instant now = Instant.now();
        long nextId = state.getNextId() <= 0 ? 1 : state.getNextId();

        ChatDocument message = new ChatDocument();
        message.setId(nextId);
        message.setType("system");
        message.setAuthor("系统");
        message.setContent(normalized);
        message.setCreatedAt(now);

        List<ChatDocument> messages = new ArrayList<>(state.getMessages());
        messages.add(message);
        state.setMessages(prune(messages, now));
        state.setNextId(nextId + 1);
        writeState(state);
        return message;
    }

    public synchronized void deleteMessage(long id) {
        if (id <= 0) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "id is required");
        }
        ChatState state = readState();
        boolean deleted = false;
        for (ChatDocument message : state.getMessages()) {
            if (message.getId() == id && !message.isDeleted()) {
                message.setDeleted(true);
                deleted = true;
                break;
            }
        }
        if (!deleted) {
            throw new SidecarException(ErrorCode.NOT_FOUND, "message not found");
        }
        writeState(state);
    }

    public synchronized ChatMuteDocument mute(String authorId, int durationMinutes, String reason) {
        String trimmed = authorId == null ? "" : authorId.trim();
        if (trimmed.isEmpty()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "author_id is required");
        }
        Duration duration = durationMinutes > 0 ? Duration.ofMinutes(durationMinutes) : DEFAULT_MUTE;

        ChatState state = readState();
        ChatMuteDocument mute = new ChatMuteDocument();
        mute.setAuthorId(trimmed);
        mute.setReason(reason == null ? "" : reason.trim());
        mute.setExpires(Instant.now().plus(duration));

        Map<String, ChatMuteDocument> mutes = new LinkedHashMap<>(state.getMutes());
        mutes.put(trimmed, mute);
        state.setMutes(pruneExpiredMutes(mutes, Instant.now()));
        writeState(state);
        return mute;
    }

    static String normalizeContent(String content) {
        if (content == null) {
            return "";
        }
        String trimmed = content.trim();
        if (trimmed.codePointCount(0, trimmed.length()) <= MAX_CHAT_RUNES) {
            return trimmed;
        }
        int end = trimmed.offsetByCodePoints(0, MAX_CHAT_RUNES);
        return trimmed.substring(0, end);
    }

    static List<ChatDocument> prune(List<ChatDocument> messages, Instant now) {
        Instant cutoff = now.minus(MAX_AGE);
        List<ChatDocument> retained = new ArrayList<>(messages.size());
        for (ChatDocument message : messages) {
            Instant createdAt = message.getCreatedAt();
            if (createdAt == null || !createdAt.isBefore(cutoff)) {
                retained.add(message);
            }
        }
        return retained;
    }

    static Map<String, ChatMuteDocument> pruneExpiredMutes(Map<String, ChatMuteDocument> mutes, Instant now) {
        Map<String, ChatMuteDocument> active = new LinkedHashMap<>();
        mutes.forEach((key, mute) -> {
            if (mute.getExpires() != null && mute.getExpires().isAfter(now)) {
                active.put(key, mute);
            }
        });
        return active;
    }

    private ChatState readState() {
        Path path = statePath();
        if (!Files.exists(path)) {
            return new ChatState();
        }
        try {
            ChatState state = mapper.readValue(path.toFile(), ChatState.class);
            return state == null ? new ChatState() : state;
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to read chat state");
        }
    }

    private void writeState(ChatState state) {
        Path target = statePath();
        try {
            Files.createDirectories(target.getParent());
            Path tmp = Files.createTempFile(target.getParent(), "chat", ".tmp");
            Files.write(tmp, mapper.writeValueAsBytes(state));
            Files.move(tmp, target, StandardCopyOption.REPLACE_EXISTING, StandardCopyOption.ATOMIC_MOVE);
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to persist chat state");
        }
    }

    private Path statePath() {
        Path path = dataDir.resolve("chat.json").toAbsolutePath().normalize();
        if (!path.startsWith(dataDir)) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid chat state path");
        }
        return path;
    }
}
