package com.lingqiao.server.agentportal;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.jdbc.TimestampWriter;
import java.sql.DriverManager;
import java.time.Instant;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class AgentPortalCardServiceTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private AgentPortalCardService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(
            DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE agents (
              id TEXT PRIMARY KEY,
              disabled INTEGER NOT NULL DEFAULT 0,
              max_session_duration_minutes INTEGER NOT NULL DEFAULT 0,
              max_daily_sessions INTEGER NOT NULL DEFAULT 0,
              max_concurrent_sessions INTEGER NOT NULL DEFAULT 0
            )
            """);
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
        service = new AgentPortalCardService(jdbc);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void generatedCardsAreOwnedAndListedByAgent() {
        insertAgent("agent-1", 0, 0, 0, 0);
        insertAgent("agent-2", 0, 0, 0, 0);

        var generated = service.generate("agent-1", 24, "single", 2);
        var batch = service.batchGenerate("agent-1", 2, 48, "batch", 1);
        service.generate("agent-2", 24, "other", 1);

        assertThat(generated.agentId()).isEqualTo("agent-1");
        assertThat(batch).hasSize(2).allMatch(card -> card.agentId().equals("agent-1"));
        assertThat(service.list("agent-1"))
            .hasSize(3)
            .allMatch(card -> card.agentId().equals("agent-1"));
    }

    @Test
    void listPreservesStoredStatusLikeGoAgentCards() {
        insertAgent("agent-status", 0, 0, 0, 0);
        jdbc.update("""
            INSERT INTO cards (
              code, machine_id, agent_id, created_at, activated_at, expires_at,
              status, note, max_sessions
            ) VALUES ('AAAAAA-BBBBBB-CCCCCC', '', ?, ?, NULL, ?, 'active', '', 1)
            """,
            "agent-status",
            TimestampWriter.format(Instant.now().minusSeconds(7200)),
            TimestampWriter.format(Instant.now().minusSeconds(3600))
        );

        assertThat(service.list("agent-status"))
            .singleElement()
            .extracting(AgentPortalCardService.AgentCard::status)
            .isEqualTo("active");
    }

    @Test
    void enforcesDurationConcurrentAndDailyLimits() {
        insertAgent("duration", 0, 60, 0, 0);
        insertAgent("concurrent", 0, 0, 0, 1);
        insertAgent("daily", 0, 0, 2, 0);

        assertThatThrownBy(() -> service.generate("duration", 2, "", 1))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("duration exceeds maximum");
        assertThatThrownBy(() -> service.generate("concurrent", 1, "", 2))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("sessions per card");

        service.generate("daily", 1, "", 1);
        assertThatThrownBy(() -> service.batchGenerate("daily", 2, 1, "", 1))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("daily sessions limit");
    }

    @Test
    void rejectsDisabledAndUnknownAgentsBeforeWriting() {
        insertAgent("disabled", 1, 0, 0, 0);

        assertThatThrownBy(() -> service.generate("disabled", 1, "", 1))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("agent is disabled");
        assertThatThrownBy(() -> service.generate("missing", 1, "", 1))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("agent not found");
        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM cards", Integer.class)).isZero();
    }

    private void insertAgent(
        String id,
        int disabled,
        int maxDurationMinutes,
        int maxDailySessions,
        int maxConcurrentSessions
    ) {
        jdbc.update("""
            INSERT INTO agents (
              id, disabled, max_session_duration_minutes, max_daily_sessions,
              max_concurrent_sessions
            ) VALUES (?, ?, ?, ?, ?)
            """, id, disabled, maxDurationMinutes, maxDailySessions, maxConcurrentSessions);
    }
}
