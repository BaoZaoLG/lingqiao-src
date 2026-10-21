package com.lingqiao.server.agents;

import com.lingqiao.server.common.jdbc.TimestampReader;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.time.Instant;
import java.time.format.DateTimeParseException;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;

/**
 * Admin agent listing mirroring Go agents.AdminHandler + buildAdminAgentSummaries:
 * per-agent card counts, distinct bound machines, latest card timestamp, and the
 * same ordering (most cards first, then username ascending).
 */
public class AgentsService {
    private static final RowMapper<AgentAccount> ACCOUNT_MAPPER = (rs, rowNum) -> mapAccount(rs);
    private static final RowMapper<CardSnapshot> CARD_MAPPER = (rs, rowNum) -> mapCard(rs);

    private final JdbcTemplate jdbc;

    public AgentsService(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    public List<Agent> list() {
        return summarize(accounts(), cards(), Instant.now());
    }

    public Agent get(String id) {
        if (id == null || id.isBlank()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "agent id is required");
        }
        return list().stream()
            .filter(agent -> agent.id().equals(id))
            .findFirst()
            .orElseThrow(() -> new SidecarException(ErrorCode.AGENT_NOT_FOUND, "agent not found"));
    }

    List<AgentAccount> accounts() {
        return jdbc.query("""
            SELECT id, username, prefix, created_at, disabled FROM agents ORDER BY created_at DESC
            """, ACCOUNT_MAPPER);
    }

    List<CardSnapshot> cards() {
        return jdbc.query("""
            SELECT agent_id, machine_id, status, created_at, expires_at FROM cards
            """, CARD_MAPPER);
    }

    /** Pure aggregation, extracted so it can be tested without a database. */
    static List<Agent> summarize(List<AgentAccount> accounts, List<CardSnapshot> cards, Instant now) {
        Map<String, Counters> counters = new HashMap<>();
        for (AgentAccount account : accounts) {
            counters.put(account.id(), new Counters());
        }

        for (CardSnapshot card : cards) {
            Counters counter = counters.get(card.agentId());
            if (counter == null) {
                continue;
            }
            counter.totalCards++;
            if ("active".equals(card.status()) && card.expiresAt() != null && card.expiresAt().isAfter(now)) {
                counter.activeCards++;
            }
            if ("expired".equals(card.status())
                || (card.expiresAt() != null && card.expiresAt().isBefore(now))) {
                counter.expiredCards++;
            }
            if (card.machineId() != null && !card.machineId().isEmpty()) {
                counter.machines.add(card.machineId());
            }
            if (card.createdAt() != null
                && (counter.lastCardCreatedAt == null || card.createdAt().isAfter(counter.lastCardCreatedAt))) {
                counter.lastCardCreatedAt = card.createdAt();
            }
        }

        List<Agent> result = new ArrayList<>(accounts.size());
        for (AgentAccount account : accounts) {
            Counters counter = counters.get(account.id());
            result.add(new Agent(
                account.id(), account.username(), account.prefix(), account.createdAt(),
                account.disabled(), counter.totalCards, counter.activeCards, counter.expiredCards,
                counter.machines.size(), counter.lastCardCreatedAt));
        }

        result.sort(Comparator
            .comparingInt(Agent::totalCards).reversed()
            .thenComparing(Agent::username, Comparator.nullsFirst(Comparator.naturalOrder())));
        return result;
    }

    private static AgentAccount mapAccount(ResultSet rs) throws SQLException {
        return new AgentAccount(
            nullToEmpty(rs.getString("id")),
            nullToEmpty(rs.getString("username")),
            nullToEmpty(rs.getString("prefix")),
            readInstant(rs, "created_at"),
            rs.getInt("disabled") != 0);
    }

    private static CardSnapshot mapCard(ResultSet rs) throws SQLException {
        return new CardSnapshot(
            nullToEmpty(rs.getString("agent_id")),
            nullToEmpty(rs.getString("machine_id")),
            nullToEmpty(rs.getString("status")),
            readInstant(rs, "created_at"),
            readInstant(rs, "expires_at"));
    }

    private static Instant readInstant(ResultSet rs, String column) throws SQLException {
        try {
            java.sql.Timestamp timestamp = rs.getTimestamp(column);
            if (timestamp != null) {
                return timestamp.toInstant();
            }
        } catch (SQLException ignored) {
            // SQLite may store text timestamps; fall through to string parsing.
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

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }

    record AgentAccount(String id, String username, String prefix, Instant createdAt, boolean disabled) {
    }

    record CardSnapshot(String agentId, String machineId, String status, Instant createdAt, Instant expiresAt) {
    }

    private static final class Counters {
        private int totalCards;
        private int activeCards;
        private int expiredCards;
        private Instant lastCardCreatedAt;
        private final Set<String> machines = new HashSet<>();
    }
}
