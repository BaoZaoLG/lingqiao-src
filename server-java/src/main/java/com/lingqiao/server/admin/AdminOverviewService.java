package com.lingqiao.server.admin;

import com.fasterxml.jackson.annotation.JsonProperty;
import com.lingqiao.server.common.jdbc.TimestampWriter;
import com.lingqiao.server.common.jdbc.TimestampReader;
import com.lingqiao.server.cards.Card;
import com.lingqiao.server.cards.CardStatuses;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.LocalDate;
import java.util.ArrayList;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import org.springframework.dao.DataAccessException;
import org.springframework.jdbc.core.JdbcTemplate;

/**
 * Admin dashboard aggregates mirroring Go admin_overview.go:
 * HandleDashboard, HandleServerStats and HandleOpsOverview.
 */
public class AdminOverviewService {
    static final Duration EXPIRING_WINDOW = Duration.ofDays(7);
    static final int EXPIRING_LIMIT = 10;
    static final int RECENT_AUDIT_LIMIT = 8;

    private final JdbcTemplate jdbc;
    private final Instant serverStart;
    private final Clock clock;

    public AdminOverviewService(JdbcTemplate jdbc, Instant serverStart) {
        this(jdbc, serverStart, Clock.systemDefaultZone());
    }

    AdminOverviewService(JdbcTemplate jdbc, Instant serverStart, Clock clock) {
        this.jdbc = jdbc;
        this.clock = clock;
        this.serverStart = serverStart == null ? clock.instant() : serverStart;
    }

    public DashboardStats dashboard() {
        Instant now = clock.instant();
        List<Card> cards = allCards();
        int active = 0;
        int expired = 0;
        for (Card card : cards) {
            if (CardStatuses.ACTIVE.equals(card.status()) && now.isBefore(card.expiresAt())) {
                active++;
            }
            if (now.isAfter(card.expiresAt()) || CardStatuses.EXPIRED.equals(card.status())) {
                expired++;
            }
        }
        return new DashboardStats(cards.size(), active, expired, activeSessionCount(now));
    }

    public ServerStats serverStats(long requestCount) {
        Instant now = clock.instant();
        return new ServerStats(
            Duration.between(serverStart, now).toSeconds(),
            allCards().size(),
            activeSessionCount(now),
            requestCount);
    }

