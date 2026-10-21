package com.lingqiao.server.client;

import com.lingqiao.server.common.jdbc.TimestampWriter;
import com.lingqiao.server.cards.CardCodes;
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
import java.util.UUID;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

/**
 * Client activation lifecycle mirroring Go sessions.Service.Activate/Heartbeat/Deactivate
 * and cards.SessionService lifecycle rules.
 */
public class ClientSessionService {
    private final JdbcTemplate jdbc;
    private final TransactionTemplate transactions;
    private final Duration sessionTtl;

    public ClientSessionService(JdbcTemplate jdbc, Duration sessionTtl) {
        this.jdbc = jdbc;
        this.sessionTtl = sessionTtl == null || sessionTtl.isZero() ? Duration.ofHours(4) : sessionTtl;
        this.transactions = new TransactionTemplate(new DataSourceTransactionManager(
            Objects.requireNonNull(jdbc.getDataSource(), "write datasource is required")));
    }

    public ActivationResult activate(ActivationInput input) {
        if (input == null || isBlank(input.cardCode())) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "card is required");
        }
        String normalized = CardCodes.normalize(input.cardCode());
        return transactions.execute(status -> {
            Map<String, Object> card = loadCard(normalized);
            String cardCode = str(card.get("code"));
            Instant now = Instant.now();

            assertCardUsable(card, now);
            assertNotBlacklisted(cardCode, input.machineId());

            String boundMachine = str(card.get("machine_id"));
            int maxSessions = num(card.get("max_sessions"));
            if (maxSessions <= 1 && !boundMachine.isEmpty() && !boundMachine.equals(input.machineId())) {
                throw new SidecarException(ErrorCode.FORBIDDEN, "card is bound to another machine");
            }

            int activeCount = countActiveSessions(cardCode, now);
            if (maxSessions > 0 && activeCount >= maxSessions) {
                throw new SidecarException(ErrorCode.FORBIDDEN,
                    "max sessions reached: " + activeCount + "/" + maxSessions);
            }

            Instant activatedAt = instant(card.get("activated_at"));
            Instant createdAt = instant(card.get("created_at"));
            Instant expiresAt = instant(card.get("expires_at"));
            String nextMachine = boundMachine.isEmpty() ? input.machineId() : boundMachine;
            Instant nextExpiresAt = expiresAt;
            Instant nextActivatedAt = activatedAt;
            if (activatedAt == null) {
                nextActivatedAt = now;
                if (createdAt != null && expiresAt != null) {
                    nextExpiresAt = now.plus(Duration.between(createdAt, expiresAt));
                }
            }

            jdbc.update("UPDATE cards SET machine_id = ?, activated_at = ?, expires_at = ? WHERE code = ?",
                nextMachine,
                nextActivatedAt == null ? null : TimestampWriter.format(nextActivatedAt),
                nextExpiresAt == null ? null : TimestampWriter.format(nextExpiresAt),
                cardCode);

            String token = UUID.randomUUID().toString().replace("-", "");
            Instant sessionExpiresAt = now.plus(sessionTtl);
            jdbc.update("""
                INSERT INTO sessions (token, card_code, machine_id, fingerprint, client_version,
                                      created_at, last_seen, expires_at, remote_addr)
                VALUES (?,?,?,?,?,?,?,?,?)
                """,
                token, cardCode, nullToEmpty(input.machineId()), nullToEmpty(input.fingerprint()),
                nullToEmpty(input.clientVersion()), TimestampWriter.format(now), TimestampWriter.format(now),
                TimestampWriter.format(sessionExpiresAt), nullToEmpty(input.remoteAddr()));

            return new ActivationResult(token, sessionExpiresAt, nextExpiresAt == null ? now : nextExpiresAt);
        });
    }

    public HeartbeatResult heartbeat(HeartbeatInput input) {
        if (input == null || isBlank(input.sessionToken())) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "session_token is required");
        }
        return transactions.execute(status -> {
            Instant now = Instant.now();
            List<Map<String, Object>> rows = jdbc.queryForList(
                "SELECT token, card_code, machine_id, expires_at FROM sessions WHERE token = ?",
                input.sessionToken());
            if (rows.isEmpty()) {
                throw new SidecarException(ErrorCode.UNAUTHORIZED, "session not found");
            }
            Map<String, Object> session = rows.get(0);
            Instant sessionExpiresAt = instant(session.get("expires_at"));
            if (sessionExpiresAt == null || sessionExpiresAt.isBefore(now)) {
                throw new SidecarException(ErrorCode.UNAUTHORIZED, "session expired");
            }
            if (!str(session.get("machine_id")).equals(nullToEmpty(input.machineId()))) {
                throw new SidecarException(ErrorCode.FORBIDDEN, "machine mismatch");
            }

            String cardCode = str(session.get("card_code"));
            Map<String, Object> card = loadCard(cardCode);
            assertCardUsable(card, now);

            Instant nextExpiresAt = now.plus(sessionTtl);
            jdbc.update("UPDATE sessions SET last_seen = ?, expires_at = ?, client_version = ? WHERE token = ?",
                TimestampWriter.format(now), TimestampWriter.format(nextExpiresAt),
                nullToEmpty(input.clientVersion()), input.sessionToken());

            Instant cardExpiresAt = instant(card.get("expires_at"));
            return new HeartbeatResult(nextExpiresAt, cardExpiresAt == null ? now : cardExpiresAt);
        });
    }

    public void deactivate(String sessionToken, String machineId) {
        if (isBlank(sessionToken)) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "session_token is required");
        }
        transactions.executeWithoutResult(status -> {
            List<Map<String, Object>> rows = jdbc.queryForList(
                "SELECT machine_id FROM sessions WHERE token = ?", sessionToken);
            if (rows.isEmpty()) {
                throw new SidecarException(ErrorCode.SESSION_NOT_FOUND, "session not found");
            }
            if (!str(rows.get(0).get("machine_id")).equals(nullToEmpty(machineId))) {
                throw new SidecarException(ErrorCode.FORBIDDEN, "machine mismatch");
            }
            jdbc.update("DELETE FROM sessions WHERE token = ?", sessionToken);
        });
    }

    private Map<String, Object> loadCard(String normalizedCode) {
        List<Map<String, Object>> rows = jdbc.queryForList("""
            SELECT code, machine_id, status, max_sessions, created_at, activated_at, expires_at
            FROM cards
            WHERE code = ? OR REPLACE(REPLACE(UPPER(code), '-', ''), ' ', '') = ?
            LIMIT 1
            """, normalizedCode, normalizedCode);
        if (rows.isEmpty()) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "card not found");
        }
        return rows.get(0);
    }

    private void assertCardUsable(Map<String, Object> card, Instant now) {
        String cardStatus = str(card.get("status"));
        if (cardStatus.equals("disabled") || cardStatus.equals("disable") || cardStatus.equals("archived")) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "card is disabled");
        }
        Instant expiresAt = instant(card.get("expires_at"));
        if (cardStatus.equals("expired") || (expiresAt != null && expiresAt.isBefore(now))) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "card is expired");
        }
    }

    private void assertNotBlacklisted(String cardCode, String machineId) {
        Integer cardHits = jdbc.queryForObject(
            "SELECT COUNT(*) FROM blacklist WHERE value = ?", Integer.class, cardCode);
        if (cardHits != null && cardHits > 0) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "card is blacklisted");
        }
        if (!isBlank(machineId)) {
            Integer machineHits = jdbc.queryForObject(
                "SELECT COUNT(*) FROM blacklist WHERE value = ?", Integer.class, machineId);
            if (machineHits != null && machineHits > 0) {
                throw new SidecarException(ErrorCode.FORBIDDEN, "machine is blacklisted");
            }
        }
    }

    private int countActiveSessions(String cardCode, Instant now) {
        Integer count = jdbc.queryForObject(
            "SELECT COUNT(*) FROM sessions WHERE card_code = ? AND expires_at > ?",
            Integer.class, cardCode, TimestampWriter.format(now));
        return count == null ? 0 : count;
    }

    static Instant instant(Object value) {
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

    private static String str(Object value) {
        return value == null ? "" : String.valueOf(value);
    }

    private static int num(Object value) {
        return value instanceof Number number ? number.intValue() : 0;
    }

    private static boolean isBlank(String value) {
        return value == null || value.isBlank();
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }

    public record ActivationInput(
        String cardCode,
        String machineId,
        String fingerprint,
        String clientVersion,
        String remoteAddr
    ) {
    }

    public record HeartbeatInput(String sessionToken, String machineId, String clientVersion) {
    }

    public record ActivationResult(String sessionToken, Instant sessionExpiresAt, Instant cardExpiresAt) {
    }

    public record HeartbeatResult(Instant sessionExpiresAt, Instant cardExpiresAt) {
    }
}
