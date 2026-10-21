package com.lingqiao.server.releases;

import static org.assertj.core.api.Assertions.assertThat;

import java.sql.DriverManager;
import java.util.List;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class ReleasesServiceAggregationTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private ReleasesService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(
            DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE releases (
              id TEXT PRIMARY KEY, version TEXT NOT NULL, channel TEXT NOT NULL,
              status TEXT NOT NULL, min_version TEXT NOT NULL DEFAULT '',
              force_update INTEGER NOT NULL DEFAULT 0, rollout_percent INTEGER NOT NULL DEFAULT 0,
              notes TEXT NOT NULL DEFAULT '', targeting_json TEXT NOT NULL DEFAULT '',
              script_id TEXT NOT NULL DEFAULT '', script_version TEXT NOT NULL DEFAULT '',
              script_sha256 TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL,
              published_at TEXT, paused_at TEXT, rolled_back_at TEXT,
              rolled_back_to TEXT NOT NULL DEFAULT ''
            )
            """);
        jdbc.execute("""
            CREATE TABLE release_packages (
              id TEXT PRIMARY KEY, release_id TEXT NOT NULL, kind TEXT NOT NULL,
              filename TEXT NOT NULL, path TEXT NOT NULL, file_size INTEGER NOT NULL,
              sha256 TEXT NOT NULL, created_at TEXT NOT NULL
            )
            """);
        jdbc.execute("""
            CREATE TABLE release_events (
              id INTEGER PRIMARY KEY AUTOINCREMENT, release_id TEXT NOT NULL,
              version TEXT NOT NULL, machine_id TEXT NOT NULL,
              card_code TEXT NOT NULL DEFAULT '', agent_id TEXT NOT NULL DEFAULT '',
              type TEXT NOT NULL, error_code TEXT NOT NULL DEFAULT '',
              detail TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL
            )
            """);
        service = new ReleasesService(jdbc);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void listPackagesReturnsNewestFirst() {
        insertRelease("rel-1");
        insertPackage("pkg-old", "rel-1", "2026-01-01T00:00:00Z");
        insertPackage("pkg-new", "rel-1", "2026-06-01T00:00:00Z");
        insertPackage("pkg-other", "rel-2", "2026-07-01T00:00:00Z");

        List<ReleasePackage> packages = service.listPackages("rel-1");

        assertThat(packages).extracting(ReleasePackage::id)
            .containsExactly("pkg-new", "pkg-old");
    }

    @Test
    void listPackagesIsEmptyForUnknownRelease() {
        assertThat(service.listPackages("missing")).isEmpty();
    }

    @Test
    void metricsCountsEveryGoEventType() {
        insertRelease("rel-1");
        insertEvent("rel-1", "offered", "M1");
        insertEvent("rel-1", "offered", "M2");
        insertEvent("rel-1", "download_started", "M1");
        insertEvent("rel-1", "download_failed", "M1");
        insertEvent("rel-1", "download_completed", "M2");
        insertEvent("rel-1", "install_started", "M2");
        insertEvent("rel-1", "install_success", "M2");
        insertEvent("rel-1", "install_failed", "M1");
        insertEvent("rel-1", "rollback", "M1");
        insertEvent("rel-1", "dismissed", "M3");

        ReleaseMetrics metrics = service.metrics("rel-1");

        assertThat(metrics.releaseId()).isEqualTo("rel-1");
        assertThat(metrics.offered()).isEqualTo(2);
        assertThat(metrics.downloadStarted()).isEqualTo(1);
        assertThat(metrics.downloadFailed()).isEqualTo(1);
        assertThat(metrics.downloadCompleted()).isEqualTo(1);
        assertThat(metrics.installStarted()).isEqualTo(1);
        assertThat(metrics.installSuccess()).isEqualTo(1);
        assertThat(metrics.installFailed()).isEqualTo(1);
        assertThat(metrics.rollback()).isEqualTo(1);
        assertThat(metrics.dismissed()).isEqualTo(1);
        assertThat(metrics.uniqueMachines()).isEqualTo(3);
    }

    @Test
    void metricsExcludesBlankMachineIdsFromUniqueCount() {
        insertRelease("rel-1");
        insertEvent("rel-1", "offered", "M1");
        insertEvent("rel-1", "offered", "");
        insertEvent("rel-1", "offered", "");

        ReleaseMetrics metrics = service.metrics("rel-1");

        assertThat(metrics.offered()).isEqualTo(3);
        assertThat(metrics.uniqueMachines()).isEqualTo(1);
    }

    @Test
    void metricsIgnoresOtherReleases() {
        insertRelease("rel-1");
        insertEvent("rel-1", "offered", "M1");
        insertEvent("rel-2", "offered", "M9");

        assertThat(service.metrics("rel-1").offered()).isEqualTo(1);
        assertThat(service.metrics("rel-1").uniqueMachines()).isEqualTo(1);
    }

    @Test
    void metricsForReleaseWithoutEventsIsAllZero() {
        insertRelease("rel-1");

        ReleaseMetrics metrics = service.metrics("rel-1");

        assertThat(metrics).isEqualTo(ReleaseMetrics.empty("rel-1"));
    }

    @Test
    void unknownEventTypesDoNotBreakAggregation() {
        insertRelease("rel-1");
        insertEvent("rel-1", "offered", "M1");
        insertEvent("rel-1", "legacy_type", "M1");

        ReleaseMetrics metrics = service.metrics("rel-1");

        assertThat(metrics.offered()).isEqualTo(1);
        assertThat(metrics.uniqueMachines()).isEqualTo(1);
    }

    private void insertRelease(String id) {
        jdbc.update("""
            INSERT INTO releases (id, version, channel, status, created_at)
            VALUES (?, '1.0.0', 'stable', 'published', '2026-01-01T00:00:00Z')
            """, id);
    }

    private void insertPackage(String id, String releaseId, String createdAt) {
        jdbc.update("""
            INSERT INTO release_packages (id, release_id, kind, filename, path, file_size, sha256, created_at)
            VALUES (?, ?, 'full', 'app.zip', 'releases/app.zip', 1024, 'abc', ?)
            """, id, releaseId, createdAt);
    }

    private void insertEvent(String releaseId, String type, String machineId) {
        jdbc.update("""
            INSERT INTO release_events (release_id, version, machine_id, type, created_at)
            VALUES (?, '1.0.0', ?, ?, '2026-01-01T00:00:00Z')
            """, releaseId, machineId, type);
    }
}