    public OpsOverview opsOverview() {
        Instant now = clock.instant();
        List<Card> cards = allCards();
        Instant dayStart = LocalDate.now(clock)
            .atStartOfDay(clock.getZone())
            .toInstant();

        int todayActivations = jdbc.queryForObject("""
            SELECT COUNT(*) FROM audit_log WHERE action = 'card_activated' AND time > ?
            """, Integer.class, TimestampWriter.format(dayStart));
        int newMachines = jdbc.queryForObject("""
            SELECT COUNT(DISTINCT machine) FROM audit_log
            WHERE action = 'card_activated' AND time > ? AND machine != ''
            """, Integer.class, TimestampWriter.format(dayStart));

        List<Map<String, Object>> recentAudit = jdbc.query("""
            SELECT time, action, actor_type, actor_id, resource_type, resource_id,
                   before, after, user_agent, request_id, card, agent_id, machine, addr, detail
            FROM audit_log ORDER BY time DESC LIMIT ?
            """, (rs, rowNum) -> {
            Map<String, Object> event = new java.util.LinkedHashMap<>();
            event.put("time", TimestampReader.readOrEpoch(rs, "time"));
            event.put("action", nullToEmpty(rs.getString("action")));
            putIfNotEmpty(event, "actor_type", rs.getString("actor_type"));
            putIfNotEmpty(event, "actor_id", rs.getString("actor_id"));
            putIfNotEmpty(event, "resource_type", rs.getString("resource_type"));
            putIfNotEmpty(event, "resource_id", rs.getString("resource_id"));
            putIfNotEmpty(event, "before", rs.getString("before"));
            putIfNotEmpty(event, "after", rs.getString("after"));
            putIfNotEmpty(event, "user_agent", rs.getString("user_agent"));
            putIfNotEmpty(event, "request_id", rs.getString("request_id"));
            putIfNotEmpty(event, "card", rs.getString("card"));
            putIfNotEmpty(event, "agent_id", rs.getString("agent_id"));
            putIfNotEmpty(event, "machine", rs.getString("machine"));
            putIfNotEmpty(event, "ip", rs.getString("addr"));
            putIfNotEmpty(event, "detail", rs.getString("detail"));
            return event;
        }, RECENT_AUDIT_LIMIT);

        List<Card> expiring = new ArrayList<>();
        Instant expiringBefore = now.plus(EXPIRING_WINDOW);
        for (Card card : cards) {
            if (CardStatuses.ACTIVE.equals(card.status())
                && card.expiresAt().isAfter(now)
                && card.expiresAt().isBefore(expiringBefore)) {
                expiring.add(card);
            }
        }
        expiring.sort(Comparator.comparing(Card::expiresAt));
        if (expiring.size() > EXPIRING_LIMIT) {
            expiring = new ArrayList<>(expiring.subList(0, EXPIRING_LIMIT));
        }

        List<RiskyMachine> riskyMachines = jdbc.query("""
            SELECT machine_id, COUNT(code) AS card_count
            FROM cards
            WHERE machine_id != ''
            GROUP BY machine_id
            HAVING card_count > 1
            """, (rs, rowNum) -> new RiskyMachine(rs.getString("machine_id"), rs.getInt("card_count")));

        Set<String> blacklistedMachines = new HashSet<>(jdbc.queryForList(
            "SELECT value FROM blacklist WHERE type = 'machine'", String.class));
        Set<String> seenMachines = new HashSet<>();
        List<RiskyMachine> decoratedMachines = new ArrayList<>();
        for (RiskyMachine machine : riskyMachines) {
            decoratedMachines.add(new RiskyMachine(
                machine.machineId(), machine.cardCount(),
                blacklistedMachines.contains(machine.machineId())));
            seenMachines.add(machine.machineId());
        }
        for (String machineId : blacklistedMachines) {
            if (!seenMachines.contains(machineId)) {
                decoratedMachines.add(new RiskyMachine(machineId, 0, true));
            }
        }
        decoratedMachines.sort(Comparator
            .comparing(RiskyMachine::blacklisted).reversed()
            .thenComparing(Comparator.comparingInt(RiskyMachine::cardCount).reversed()));
        if (decoratedMachines.size() > 10) {
            decoratedMachines = new ArrayList<>(decoratedMachines.subList(0, 10));
        }

        List<AgentLeaderboardItem> leaderboard = buildAgentLeaderboard(
            cards,
            jdbc.queryForList("SELECT id, username FROM agents"),
            now
        );
        return new OpsOverview(todayActivations, newMachines, activeSessionCount(now),
            expiring, decoratedMachines, leaderboard, recentAudit);
    }

    public ModulesOverview modulesOverview() {
        CountResult cards = count("SELECT COUNT(*) FROM cards");
        CountResult agents = count("SELECT COUNT(*) FROM agents");
        CountResult audit = count("SELECT COUNT(*) FROM audit_log");
        CountResult sessions = count(
            "SELECT COUNT(*) FROM sessions WHERE expires_at > ?",
            TimestampWriter.format(clock.instant())
        );

        List<ModuleOverviewItem> modules = List.of(
            new ModuleOverviewItem(
                "cards", "卡密授权", cards.ok() ? "ready" : "error",
                cards.ok() ? cards.value() + " total cards" : "failed to load cards",
                Map.of(
                    "total_cards", cards.value(),
                    "active_sessions", sessions.ok() ? sessions.value() : 0
                )
            ),
            new ModuleOverviewItem(
                "agents", "代理渠道", agents.ok() ? "ready" : "error",
                agents.ok() ? agents.value() + " agents" : "failed to load agents",
                Map.of("total_agents", agents.value())
            ),
            new ModuleOverviewItem(
                "audit", "审计日志", audit.ok() ? "ready" : "error",
                audit.ok() ? audit.value() + " retained events" : "failed to load audit events",
                Map.of("total_events", audit.value())
            )
        );
        return new ModulesOverview(clock.instant(), modules);
    }

    private CountResult count(String sql, Object... args) {
        try {
            Integer value = jdbc.queryForObject(sql, Integer.class, args);
            return new CountResult(true, value == null ? 0 : value);
        } catch (DataAccessException ex) {
            return new CountResult(false, 0);
        }
    }

