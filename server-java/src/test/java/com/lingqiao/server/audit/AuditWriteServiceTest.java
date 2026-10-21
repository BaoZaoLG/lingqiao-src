package com.lingqiao.server.audit;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.sql.DriverManager;
import java.time.Instant;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class AuditWriteServiceTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private AuditWriteService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE audit_log (
              id INTEGER PRIMARY KEY AUTOINCREMENT,
              time DATETIME NOT NULL,
              action TEXT NOT NULL,
              card TEXT NOT NULL DEFAULT '',
              machine TEXT NOT NULL DEFAULT '',
              agent_id TEXT NOT NULL DEFAULT '',
              detail TEXT NOT NULL DEFAULT '',
              addr TEXT NOT NULL DEFAULT '',
              actor_type TEXT NOT NULL DEFAULT '',
              actor_id TEXT NOT NULL DEFAULT '',
              resource_type TEXT NOT NULL DEFAULT '',
              resource_id TEXT NOT NULL DEFAULT '',
              before TEXT NOT NULL DEFAULT '',
              after TEXT NOT NULL DEFAULT '',
              user_agent TEXT NOT NULL DEFAULT '',
              request_id TEXT NOT NULL DEFAULT ''
            )
            """);
        service = new AuditWriteService(jdbc);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void appendPersistsAllColumnsLikeGo() {
        service.append(new AuditEvent(
            Instant.parse("2026-07-01T00:00:00Z"), "card_created", "admin", "admin-1",
            "card", "CARD-1", "{}", "{\"status\":\"active\"}", "agent/1.0", "req-1",
            "CARD-1", "agent-1", "M1", "127.0.0.1", "created via admin"));

        List<Map<String, Object>> rows = jdbc.queryForList("SELECT * FROM audit_log");
        assertThat(rows).hasSize(1);
        Map<String, Object> row = rows.get(0);
        assertThat(row.get("action")).isEqualTo("card_created");
        assertThat(row.get("actor_type")).isEqualTo("admin");
        assertThat(row.get("resource_id")).isEqualTo("CARD-1");
        assertThat(row.get("addr")).isEqualTo("127.0.0.1");
        assertThat(row.get("request_id")).isEqualTo("req-1");
        assertThat(row.get("detail")).isEqualTo("created via admin");
    }

    @Test
    void missingTimestampDefaultsToNow() {
        service.append(new AuditEvent(
            (Instant) null, "login", null, null, null, null, null, null, null, null,
            null, null, null, null, null));

        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM audit_log", Integer.class)).isEqualTo(1);
        assertThat(jdbc.queryForObject("SELECT card FROM audit_log", String.class)).isEmpty();
    }

    @Test
    void blankActionIsRejected() {
        assertThatThrownBy(() -> service.append(new AuditEvent(
            Instant.now(), "  ", null, null, null, null, null, null, null, null,
            null, null, null, null, null)))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("action is required");
        assertThatThrownBy(() -> service.append(null))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("event is required");
    }
}
