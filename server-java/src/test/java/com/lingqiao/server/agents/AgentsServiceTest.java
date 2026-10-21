package com.lingqiao.server.agents;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.sql.DriverManager;
import java.time.Instant;
import java.util.List;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class AgentsServiceTest {
    private static final Instant NOW = Instant.parse("2026-07-01T00:00:00Z");

    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private AgentsService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(
            DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE agents (
              id TEXT PRIMARY KEY, username TEXT NOT NULL, password TEXT NOT NULL,
              prefix TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL,
              disabled INTEGER NOT NULL DEFAULT 0
            )
            """);
        jdbc.execute("""
            CREATE TABLE cards (
              code TEXT PRIMARY KEY, machine_id TEXT NOT NULL DEFAULT '',
              agent_id TEXT NOT NULL DEFAULT '', created_at DATETIME NOT NULL,
              activated_at DATETIME, expires_at DATETIME NOT NULL,
              status TEXT NOT NULL DEFAULT 'active', note TEXT NOT NULL DEFAULT '',
              max_sessions INTEGER NOT NULL DEFAULT 1
            )
            """);
        service = new AgentsService(jdbc);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void listReadsAgentsWithZeroCounters() {
        insertAgent("agent-1", "ops", "OPS", false);

        List<Agent> agents = service.list();

        assertThat(agents).hasSize(1);
        Agent agent = agents.get(0);
        assertThat(agent.id()).isEqualTo("agent-1");
        assertThat(agent.username()).isEqualTo("ops");
        assertThat(agent.prefix()).isEqualTo("OPS");
        assertThat(agent.disabled()).isFalse();
        assertThat(agent.totalCards()).isZero();
        assertThat(agent.boundMachines()).isZero();
        assertThat(agent.lastCardCreatedAt()).isNull();
    }

    @Test
    void getThrowsForUnknownAgent() {
        assertThatThrownBy(() -> service.get("missing"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("agent not found");
    }

    @Test
    void getRejectsBlankId() {
        assertThatThrownBy(() -> service.get(" "))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("agent id is required");
    }

    @Test
    void summarizeCountsActiveExpiredAndDistinctMachines() {
        List<AgentsService.AgentAccount> accounts = List.of(account("a1", "alice"));
        List<AgentsService.CardSnapshot> cards = List.of(
            card("a1", "M1", "active", NOW.minusSeconds(100), NOW.plusSeconds(3600)),
            card("a1", "M1", "active", NOW.minusSeconds(50), NOW.plusSeconds(3600)),
            card("a1", "M2", "expired", NOW.minusSeconds(10), NOW.plusSeconds(3600)),
            card("a1", "", "active", NOW.minusSeconds(5), NOW.minusSeconds(1)));

        Agent agent = AgentsService.summarize(accounts, cards, NOW).get(0);

        assertThat(agent.totalCards()).isEqualTo(4);
        // The 4th card is "active" but already expired, so it counts as expired only.
        assertThat(agent.activeCards()).isEqualTo(2);
        assertThat(agent.expiredCards()).isEqualTo(2);
        assertThat(agent.boundMachines()).isEqualTo(2);
        assertThat(agent.lastCardCreatedAt()).isEqualTo(NOW.minusSeconds(5));
    }

    @Test
    void summarizeIgnoresCardsOfUnknownAgents() {
        List<AgentsService.AgentAccount> accounts = List.of(account("a1", "alice"));
        List<AgentsService.CardSnapshot> cards = List.of(
            card("ghost", "M9", "active", NOW, NOW.plusSeconds(60)));

        Agent agent = AgentsService.summarize(accounts, cards, NOW).get(0);

        assertThat(agent.totalCards()).isZero();
        assertThat(agent.boundMachines()).isZero();
    }

    @Test
    void summarizeOrdersByCardCountThenUsername() {
        List<AgentsService.AgentAccount> accounts = List.of(
            account("a1", "zoe"), account("a2", "adam"), account("a3", "busy"));
        List<AgentsService.CardSnapshot> cards = List.of(
            card("a3", "M1", "active", NOW, NOW.plusSeconds(60)),
            card("a3", "M2", "active", NOW, NOW.plusSeconds(60)));

        List<Agent> agents = AgentsService.summarize(accounts, cards, NOW);

        assertThat(agents).extracting(Agent::username).containsExactly("busy", "adam", "zoe");
    }

    @Test
    void listAggregatesCardsFromDatabase() {
        // list() uses the real clock, so expiry must be relative to now.
        Instant future = Instant.now().plusSeconds(3600);
        insertAgent("agent-1", "ops", "", false);
        insertCard("C1", "agent-1", "M1", "active", future);
        insertCard("C2", "agent-1", "M1", "active", future);
        insertCard("C3", "agent-1", "", "expired", future);

        Agent agent = service.list().get(0);

        assertThat(agent.totalCards()).isEqualTo(3);
        assertThat(agent.activeCards()).isEqualTo(2);
        assertThat(agent.boundMachines()).isEqualTo(1);
        assertThat(agent.expiredCards()).isEqualTo(1);
    }

    @Test
    void disabledFlagIsReadFromInteger() {
        insertAgent("agent-1", "ops", "", true);

        assertThat(service.list().get(0).disabled()).isTrue();
    }

    private void insertAgent(String id, String username, String prefix, boolean disabled) {
        jdbc.update("""
            INSERT INTO agents (id, username, password, prefix, created_at, disabled)
            VALUES (?, ?, 'hash', ?, ?, ?)
            """, id, username, prefix, NOW.toString(), disabled ? 1 : 0);
    }

    private void insertCard(String code, String agentId, String machineId, String status, Instant expiresAt) {
        jdbc.update("""
            INSERT INTO cards (code, machine_id, agent_id, created_at, expires_at, status)
            VALUES (?, ?, ?, ?, ?, ?)
            """, code, machineId, agentId, NOW.toString(), expiresAt.toString(), status);
    }

    private static AgentsService.AgentAccount account(String id, String username) {
        return new AgentsService.AgentAccount(id, username, "", NOW, false);
    }

    private static AgentsService.CardSnapshot card(
        String agentId, String machineId, String status, Instant createdAt, Instant expiresAt) {
        return new AgentsService.CardSnapshot(agentId, machineId, status, createdAt, expiresAt);
    }
}
