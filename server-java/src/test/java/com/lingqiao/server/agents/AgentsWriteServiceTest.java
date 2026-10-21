package com.lingqiao.server.agents;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.sql.DriverManager;
import java.sql.Timestamp;
import java.time.Instant;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;
import org.springframework.security.crypto.bcrypt.BCrypt;

class AgentsWriteServiceTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private AgentsWriteService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE agents (
              id TEXT PRIMARY KEY,
              username TEXT NOT NULL,
              password TEXT NOT NULL,
              prefix TEXT NOT NULL DEFAULT '',
              created_at DATETIME NOT NULL,
              disabled INTEGER NOT NULL DEFAULT 0,
              max_session_duration_minutes INTEGER NOT NULL DEFAULT 0,
              max_daily_sessions INTEGER NOT NULL DEFAULT 0,
              max_concurrent_sessions INTEGER NOT NULL DEFAULT 0
            )
            """);
        jdbc.update("INSERT INTO agents (id, username, password, created_at) VALUES (?, ?, ?, ?)",
            "agent-1", "ops", BCrypt.hashpw("original", BCrypt.gensalt()), Timestamp.from(Instant.now()));
        service = new AgentsWriteService(jdbc);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void statusToggleMirrorsGoDisabledFlag() {
        service.setStatus("agent-1", true);
        assertThat(jdbc.queryForObject("SELECT disabled FROM agents WHERE id = 'agent-1'", Integer.class)).isEqualTo(1);

        service.setStatus("agent-1", false);
        assertThat(jdbc.queryForObject("SELECT disabled FROM agents WHERE id = 'agent-1'", Integer.class)).isZero();
    }

    @Test
    void limitsArePersistedAndValidated() {
        service.setLimits("agent-1", 120, 5, 2);

        assertThat(jdbc.queryForObject(
            "SELECT max_session_duration_minutes FROM agents WHERE id = 'agent-1'", Integer.class)).isEqualTo(120);
        assertThat(jdbc.queryForObject(
            "SELECT max_daily_sessions FROM agents WHERE id = 'agent-1'", Integer.class)).isEqualTo(5);
        assertThat(jdbc.queryForObject(
            "SELECT max_concurrent_sessions FROM agents WHERE id = 'agent-1'", Integer.class)).isEqualTo(2);

        assertThatThrownBy(() -> service.setLimits("agent-1", -1, 0, 0))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("must not be negative");
    }

    @Test
    void passwordResetStoresBcryptHashAndEnforcesLength() {
        service.forceResetPassword("agent-1", "newSecret");

        String stored = jdbc.queryForObject("SELECT password FROM agents WHERE id = 'agent-1'", String.class);
        assertThat(stored).startsWith("$2");
        assertThat(BCrypt.checkpw("newSecret", stored)).isTrue();

        assertThatThrownBy(() -> service.forceResetPassword("agent-1", "12345"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("6-72");
        assertThatThrownBy(() -> service.forceResetPassword("agent-1", "x".repeat(73)))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("6-72");
    }

    @Test
    void deleteAndMissingAgentsBehaveLikeGo() {
        service.delete("agent-1");
        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM agents", Integer.class)).isZero();

        assertThatThrownBy(() -> service.delete("agent-1"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("agent not found");
        assertThatThrownBy(() -> service.setStatus(" ", true))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("agent_id is required");
    }
}
