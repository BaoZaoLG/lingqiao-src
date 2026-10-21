package com.lingqiao.server.chat;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.HexFormat;
import java.util.Locale;
import java.util.Set;

/**
 * Chat identity and validation helpers mirroring Go chat helper functions:
 * chatAuthorID, chatAuthor, normalizeChatNickname, chatNicknameAllowed,
 * validChatReaction and replyPreviewContent.
 */
public final class ChatIdentity {
    public static final int MAX_NICKNAME_RUNES = 16;
    public static final int REPLY_PREVIEW_RUNES = 60;

    private static final Set<String> BANNED_NICKNAME_WORDS =
        Set.of("admin", "管理员", "系统", "官方");
    private static final Set<String> VALID_REACTIONS =
        Set.of("👍", "❤️", "😂", "？", "收到");

    private ChatIdentity() {
    }

    public static String authorId(String cardCode, String machineId) {
        String material = nullToEmpty(cardCode) + "|" + nullToEmpty(machineId);
        byte[] digest = sha256(material.getBytes(StandardCharsets.UTF_8));
        return HexFormat.of().formatHex(digest).substring(0, 16);
    }

    public static String defaultAuthorName(String authorId) {
        String prefix = authorId == null || authorId.length() < 4 ? "0000" : authorId.substring(0, 4);
        return "用户-" + prefix.toUpperCase(Locale.ROOT);
    }

    public static String normalizeNickname(String nickname) {
        if (nickname == null) {
            return "";
        }
        String trimmed = nickname.trim();
        if (trimmed.isEmpty()) {
            return "";
        }
        if (trimmed.codePointCount(0, trimmed.length()) > MAX_NICKNAME_RUNES) {
            return "";
        }
        return trimmed;
    }

    public static boolean nicknameAllowed(String nickname) {
        String lower = nickname == null ? "" : nickname.toLowerCase(Locale.ROOT);
        for (String word : BANNED_NICKNAME_WORDS) {
            if (lower.contains(word)) {
                return false;
            }
        }
        return true;
    }

    public static boolean validReaction(String reaction) {
        return reaction != null && VALID_REACTIONS.contains(reaction);
    }

    public static String replyPreview(String content) {
        if (content == null) {
            return "";
        }
        if (content.codePointCount(0, content.length()) <= REPLY_PREVIEW_RUNES) {
            return content;
        }
        return content.substring(0, content.offsetByCodePoints(0, REPLY_PREVIEW_RUNES));
    }

    private static byte[] sha256(byte[] data) {
        try {
            return MessageDigest.getInstance("SHA-256").digest(data);
        } catch (Exception ex) {
            throw new IllegalStateException("sha256 unavailable", ex);
        }
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }
}
