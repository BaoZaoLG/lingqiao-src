package com.lingqiao.server.releases;

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

class ReleasesWriteServiceTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private ReleasesWriteService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE releases (
              id TEXT PRIMARY KEY,
              version TEXT NOT NULL,
              channel TEXT NOT NULL,
              status TEXT NOT NULL,
              min_version TEXT NOT NULL DEFAULT '',
              force_update INTEGER NOT NULL DEFAULT 0,
              rollout_percent INTEGER NOT NULL DEFAULT 0,
              notes TEXT NOT NULL DEFAULT '',
              targeting_json TEXT NOT NULL DEFAULT '{}',
              script_id TEXT NOT NULL DEFAULT '',
              script_version TEXT NOT NULL DEFAULT '',
              script_sha256 TEXT NOT NULL DEFAULT '',
              created_at TEXT NOT NULL,
              published_at TEXT NOT NULL DEFAULT '',
              paused_at TEXT NOT NULL DEFAULT '',
              rolled_back_at TEXT NOT NULL DEFAULT '',
              rolled_back_to TEXT NOT NULL DEFAULT ''
            )
            """);
        service = new ReleasesWriteService(jdbc);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void saveAppliesGoDefaultsAndUpsertsById() {
        service.save(input("rel-1", "1.0.0", null, null, 500));

        assertThat(jdbc.queryForObject("SELECT channel FROM releases WHERE id='rel-1'", String.class))
            .isEqualTo("stable");
        assertThat(jdbc.queryForObject("SELECT status FROM releases WHERE id='rel-1'", String.class))
            .isEqualTo(ReleaseStatuses.DRAFT);
        assertThat(jdbc.queryForObject("SELECT rollout_percent FROM releases WHERE id='rel-1'", Integer.class))
            .isEqualTo(100);

        service.save(input("rel-1", "1.1.0", "beta", ReleaseStatuses.PUBLISHED, -5));

        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM releases", Integer.class)).isEqualTo(1);
        assertThat(jdbc.queryForObject("SELECT version FROM releases WHERE id='rel-1'", String.class))
            .isEqualTo("1.1.0");
        assertThat(jdbc.queryForObject("SELECT rollout_percent FROM releases WHERE id='rel-1'", Integer.class))
            .isZero();
    }

    @Test
    void publishClearsPauseAndRollbackMarkers() {
        service.save(input("rel-2", "2.0.0", "stable", ReleaseStatuses.DRAFT, 10));
        service.pause("rel-2");
        service.rollback("rel-2", "rel-1");

        assertThat(jdbc.queryForObject("SELECT status FROM releases WHERE id='rel-2'", String.class))
            .isEqualTo(ReleaseStatuses.ROLLED_BACK);
        assertThat(jdbc.queryForObject("SELECT rolled_back_to FROM releases WHERE id='rel-2'", String.class))
            .isEqualTo("rel-1");

        service.publish("rel-2");

        assertThat(jdbc.queryForObject("SELECT status FROM releases WHERE id='rel-2'", String.class))
            .isEqualTo(ReleaseStatuses.PUBLISHED);
        assertThat(jdbc.queryForObject("SELECT published_at FROM releases WHERE id='rel-2'", String.class))
            .endsWith("Z");
        assertThat(jdbc.queryForObject("SELECT paused_at FROM releases WHERE id='rel-2'", String.class)).isEmpty();
        assertThat(jdbc.queryForObject("SELECT rolled_back_at FROM releases WHERE id='rel-2'", String.class)).isEmpty();
        assertThat(jdbc.queryForObject("SELECT rolled_back_to FROM releases WHERE id='rel-2'", String.class)).isEmpty();
    }

    @Test
    void rolloutPercentIsClampedAndPersisted() {
        service.save(input("rel-3", "3.0.0", "stable", ReleaseStatuses.PUBLISHED, 20));

        service.setRolloutPercent("rel-3", 250);
        assertThat(jdbc.queryForObject("SELECT rollout_percent FROM releases WHERE id='rel-3'", Integer.class))
            .isEqualTo(100);

        service.setRolloutPercent("rel-3", -1);
        assertThat(jdbc.queryForObject("SELECT rollout_percent FROM releases WHERE id='rel-3'", Integer.class))
            .isZero();
    }

    @Test
    void targetingRulesArePersistedAsJson() {
        service.save(new ReleaseWriteInput(
            "rel-4", "4.0.0", "stable", ReleaseStatuses.DRAFT, "", false, 0, "",
            new TargetingRules(List.of("CARD-1"), List.of(), List.of("M1"), List.of(), List.of(), List.of()),
            "", "", "", Instant.parse("2026-07-01T00:00:00Z"), null, null, null, ""));

        String json = jdbc.queryForObject("SELECT targeting_json FROM releases WHERE id='rel-4'", String.class);
        assertThat(json).contains("CARD-1").contains("M1").contains("allow_machines");
    }

    @Test
    void invalidInputsAreRejected() {
        assertThatThrownBy(() -> service.save(input("", "1.0.0", null, null, 0)))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("release id and version are required");
        assertThatThrownBy(() -> service.save(input("rel-x", "1.0.0", null, "bogus", 0)))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid release status");
        assertThatThrownBy(() -> service.publish("missing"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("release not found");
        assertThatThrownBy(() -> service.pause(" "))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("release id is required");
    }

    private static ReleaseWriteInput input(String id, String version, String channel, String status, int rollout) {
        return new ReleaseWriteInput(
            id, version, channel, status, "1.0.0", true, rollout, "notes",
            TargetingRules.empty(), "script-1", "1.0.0", "sha", Instant.parse("2026-07-01T00:00:00Z"),
            null, null, null, "");
    }
}
