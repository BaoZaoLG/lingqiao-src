package com.lingqiao.server.releases;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.nio.file.Files;
import java.nio.file.Path;
import java.sql.DriverManager;
import java.time.Instant;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;
import org.springframework.mock.web.MockMultipartFile;

class ReleaseAdminServiceTest {
    @TempDir
    Path dataDir;

    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private ReleaseAdminService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        createSchema();
        ReleasesService reads = new ReleasesService(jdbc);
        service = new ReleaseAdminService(reads, new ReleasesWriteService(jdbc), jdbc, dataDir);
    }

    @AfterEach
    void tearDown() {
        dataSource.destroy();
    }

    @Test
    void createUsesGoDefaultsAndActiveScript() throws Exception {
        writeActiveScript();

        Release release = service.create(new ReleaseAdminService.CreateRelease(
            "2.5.0", "", "", false, 0, "", TargetingRules.empty()));

        assertThat(release.id()).startsWith("rel-stable-2-5-0-");
        assertThat(release.channel()).isEqualTo("stable");
        assertThat(release.status()).isEqualTo("draft");
        assertThat(release.rolloutPercent()).isEqualTo(100);
        assertThat(release.scriptId()).isEqualTo("script-1");
        assertThat(jdbc.queryForObject(
            "SELECT action FROM audit_log", String.class)).isEqualTo("release_created");
    }

    @Test
    void uploadAndPublishPersistPackageHashAndLifecycle() throws Exception {
        writeActiveScript();
        Release release = service.create(new ReleaseAdminService.CreateRelease(
            "2.5.0", "beta", "", true, 25, "notes", TargetingRules.empty()));
        MockMultipartFile file = new MockMultipartFile(
            "file", "installer.EXE", "application/octet-stream", "package-body".getBytes());

        ReleasePackage pkg = service.uploadPackage(release.id(), "", file);
        Release published = service.publish(release.id());

        assertThat(pkg.kind()).isEqualTo("bundle");
        assertThat(pkg.filename()).isEqualTo("LingqiaoSetup-2.5.0.exe");
        assertThat(pkg.sha256()).hasSize(64);
        assertThat(Files.readString(dataDir.resolve(pkg.path()))).isEqualTo("package-body");
        assertThat(published.status()).isEqualTo("published");
        assertThat(published.publishedAt()).isNotNull();
        assertThat(jdbc.queryForObject(
            "SELECT COUNT(*) FROM audit_log", Integer.class)).isEqualTo(3);
    }

    @Test
    void publishRequiresUploadedPackageAndExistingFile() throws Exception {
        writeActiveScript();
        Release release = service.create(new ReleaseAdminService.CreateRelease(
            "1.0.0", "", "", false, 100, "", TargetingRules.empty()));

        assertThatThrownBy(() -> service.publish(release.id()))
            .hasMessage("Cannot publish release: no update package uploaded");
    }

    @Test
    void createRequiresActiveScript() {
        assertThatThrownBy(() -> service.create(new ReleaseAdminService.CreateRelease(
            "1.0.0", "", "", false, 100, "", TargetingRules.empty())))
            .hasMessage("active script is required before creating release");
    }

    @Test
    void currentScriptIndexTakesPrecedenceOverLegacyActiveFile() throws Exception {
        writeActiveScript();
        Files.writeString(dataDir.resolve("scripts/index.json"),
            "{\"active_id\":\"current\",\"scripts\":[{\"id\":\"current\",\"version\":\"2.0.0\",\"sha256\":\"new\"}]}");

        Release release = service.create(new ReleaseAdminService.CreateRelease(
            "3.0.0", "", "", false, 100, "", TargetingRules.empty()));

        assertThat(release.scriptId()).isEqualTo("current");
        assertThat(release.scriptVersion()).isEqualTo("2.0.0");
        assertThat(release.scriptSha256()).isEqualTo("new");
    }

    private void writeActiveScript() throws Exception {
        Files.createDirectories(dataDir.resolve("scripts"));
        Files.writeString(dataDir.resolve("scripts/active.json"),
            "{\"id\":\"script-1\",\"version\":\"1.2.3\",\"sha256\":\"abc\",\"content\":\"ignored\"}");
    }

    private void createSchema() {
        jdbc.execute("""
            CREATE TABLE releases (
              id TEXT PRIMARY KEY, version TEXT NOT NULL, channel TEXT NOT NULL,
              status TEXT NOT NULL, min_version TEXT NOT NULL DEFAULT '',
              force_update INTEGER NOT NULL DEFAULT 0, rollout_percent INTEGER NOT NULL DEFAULT 100,
              notes TEXT NOT NULL DEFAULT '', targeting_json TEXT NOT NULL DEFAULT '{}',
              script_id TEXT NOT NULL DEFAULT '', script_version TEXT NOT NULL DEFAULT '',
              script_sha256 TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL,
              published_at TEXT NOT NULL DEFAULT '', paused_at TEXT NOT NULL DEFAULT '',
              rolled_back_at TEXT NOT NULL DEFAULT '', rolled_back_to TEXT NOT NULL DEFAULT ''
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
              version TEXT NOT NULL, machine_id TEXT NOT NULL, card_code TEXT NOT NULL DEFAULT '',
              agent_id TEXT NOT NULL DEFAULT '', type TEXT NOT NULL,
              error_code TEXT NOT NULL DEFAULT '', detail TEXT NOT NULL DEFAULT '',
              created_at TEXT NOT NULL
            )
            """);
        jdbc.execute("""
            CREATE TABLE audit_log (
              time DATETIME NOT NULL, action TEXT NOT NULL, card TEXT, machine TEXT,
              agent_id TEXT, detail TEXT, addr TEXT, actor_type TEXT, actor_id TEXT,
              resource_type TEXT, resource_id TEXT, before TEXT, after TEXT,
              user_agent TEXT, request_id TEXT
            )
            """);
    }
}
