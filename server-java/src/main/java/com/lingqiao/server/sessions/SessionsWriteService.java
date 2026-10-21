package com.lingqiao.server.sessions;

import com.lingqiao.server.common.jdbc.TimestampWriter;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.sql.Timestamp;
import java.time.Duration;
import java.time.Instant;
import java.time.LocalDateTime;
import java.time.OffsetDateTime;
import java.time.ZoneOffset;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

/**
 * Java write path for session termination. Mirrors Go sessions.Service.Deactivate
 * for the admin force-logout and client deactivate flows.
 */
public class SessionsWriteService {
    private final JdbcTemplate jdbc;
    private final TransactionTemplate transactions;

    public SessionsWriteService(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
        this.transactions = new TransactionTemplate(new DataSourceTransactionManager(
            Objects.requireNonNull(jdbc.getDataSource(), "write datasource is required")));
    }

    /** Admin force logout: no machine/card ownership checks, no card unbind. */
    public void forceLogout(String sessionToken) {
        deactivate(new DeactivateInput(sessionToken, null, null, false, true));
    }

    /** Client deactivate: enforces ownership and optionally unbinds the card. */
    public void deactivate(DeactivateInput input) {
        String token = input.sessionToken();
        if (token == null || token.isBlank()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "session_token is required");
        }
        transactions.executeWithoutResult(status -> {
            Map<String, Object> session = loadSession(token);
            String cardCode = String.valueOf(session.getOrDefault("card_code", ""));
            String machineId = String.valueOf(session.getOrDefault("machine_id", ""));

            if (!input.adminForce()) {
                if (input.machineId() != null && !input.machineId().isBlank()
                    && !input.machineId().equals(machineId)) {
                    throw new SidecarException(ErrorCode.FORBIDDEN, "machine mismatch");
                }
                if (input.cardCode() != null && !input.cardCode().isBlank()
                    && !input.cardCode().equals(cardCode)) {
                    throw new SidecarException(ErrorCode.FORBIDDEN, "session card mismatch");
                }
            }

            if (input.unbind()) {
                unbindCardPreservingRemaining(cardCode, Instant.now());
            }

            if (jdbc.update("DELETE FROM sessions WHERE token = ?", token) == 0) {
                throw new SidecarException(ErrorCode.SESSION_NOT_FOUND, "session not found");
            }
        });
    }

    private Map<String, Object> loadSession(String token) {
        List<Map<String, Object>> rows = jdbc.queryForList(
            "SELECT token, card_code, machine_id FROM sessions WHERE token = ?", token);
        if (rows.isEmpty()) {
            throw new SidecarException(ErrorCode.SESSION_NOT_FOUND, "session not found");
        }
        return rows.get(0);
    }

    private void unbindCardPreservingRemaining(String cardCode, Instant now) {
        if (cardCode == null || cardCode.isBlank()) {
            return;
        }
        List<Map<String, Object>> rows = jdbc.queryForList(
            "SELECT activated_at, expires_at FROM cards WHERE code = ?", cardCode);
        if (rows.isEmpty()) {
            throw new SidecarException(ErrorCode.CARD_NOT_FOUND, "card not found");
        }
        Map<String, Object> card = rows.get(0);
        Instant activatedAt = toInstant(card.get("activated_at"));
        Instant expiresAt = toInstant(card.get("expires_at"));
        Instant nextExpiresAt = expiresAt;
        if (activatedAt != null && expiresAt != null) {
            nextExpiresAt = now.plus(Duration.between(activatedAt, expiresAt));
        }
        jdbc.update(
            "UPDATE cards SET machine_id = '', activated_at = NULL, expires_at = ? WHERE code = ?",
            nextExpiresAt == null ? null : TimestampWriter.format(nextExpiresAt), cardCode);
    }

    static Instant toInstant(Object value) {
        if (value instanceof Timestamp timestamp) {
            return timestamp.toInstant();
        }
        if (value instanceof Number epochMillis) {
            return Instant.ofEpochMilli(epochMillis.longValue());
        }
        if (value instanceof String text && !text.isBlank()) {
            String normalized = text.trim().replace(' ', 'T');
            try {
                return Instant.parse(normalized.endsWith("Z") ? normalized : normalized + "Z");
            } catch (Exception ignored) {
                // fall through
            }
            try {
                return OffsetDateTime.parse(normalized).toInstant();
            } catch (Exception ignored) {
                // fall through
            }
            try {
                return LocalDateTime.parse(normalized).toInstant(ZoneOffset.UTC);
            } catch (Exception ignored) {
                return null;
            }
        }
        return null;
    }

    public record DeactivateInput(
        String sessionToken,
        String machineId,
        String cardCode,
        boolean unbind,
        boolean adminForce
    ) {
    }
}
