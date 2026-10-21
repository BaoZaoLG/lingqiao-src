package com.lingqiao.server.cards;

import com.lingqiao.server.common.jdbc.TimestampReader;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Timestamp;
import java.time.Instant;
import java.time.format.DateTimeParseException;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.List;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;

public class CardsService {
    private static final RowMapper<Card> ROW_MAPPER = (rs, rowNum) -> mapCard(rs);

    private final JdbcTemplate jdbc;

    public CardsService(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    public List<Card> list(CardListFilter filter) {
        return list(filter, Instant.now());
    }

    /**
     * Agent portal compatibility path: Go returns the stored card status here
     * and only computes expiry dynamically for dashboard counters.
     */
    public List<Card> listStoredByAgent(String agentId) {
        return jdbc.query("""
            SELECT code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions
            FROM cards WHERE agent_id = ? ORDER BY created_at DESC
            """, ROW_MAPPER, agentId);
    }

    /** Raw stored status path used by Go-compatible machine detail responses. */
    public List<Card> listStoredByMachine(String machineId) {
        return jdbc.query("""
            SELECT code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions
            FROM cards WHERE machine_id = ? ORDER BY created_at DESC
            """, ROW_MAPPER, machineId);
    }

    /** Visible for testing so the effective-status clock is deterministic. */
    List<Card> list(CardListFilter filter, Instant now) {
        List<Object> args = new ArrayList<>();
        StringBuilder sql = new StringBuilder("""
            SELECT code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions
            FROM cards WHERE 1=1
            """);
        if (hasText(filter.query())) {
            sql.append(" AND (code LIKE ? OR note LIKE ?)");
            String like = "%" + filter.query() + "%";
            args.add(like);
            args.add(like);
        }
        String sqlStatus = filter.sqlStatus();
        if (hasText(sqlStatus)) {
            sql.append(" AND status = ?");
            args.add(sqlStatus);
        }
        if (hasText(filter.machineId())) {
            sql.append(" AND machine_id = ?");
            args.add(filter.machineId());
        }
        if (hasText(filter.agentId())) {
            sql.append(" AND agent_id = ?");
            args.add(filter.agentId());
        }
        sql.append(" ORDER BY created_at DESC");

        List<Card> rows = jdbc.query(sql.toString(), ROW_MAPPER, args.toArray());
        return applyAdvancedFilters(rows, filter, now);
    }

    /**
     * Mirrors Go's in-memory pass: bound/expiry/max-session filters, then the
     * effective status is recomputed and matched against the requested status.
     */
    static List<Card> applyAdvancedFilters(List<Card> rows, CardListFilter filter, Instant now) {
        String requestedStatus = filter.status();
        List<Card> filtered = new ArrayList<>(rows.size());
        for (Card card : rows) {
            if (filter.bound() != null) {
                boolean isBound = card.machineId() != null && !card.machineId().isEmpty();
                if (filter.bound() != isBound) {
                    continue;
                }
            }
            if (filter.expiresBefore() != null
                && card.expiresAt() != null
                && card.expiresAt().isAfter(filter.expiresBefore())) {
                continue;
            }
            if (filter.expiresAfter() != null
                && card.expiresAt() != null
                && card.expiresAt().isBefore(filter.expiresAfter())) {
                continue;
            }
            if (filter.maxSessions() != null && card.maxSessions() != filter.maxSessions()) {
                continue;
            }
            String effective = CardStatuses.effective(card.status(), card.expiresAt(), now);
            if (hasText(requestedStatus) && !effective.equals(requestedStatus)) {
                continue;
            }
            filtered.add(new Card(
                card.code(), card.machineId(), card.agentId(), card.createdAt(),
                card.activatedAt(), card.expiresAt(), effective, card.note(), card.maxSessions()));
        }
        filtered.sort(Comparator.comparing(
            Card::createdAt, Comparator.nullsLast(Comparator.reverseOrder())));
        return filtered;
    }

    public Card get(String code) {
        List<Card> cards = jdbc.query("""
            SELECT code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions
            FROM cards WHERE code = ?
            """, ROW_MAPPER, code);
        if (cards.isEmpty()) {
            throw new SidecarException(ErrorCode.CARD_NOT_FOUND, "card not found");
        }
        return cards.get(0);
    }

    private static Card mapCard(ResultSet rs) throws SQLException {
        return new Card(
            nullToEmpty(rs.getString("code")),
            nullToEmpty(rs.getString("machine_id")),
            nullToEmpty(rs.getString("agent_id")),
            readInstantOrEpoch(rs, "created_at"),
            readInstant(rs, "activated_at"),
            readInstantOrEpoch(rs, "expires_at"),
            nullToEmpty(rs.getString("status")),
            nullToEmpty(rs.getString("note")),
            rs.getInt("max_sessions")
        );
    }

    private static Instant readInstantOrEpoch(ResultSet rs, String column) throws SQLException {
        Instant value = readInstant(rs, column);
        return value == null ? Instant.EPOCH : value;
    }

    /**
     * SQLite stores DATETIME as text, and the driver only parses its own
     * "yyyy-MM-dd HH:mm:ss.SSS" shape, so RFC3339 values need a manual fallback.
     */
    private static Instant readInstant(ResultSet rs, String column) throws SQLException {
        try {
            Timestamp timestamp = rs.getTimestamp(column);
            if (timestamp != null) {
                return timestamp.toInstant();
            }
        } catch (SQLException ignored) {
            // Fall through to string parsing below.
        }
        String raw = rs.getString(column);
        if (raw == null || raw.isBlank()) {
            return null;
        }
        try {
            return Instant.parse(raw);
        } catch (DateTimeParseException ex) {
            return null;
        }
    }

    private static boolean hasText(String value) {
        return value != null && !value.isBlank();
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }
}
