package com.lingqiao.server.releases;

import static org.assertj.core.api.Assertions.assertThat;

import java.sql.DriverManager;
import java.time.Instant;
import java.util.List;
import java.util.Optional;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;

class ReleaseUpdateServiceTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private ReleaseUpdateService service;
    private ReleaseSelectionStore store;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE releases (
              id TEXT PRIMARY KEY, version TEXT NOT NULL, channel TEXT NOT NULL, status TEXT NOT NULL,
              min_version TEXT NOT NULL DEFAULT '', force_update INTEGER NOT NULL DEFAULT 0,
              rollout_percent INTEGER NOT NULL DEFAULT 0, notes TEXT NOT NULL DEFAULT '',
              targeting_json TEXT NOT NULL DEFAULT '{}', script_id TEXT NOT NULL DEFAULT '',
              script_version TEXT NOT NULL DEFAULT '', script_sha256 TEXT NOT NULL DEFAULT '',
              created_at TEXT NOT NULL, published_at TEXT NOT NULL DEFAULT '',
              paused_at TEXT NOT NULL DEFAULT '', rolled_back_at TEXT NOT NULL DEFAULT '',
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
        store = new ReleaseSelectionStore(jdbc);
        service = new ReleaseUpdateService(store);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void selectsNewestEligiblePublishedRelease() {
        insertRelease("rel-old", "2.3.0", "stable", ReleaseStatuses.PUBLISHED, 100, "{}",
            "2026-07-01T00:00:00Z", "2026-07-01T00:00:00Z");
        insertRelease("rel-new", "2.5.0", "stable", ReleaseStatuses.PUBLISHED, 100, "{}",
            "2026-07-02T00:00:00Z", "2026-07-02T00:00:00Z");
        insertPackage("pkg-new", "rel-new", 2048, "sha-new");
        insertPackage("pkg-old", "rel-old", 1024, "sha-old");

        Optional<ReleaseUpdateService.SelectedRelease> selected = service.select(client("2.4.0", "stable", "M1"));

        assertThat(selected).isPresent();
        assertThat(selected.get().release().id()).isEqualTo("rel-new");
        assertThat(selected.get().pkg().id()).isEqualTo("pkg-new");
    }

    @Test
    void skipsOlderVersionsDraftsAndOtherChannels() {
        insertRelease("rel-draft", "9.0.0", "stable", ReleaseStatuses.DRAFT, 100, "{}",
            "2026-07-03T00:00:00Z", "");
        insertRelease("rel-beta", "9.1.0", "beta", ReleaseStatuses.PUBLISHED, 100, "{}",
            "2026-07-03T00:00:00Z", "2026-07-03T00:00:00Z");
        insertRelease("rel-same", "2.4.0", "stable", ReleaseStatuses.PUBLISHED, 100, "{}",
            "2026-07-03T00:00:00Z", "2026-07-03T00:00:00Z");
        insertPackage("pkg-draft", "rel-draft", 1, "a");
        insertPackage("pkg-beta", "rel-beta", 1, "b");
        insertPackage("pkg-same", "rel-same", 1, "c");

        assertThat(service.select(client("2.4.0", "stable", "M1"))).isEmpty();
        assertThat(service.select(client("2.4.0", "beta", "M1"))).isPresent();
    }

    @Test
    void releaseWithoutPackagesIsSkipped() {
        insertRelease("rel-nopkg", "3.0.0", "stable", ReleaseStatuses.PUBLISHED, 100, "{}",
            "2026-07-04T00:00:00Z", "2026-07-04T00:00:00Z");

        assertThat(service.select(client("2.4.0", "stable", "M1"))).isEmpty();
    }

    @Test
    void rolloutAndTargetingFiltersAreApplied() {
        insertRelease("rel-zero", "3.0.0", "stable", ReleaseStatuses.PUBLISHED, 0, "{}",
            "2026-07-05T00:00:00Z", "2026-07-05T00:00:00Z");
        insertPackage("pkg-zero", "rel-zero", 1, "a");
        assertThat(service.select(client("2.4.0", "stable", "M1"))).isEmpty();

        jdbc.update("UPDATE releases SET rollout_percent = 100, targeting_json = ? WHERE id = 'rel-zero'",
            "{\"deny_machines\":[\"M1\"]}");
        assertThat(service.select(client("2.4.0", "stable", "M1"))).isEmpty();
        assertThat(service.select(client("2.4.0", "stable", "M2"))).isPresent();
    }

    @Test
    void manifestMirrorsGoFieldMapping() {
        insertRelease("rel-m", "2.9.0", "stable", ReleaseStatuses.PUBLISHED, 100, "{}",
            "2026-07-06T00:00:00Z", "2026-07-06T00:00:00Z");
        jdbc.update("""
            UPDATE releases SET min_version = '2.0.0', force_update = 1, notes = 'notes',
                script_id = 's1', script_version = '1.2.3', script_sha256 = 'ssha' WHERE id = 'rel-m'
            """);
        insertPackage("pkg-m", "rel-m", 4096, "pkgsha");

        ReleaseUpdateService.SelectedRelease selected = service.select(client("2.4.0", "stable", "M1")).orElseThrow();
        UpdateManifest manifest = service.toManifest(selected);

        assertThat(manifest.releaseId()).isEqualTo("rel-m");
        assertThat(manifest.packageUrl()).isEqualTo("/api/v1/update/package/pkg-m");
        assertThat(manifest.packageSize()).isEqualTo(4096);
        assertThat(manifest.packageSha256()).isEqualTo("pkgsha");
        assertThat(manifest.minVersion()).isEqualTo("2.0.0");
        assertThat(manifest.forceUpdate()).isTrue();
        assertThat(manifest.scriptVersion()).isEqualTo("1.2.3");
        assertThat(manifest.createdAt()).isEqualTo(Instant.parse("2026-07-06T00:00:00Z"));
    }

    @Test
    void packageLookupSupportsDownloadFlow() {
        insertRelease("rel-dl", "3.1.0", "stable", ReleaseStatuses.PUBLISHED, 100, "{}",
            "2026-07-07T00:00:00Z", "2026-07-07T00:00:00Z");
        insertPackage("pkg-dl", "rel-dl", 10, "dlsha");

        assertThat(store.findPackage("pkg-dl")).isPresent();
        assertThat(store.requirePackage("pkg-dl").filename()).isEqualTo("pkg-dl.zip");
        assertThat(store.findPackage("missing")).isEmpty();
        assertThat(store.listPackages("rel-dl")).hasSize(1);
    }

    private static ReleaseSelector.ClientContext client(String version, String channel, String machineId) {
        return new ReleaseSelector.ClientContext(version, channel, "", machineId, "");
    }

    private void insertRelease(String id, String version, String channel, String status,
                               int rollout, String targeting, String createdAt, String publishedAt) {
        jdbc.update("""
            INSERT INTO releases (id, version, channel, status, rollout_percent, targeting_json,
                                  created_at, published_at)
            VALUES (?,?,?,?,?,?,?,?)
            """, id, version, channel, status, rollout, targeting, createdAt, publishedAt);
    }

    private void insertPackage(String id, String releaseId, long size, String sha256) {
        jdbc.update("""
            INSERT INTO release_packages (id, release_id, kind, filename, path, file_size, sha256, created_at)
            VALUES (?,?,?,?,?,?,?,?)
            """, id, releaseId, "bundle", id + ".zip", "packages/" + id + ".zip", size, sha256,
            "2026-07-01T00:00:00Z");
    }

    @Test
    void listPublishedReturnsOnlyMatchingChannel() {
        insertRelease("rel-s", "1.0.0", "stable", ReleaseStatuses.PUBLISHED, 100, "{}",
            "2026-07-08T00:00:00Z", "2026-07-08T00:00:00Z");
        insertRelease("rel-b", "1.0.0", "beta", ReleaseStatuses.PUBLISHED, 100, "{}",
            "2026-07-08T00:00:00Z", "2026-07-08T00:00:00Z");

        List<Release> stable = store.listPublished("stable");
        assertThat(stable).extracting(Release::id).containsExactly("rel-s");
        assertThat(store.listPublished("")).extracting(Release::id).containsExactly("rel-s");
    }
}