    private int activeSessionCount(Instant now) {
        try {
            Integer count = jdbc.queryForObject(
                "SELECT COUNT(*) FROM sessions WHERE expires_at > ?",
                Integer.class, TimestampWriter.format(now));
            return count == null ? 0 : count;
        } catch (DataAccessException ex) {
            return 0;
        }
    }

    private List<Card> allCards() {
        return jdbc.query("""
            SELECT code, machine_id, agent_id, created_at, activated_at, expires_at, status, note, max_sessions
            FROM cards
            """, (rs, rowNum) -> new Card(
            nullToEmpty(rs.getString("code")),
            nullToEmpty(rs.getString("machine_id")),
            nullToEmpty(rs.getString("agent_id")),
            TimestampReader.readOrEpoch(rs, "created_at"),
            TimestampReader.read(rs, "activated_at"),
            TimestampReader.readOrEpoch(rs, "expires_at"),
            nullToEmpty(rs.getString("status")),
            nullToEmpty(rs.getString("note")),
            rs.getInt("max_sessions")));
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }

    private static void putIfNotEmpty(Map<String, Object> target, String key, String value) {
        if (value != null && !value.isEmpty()) {
            target.put(key, value);
        }
    }

    public record DashboardStats(int totalCards, int activeCards, int expiredCards, int activeSessions) {
    }

    public record ServerStats(long uptimeSeconds, int totalCards, int activeSessions, long requestCount) {
    }

    static List<AgentLeaderboardItem> buildAgentLeaderboard(
        List<Card> cards,
        List<Map<String, Object>> agents,
        Instant now
    ) {
        Map<String, String> names = new HashMap<>();
        for (Map<String, Object> agent : agents) {
            names.put(String.valueOf(agent.get("id")), String.valueOf(agent.get("username")));
        }
        Map<String, int[]> counts = new HashMap<>();
        for (Card card : cards) {
            if (card.agentId() == null || card.agentId().isEmpty()) {
                continue;
            }
            int[] values = counts.computeIfAbsent(card.agentId(), ignored -> new int[3]);
            values[0]++;
            if (CardStatuses.ACTIVE.equals(card.status()) && card.expiresAt().isAfter(now)) {
                values[1]++;
            }
            if (CardStatuses.EXPIRED.equals(card.status()) || card.expiresAt().isBefore(now)) {
                values[2]++;
            }
        }
        List<AgentLeaderboardItem> result = new ArrayList<>();
        for (Map.Entry<String, int[]> entry : counts.entrySet()) {
            String agentId = entry.getKey();
            int[] values = entry.getValue();
            result.add(new AgentLeaderboardItem(
                agentId, names.getOrDefault(agentId, agentId), values[0], values[1], values[2]));
        }
        result.sort(Comparator.comparingInt(AgentLeaderboardItem::totalCards).reversed());
        return result.size() > 10 ? new ArrayList<>(result.subList(0, 10)) : result;
    }

    public record RiskyMachine(
        @JsonProperty("machine_id") String machineId,
        @JsonProperty("card_count") int cardCount,
        @JsonProperty("is_blacklisted") boolean blacklisted
    ) {
        RiskyMachine(String machineId, int cardCount) {
            this(machineId, cardCount, false);
        }
    }

    public record AgentLeaderboardItem(
        @JsonProperty("agent_id") String agentId,
        String username,
        @JsonProperty("total_cards") int totalCards,
        @JsonProperty("active_cards") int activeCards,
        @JsonProperty("expired_cards") int expiredCards
    ) {
    }

    public record OpsOverview(
        int todayActivations,
        int newMachines,
        int activeSessions,
        List<Card> expiringCards,
        List<RiskyMachine> riskyMachines,
        List<AgentLeaderboardItem> agentLeaderboard,
        List<Map<String, Object>> recentAudit
    ) {
    }

    public record ModuleOverviewItem(
        String key,
        String name,
        String status,
        String summary,
        Map<String, Object> metrics
    ) {
    }

    public record ModulesOverview(Instant generatedAt, List<ModuleOverviewItem> modules) {
    }

    private record CountResult(boolean ok, int value) {
    }
}
