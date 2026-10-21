package com.lingqiao.server.releases;

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

class ReleaseEventRecorderTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private ReleaseEventRecorder recorder;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE release_events (
              id INTEGER PRIMARY KEY AUTOINCREMENT,
              release_id TEXT NOT NULL,
              version TEXT NOT NULL,
              machine_id TEXT NOT NULL,
              card_code TEXT NOT NULL DEFAULT '',
              agent_id TEXT NOT NULL DEFAULT '',
              type TEXT NOT NULL,
              error_code TEXT NOT NULL DEFAULT '',
              detail TEXT NOT NULL DEFAULT '',
              created_at TEXT NOT NULL
            )
            """);
        recorder = new ReleaseEventRecorder(jdbc);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void recordPersistsAllColumnsAndNormalizesNulls() {
        recorder.record(new ReleaseEventRecorder.ReleaseEvent(
            "rel-1", "2.4.0", "M1", null, null,
            ReleaseEventRecorder.INSTALL_SUCCESS, null, null, Instant.parse("2026-07-01T00:00:00Z")));

        List<Map<String, Object>> rows = jdbc.queryForList("SELECT * FROM release_events");
        assertThat(rows).hasSize(1);
        Map<String, Object> row = rows.get(0);
        assertThat(row.get("release_id")).isEqualTo("rel-1");
        assertThat(row.get("type")).isEqualTo(ReleaseEventRecorder.INSTALL_SUCCESS);
        assertThat(row.get("card_code")).isEqualTo("");
        assertThat(row.get("agent_id")).isEqualTo("");
        assertThat(row.get("error_code")).isEqualTo("");
        assertThat(row.get("created_at")).isEqualTo("2026-07-01T00:00:00Z");
    }

    @Test
    void missingTimestampDefaultsToNow() {
        recorder.record(new ReleaseEventRecorder.ReleaseEvent(
            "rel-2", "1.0.0", "M2", "", "", ReleaseEventRecorder.OFFERED, "", "", null));

        String createdAt = jdbc.queryForObject(
            "SELECT created_at FROM release_events WHERE release_id='rel-2'", String.class);
        assertThat(createdAt).endsWith("Z");
    }

    @Test
    void missingFieldsAreRejectedButCustomTypesMatchGoStorage() {
        assertThatThrownBy(() -> recorder.record(new ReleaseEventRecorder.ReleaseEvent(
            "", "1.0.0", "M", "", "", ReleaseEventRecorder.OFFERED, "", "", Instant.now())))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("release id and event type are required");
        recorder.record(new ReleaseEventRecorder.ReleaseEvent(
            "rel-3", "1.0.0", "M", "", "", "bogus", "", "", Instant.now()));
        assertThat(jdbc.queryForObject("SELECT type FROM release_events", String.class))
            .isEqualTo("bogus");
    }

    @Test
    void recordQuietlySwallowsFailuresLikeGoOfferPath() {
        recorder.recordQuietly(new ReleaseEventRecorder.ReleaseEvent(
            "", "", "", "", "", "bogus", "", "", Instant.now()));

        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM release_events", Integer.class)).isZero();
    }

    @Test
    void validTypeSetMatchesGoEventTypes() {
        assertThat(ReleaseEventRecorder.isValidType("offered")).isTrue();
        assertThat(ReleaseEventRecorder.isValidType("download_started")).isTrue();
        assertThat(ReleaseEventRecorder.isValidType("download_failed")).isTrue();
        assertThat(ReleaseEventRecorder.isValidType("download_completed")).isTrue();
        assertThat(ReleaseEventRecorder.isValidType("install_started")).isTrue();
        assertThat(ReleaseEventRecorder.isValidType("install_success")).isTrue();
        assertThat(ReleaseEventRecorder.isValidType("install_failed")).isTrue();
        assertThat(ReleaseEventRecorder.isValidType("rollback")).isTrue();
        assertThat(ReleaseEventRecorder.isValidType("dismissed")).isTrue();
        assertThat(ReleaseEventRecorder.isValidType("offered ")).isFalse();
        assertThat(ReleaseEventRecorder.isValidType("installed")).isFalse();
        assertThat(ReleaseEventRecorder.isValidType("rolled_back")).isFalse();
        assertThat(ReleaseEventRecorder.isValidType(null)).isFalse();
    }

    @Test
    void everyGoEventTypeIsAccepted() {
        List<String> goEventTypes = List.of(
            "offered", "download_started", "download_failed", "download_completed",
            "install_started", "install_success", "install_failed", "rollback", "dismissed");
        for (String type : goEventTypes) {
            recorder.record(new ReleaseEventRecorder.ReleaseEvent(
                "rel-all", "1.0.0", "M", "", "", type, "", "", Instant.now()));
        }
        assertThat(jdbc.queryForObject(
            "SELECT COUNT(DISTINCT type) FROM release_events WHERE release_id='rel-all'", Integer.class))
            .isEqualTo(goEventTypes.size());
    }
}
