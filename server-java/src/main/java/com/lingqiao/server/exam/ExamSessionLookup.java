package com.lingqiao.server.exam;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.sql.Timestamp;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import org.springframework.jdbc.core.JdbcTemplate;

/**
 * Resolves an active client session for exam ingestion.
 * Mirrors Go sessions.Service.ValidateActiveToken for the fields exam needs.
 */
public class ExamSessionLookup {
    private final JdbcTemplate jdbc;

    public ExamSessionLookup(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    public ExamSession requireActive(String token) {
        if (token == null || token.isEmpty()) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "session_token required");
        }
        List<Map<String, Object>> rows = jdbc.queryForList(
            "SELECT card_code, machine_id, expires_at FROM sessions WHERE token = ?", token);
        if (rows.isEmpty()) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "invalid session");
        }
        Map<String, Object> row = rows.get(0);
        Instant expiresAt = toInstant(row.get("expires_at"));
        if (expiresAt == null || Instant.now().isAfter(expiresAt)) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "invalid session");
        }
        return new ExamSession(
            String.valueOf(row.getOrDefault("card_code", "")),
            String.valueOf(row.getOrDefault("machine_id", ""))
        );
    }

    private static Instant toInstant(Object value) {
        if (value instanceof Timestamp timestamp) {
            return timestamp.toInstant();
        }
        if (value instanceof Number epochMillis) {
            return Instant.ofEpochMilli(epochMillis.longValue());
        }
        if (value instanceof String text && !text.isBlank()) {
            return parseText(text.trim());
        }
        return null;
    }

    private static Instant parseText(String text) {
        String normalized = text.replace(' ', 'T');
        try {
            return Instant.parse(normalized.endsWith("Z") ? normalized : normalized + "Z");
        } catch (Exception ignored) {
            // fall through to offset/local parsing
        }
        try {
            return java.time.OffsetDateTime.parse(normalized).toInstant();
        } catch (Exception ignored) {
            // fall through
        }
        try {
            return java.time.LocalDateTime.parse(normalized).toInstant(java.time.ZoneOffset.UTC);
        } catch (Exception ignored) {
            return null;
        }
    }

    public record ExamSession(String cardCode, String machineId) {
    }
}
