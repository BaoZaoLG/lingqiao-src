package com.lingqiao.server.client;

import com.lingqiao.server.common.jdbc.TimestampWriter;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.sql.Timestamp;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import org.springframework.jdbc.core.JdbcTemplate;

/**
 * Validates client session tokens for download endpoints.
 * Mirrors Go sessions.Service.ValidateActiveToken / ValidateBound.
 */
public class ClientSessionValidator {
    private final JdbcTemplate jdbc;

    public ClientSessionValidator(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    public ValidatedSession validateActive(String token) {
        if (token == null || token.isEmpty()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "token is required");
        }
        List<Map<String, Object>> rows = jdbc.queryForList(
            "SELECT card_code, machine_id, expires_at FROM sessions WHERE token = ?", token);
        if (rows.isEmpty()) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "invalid or expired session");
        }
        Map<String, Object> row = rows.get(0);
        Instant expiresAt = ClientSessionService.instant(row.get("expires_at"));
        if (expiresAt == null || expiresAt.isBefore(Instant.now())) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "invalid or expired session");
        }
        return new ValidatedSession(
            String.valueOf(row.getOrDefault("card_code", "")),
            String.valueOf(row.getOrDefault("machine_id", "")));
    }

    public ValidatedSession validateBound(String token, String machineId, String cardCode) {
        if (token == null || token.isEmpty()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "session_token is required");
        }
        if (machineId == null || machineId.isEmpty()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "machine_id is required");
        }
        ValidatedSession session = validateActive(token);
        if (!machineId.equals(session.machineId())) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid or expired session");
        }
        if (cardCode != null && !cardCode.isEmpty()
            && !com.lingqiao.server.cards.CardCodes.same(cardCode, session.cardCode())) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid or expired session");
        }
        List<Map<String, Object>> cards = jdbc.queryForList(
            "SELECT status, expires_at FROM cards WHERE code = ?", session.cardCode());
        if (cards.isEmpty()) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "card not found");
        }
        Map<String, Object> card = cards.get(0);
        String status = String.valueOf(card.getOrDefault("status", ""));
        if ("disabled".equals(status) || "disable".equals(status)) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "card disabled");
        }
        Instant cardExpiresAt = ClientSessionService.instant(card.get("expires_at"));
        if ("expired".equals(status)
            || (cardExpiresAt != null && cardExpiresAt.isBefore(Instant.now()))) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "card expired");
        }
        return session;
    }

    public void touch(String token) {
        jdbc.update("UPDATE sessions SET last_seen = ? WHERE token = ?",
            TimestampWriter.format(Instant.now()), token);
    }

    public record ValidatedSession(String cardCode, String machineId) {
    }
}
