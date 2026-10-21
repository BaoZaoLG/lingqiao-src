package com.lingqiao.server.admin;

import com.lingqiao.server.common.jdbc.TimestampWriter;
import static org.assertj.core.api.Assertions.assertThat;

import com.lingqiao.server.cards.Card;
import com.lingqiao.server.cards.CardStatuses;
import java.sql.DriverManager;
import java.time.Clock;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneId;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class AdminOverviewServiceTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private AdminOverviewService service;
    private Instant now;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE cards (
              code TEXT PRIMARY KEY, machine_id TEXT NOT NULL DEFAULT '',
              agent_id TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL,
              activated_at DATETIME, expires_at DATETIME NOT NULL,
              status TEXT NOT NULL DEFAULT 'active', note TEXT NOT NULL DEFAULT '',
              max_sessions INTEGER NOT NULL DEFAULT 1
            )
            """);
        jdbc.execute("""
            CREATE TABLE sessions (
              token TEXT PRIMARY KEY, card_code TEXT NOT NULL, machine_id TEXT NOT NULL,
              expires_at DATETIME NOT NULL
            )
            """);
        jdbc.execute("""
            CREATE TABLE audit_log (
              id INTEGER PRIMARY KEY AUTOINCREMENT, time DATETIME NOT NULL, action TEXT NOT NULL,
              card TEXT NOT NULL DEFAULT '', machine TEXT NOT NULL DEFAULT '',
              agent_id TEXT NOT NULL DEFAULT '', detail TEXT NOT NULL DEFAULT '',
              addr TEXT NOT NULL DEFAULT '', actor_type TEXT NOT NULL DEFAULT '',
              actor_id TEXT NOT NULL DEFAULT '', resource_type TEXT NOT NULL DEFAULT '',
              resource_id TEXT NOT NULL DEFAULT '', before TEXT NOT NULL DEFAULT '',
              after TEXT NOT NULL DEFAULT '', user_agent TEXT NOT NULL DEFAULT '',
              request_id TEXT NOT NULL DEFAULT ''
            )
            """);
        jdbc.execute("""
            CREATE TABLE blacklist (
              value TEXT PRIMARY KEY, type TEXT NOT NULL DEFAULT '',
              reason TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL
            )
            """);
        jdbc.execute("""
            CREATE TABLE agents (
              id TEXT PRIMARY KEY, username TEXT NOT NULL
            )
            """);
        now = Instant.parse("2026-07-15T04:00:00Z");
        Clock clock = Clock.fixed(now, ZoneId.of("Asia/Shanghai"));
        service = new AdminOverviewService(jdbc, now.minus(Duration.ofMinutes(30)), clock);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void dashboardCountsActiveAndExpiredCards() {
        insertCard("C-ACTIVE", "M1", CardStatuses.ACTIVE, now.plus(Duration.ofDays(3)));
        insertCard("C-EXPIRED-TIME", "M2", CardStatuses.ACTIVE, now.minus(Duration.ofDays(1)));
        insertCard("C-EXPIRED-STATUS", "M3", CardStatuses.EXPIRED, now.plus(Duration.ofDays(5)));
        insertCard("C-DISABLED", "M4", CardStatuses.DISABLED, now.plus(Duration.ofDays(5)));
        insertSession("s-live", "C-ACTIVE", now.plus(Duration.ofHours(1)));
        insertSession("s-dead", "C-ACTIVE", now.minus(Duration.ofHours(1)));

        AdminOverviewService.DashboardStats stats = service.dashboard();

        assertThat(stats.totalCards()).isEqualTo(4);
        assertThat(stats.activeCards()).isEqualTo(1);
        assertThat(stats.expiredCards()).isEqualTo(2);
        assertThat(stats.activeSessions()).isEqualTo(1);
    }

    @Test
    void serverStatsReportUptimeAndCounters() {
        insertCard("C1", "M1", CardStatuses.ACTIVE, now.plus(Duration.ofDays(1)));

        AdminOverviewService.ServerStats stats = service.serverStats(42);

        assertThat(stats.uptimeSeconds()).isGreaterThanOrEqualTo(1799);
        assertThat(stats.totalCards()).isEqualTo(1);
        assertThat(stats.requestCount()).isEqualTo(42);
    }

    @Test
    void opsOverviewAggregatesActivationsExpiringAndRiskyMachines() {
        insertCard("C-EXPIRING-1", "M1", CardStatuses.ACTIVE, now.plus(Duration.ofDays(2)));
        insertCard("C-EXPIRING-2", "M1", CardStatuses.ACTIVE, now.plus(Duration.ofDays(6)));
        insertCard("C-FAR", "M9", CardStatuses.ACTIVE, now.plus(Duration.ofDays(30)));
        insertAudit("card_activated", "M1", now.minus(Duration.ofMinutes(5)));
        insertAudit("card_activated", "M2", now.minus(Duration.ofMinutes(4)));
        insertAudit("card_activated", "M1", now.minus(Duration.ofMinutes(3)));
        insertAudit("card_created", "M3", now.minus(Duration.ofMinutes(2)));

        AdminOverviewService.OpsOverview ops = service.opsOverview();

        assertThat(ops.todayActivations()).isEqualTo(3);
        assertThat(ops.newMachines()).isEqualTo(2);
        assertThat(ops.expiringCards()).extracting(Card::code)
            .containsExactly("C-EXPIRING-1", "C-EXPIRING-2");
        assertThat(ops.riskyMachines()).hasSize(1);
        assertThat(ops.riskyMachines().get(0).machineId()).isEqualTo("M1");
        assertThat(ops.riskyMachines().get(0).cardCount()).isEqualTo(2);
        assertThat(ops.recentAudit()).hasSize(4);
        assertThat(ops.recentAudit()).allSatisfy(event -> {
            assertThat(event).containsKeys("time", "action");
            assertThat(event).doesNotContainKeys(
                "actor_type", "actor_id", "resource_type", "resource_id",
                "before", "after", "user_agent", "request_id", "card", "agent_id", "detail");
        });
    }

    @Test
    void expiringCardsAreSortedAndCapped() {
        for (int i = 1; i <= 12; i++) {
            insertCard("C-" + String.format("%02d", i), "M" + i, CardStatuses.ACTIVE,
                now.plus(Duration.ofHours(i)));
        }

        AdminOverviewService.OpsOverview ops = service.opsOverview();

        assertThat(ops.expiringCards()).hasSize(AdminOverviewService.EXPIRING_LIMIT);
        assertThat(ops.expiringCards().get(0).code()).isEqualTo("C-01");
        assertThat(ops.expiringCards().get(9).code()).isEqualTo("C-10");
    }

    @Test
    void recentAuditIsLimitedToEightNewestEntries() {
        for (int i = 0; i < 15; i++) {
            insertAudit("card_created", "M" + i, now.minus(Duration.ofMinutes(i)));
        }

        AdminOverviewService.OpsOverview ops = service.opsOverview();

        assertThat(ops.recentAudit()).hasSize(AdminOverviewService.RECENT_AUDIT_LIMIT);
    }

    @Test
    void opsOverviewIncludesBlacklistedMachinesAndAgentLeaderboard() {
        jdbc.update("INSERT INTO agents (id, username) VALUES ('a1', 'alice')");
        insertCard("C-A1-1", "M1", "a1", CardStatuses.ACTIVE, now.plus(Duration.ofDays(1)));
        insertCard("C-A1-2", "M1", "a1", CardStatuses.EXPIRED, now.minus(Duration.ofDays(1)));
        jdbc.update("""
            INSERT INTO blacklist (value, type, reason, created_at)
            VALUES ('M-BLOCKED', 'machine', '', ?)
            """, TimestampWriter.format(now));

        AdminOverviewService.OpsOverview ops = service.opsOverview();

        assertThat(ops.riskyMachines())
            .anyMatch(machine -> machine.machineId().equals("M-BLOCKED") && machine.blacklisted());
        assertThat(ops.agentLeaderboard()).singleElement().satisfies(item -> {
            assertThat(item.agentId()).isEqualTo("a1");
            assertThat(item.username()).isEqualTo("alice");
            assertThat(item.totalCards()).isEqualTo(2);
            assertThat(item.activeCards()).isEqualTo(1);
            assertThat(item.expiredCards()).isEqualTo(1);
        });
    }

    @Test
    void modulesOverviewMatchesGoModuleKeysAndMetrics() {
        insertCard("C-MODULE", "", CardStatuses.ACTIVE, now.plusSeconds(3600));
        jdbc.update("INSERT INTO agents (id, username) VALUES ('a-module', 'module-agent')");
        insertAudit("card_created", "", now);
        insertSession("s-module", "C-MODULE", now.plusSeconds(3600));

        AdminOverviewService.ModulesOverview result = service.modulesOverview();

        assertThat(result.generatedAt()).isNotNull();
        assertThat(result.modules()).extracting(AdminOverviewService.ModuleOverviewItem::key)
            .containsExactly("cards", "agents", "audit");
        assertThat(result.modules().get(0).metrics())
            .containsEntry("total_cards", 1)
            .containsEntry("active_sessions", 1);
        assertThat(result.modules().get(1).metrics()).containsEntry("total_agents", 1);
        assertThat(result.modules().get(2).metrics()).containsEntry("total_events", 1);
    }

    private void insertCard(String code, String machineId, String status, Instant expiresAt) {
        insertCard(code, machineId, "", status, expiresAt);
    }

    private void insertCard(
        String code,
        String machineId,
        String agentId,
        String status,
        Instant expiresAt
    ) {
        jdbc.update("""
            INSERT INTO cards (
              code, machine_id, agent_id, created_at, activated_at, expires_at, status, max_sessions
            ) VALUES (?,?,?,?,?,?,?,1)
            """, code, machineId, agentId, TimestampWriter.format(now.minus(Duration.ofDays(1))),
            TimestampWriter.format(now.minus(Duration.ofHours(2))), TimestampWriter.format(expiresAt), status);
    }

    private void insertSession(String token, String cardCode, Instant expiresAt) {
        jdbc.update("INSERT INTO sessions (token, card_code, machine_id, expires_at) VALUES (?,?,?,?)",
            token, cardCode, "M1", TimestampWriter.format(expiresAt));
    }

    private void insertAudit(String action, String machine, Instant time) {
        jdbc.update("INSERT INTO audit_log (time, action, machine) VALUES (?,?,?)",
            TimestampWriter.format(time), action, machine);
    }
}
