package com.lingqiao.server.agents;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.lingqiao.server.cards.Card;
import com.lingqiao.server.common.errors.SidecarException;
import java.sql.DriverManager;
import java.util.List;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class AgentResourcesServiceTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private AgentResourcesService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(
            DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE cards (
              code TEXT PRIMARY KEY,
              machine_id TEXT NOT NULL DEFAULT '',
              agent_id TEXT NOT NULL DEFAULT '',
              created_at DATETIME NOT NULL,
              activated_at DATETIME,
              expires_at DATETIME NOT NULL,
              status TEXT NOT NULL DEFAULT 'active',
              note TEXT NOT NULL DEFAULT '',
              max_sessions INTEGER NOT NULL DEFAULT 1
            )
            """);
        jdbc.execute("""
            CREATE TABLE sessions (
              token TEXT PRIMARY KEY,
              card_code TEXT NOT NULL,
              machine_id TEXT NOT NULL,
              last_seen DATETIME NOT NULL
            )
            """);
        jdbc.execute("""
            CREATE TABLE blacklist (
              value TEXT PRIMARY KEY,
              type TEXT NOT NULL DEFAULT '',
              reason TEXT NOT NULL DEFAULT '',
              created_at DATETIME NOT NULL
            )
            """);
        service = new AgentResourcesService(jdbc);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void machinesMirrorGoAggregationAndBlacklistAnnotation() {
        insertCard("A", "M1", "agent-1", "2025-01-01T00:00:00Z", "active", "");
        insertCard("B", "M1", "agent-1", "2025-01-02T00:00:00Z", "active", "");
        jdbc.update(
            "INSERT INTO sessions(token, card_code, machine_id, last_seen) VALUES ('T1','A','M1',?)",
            "2025-01-03T00:00:00Z");
        jdbc.update(
            "INSERT INTO sessions(token, card_code, machine_id, last_seen) VALUES ('T2','B','M1',?)",
            "2025-01-04T00:00:00Z");
        jdbc.update("""
            INSERT INTO blacklist(value, type, reason, created_at)
            VALUES ('M1', 'machine', 'risk', '2025-01-01T00:00:00Z')
            """);

        List<AgentResourcesService.MachineInfo> machines = service.machines();

        assertThat(machines).hasSize(1);
        AgentResourcesService.MachineInfo machine = machines.get(0);
        // Go's current LEFT JOIN counts card/session combinations.
        assertThat(machine.cardCount()).isEqualTo(4);
        assertThat(machine.lastSeen()).hasToString("2025-01-04T00:00:00Z");
        assertThat(machine.blacklisted()).isTrue();
    }

    @Test
    void machineCardsKeepStoredStatusAndGoOrdering() {
        insertCard("A", "M1", "agent-1", "2025-01-01T00:00:00Z", "disabled", "old");
        insertCard("B", "M1", "agent-1", "2025-01-02T00:00:00Z", "active", "new");

        List<Card> cards = service.machineCards("M1");

        assertThat(cards).extracting(Card::code).containsExactly("B", "A");
        assertThat(cards).extracting(Card::status).containsExactly("active", "disabled");
    }

    @Test
    void agentCardsUseSnapshotOmitEmptyShapeAndReturnEmptyOnReadFailure() throws Exception {
        insertCard("A", "", "agent-1", "2025-01-01T00:00:00Z", "active", "");

        AgentResourcesService.AgentCard card = service.agentCards("agent-1").get(0);
        ObjectMapper mapper = new ObjectMapper().registerModule(new JavaTimeModule());
        String json = mapper.writeValueAsString(card);

        assertThat(json).contains("\"agent_id\":\"agent-1\"");
        assertThat(json).doesNotContain("machine_id", "activated_at", "\"note\"");

        jdbc.execute("DROP TABLE cards");
        assertThat(service.agentCards("agent-1")).isEmpty();
    }

    @Test
    void detailRoutesRequireTheirIdentifiers() {
        assertThatThrownBy(() -> service.machineCards(""))
            .isInstanceOf(SidecarException.class)
            .hasMessage("machine id is required");
        assertThatThrownBy(() -> service.agentCards(""))
            .isInstanceOf(SidecarException.class)
            .hasMessage("agent_id is required");
    }

    private void insertCard(
        String code,
        String machineId,
        String agentId,
        String createdAt,
        String status,
        String note
    ) {
        jdbc.update("""
            INSERT INTO cards (
              code, machine_id, agent_id, created_at, activated_at,
              expires_at, status, note, max_sessions
            ) VALUES (?, ?, ?, ?, NULL, '2030-01-01T00:00:00Z', ?, ?, 1)
            """, code, machineId, agentId, createdAt, status, note);
    }
}
