package com.lingqiao.server.releases;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.lingqiao.server.audit.AuditEvent;
import com.lingqiao.server.audit.AuditWriteService;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.jdbc.TimestampWriter;
import java.io.InputStream;
import java.io.OutputStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardOpenOption;
import java.security.MessageDigest;
import java.time.Instant;
import java.util.HexFormat;
import java.util.List;
import java.util.Locale;
import org.springframework.dao.DataAccessException;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.web.multipart.MultipartFile;

public class ReleaseAdminService {
    private final ReleasesService reads;
    private final ReleasesWriteService writes;
    private final JdbcTemplate jdbc;
    private final Path dataDir;
    private final PackagePathResolver paths;
    private final AuditWriteService audit;
    private final ObjectMapper mapper =
        new ObjectMapper().disable(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES);

    public ReleaseAdminService(
        ReleasesService reads,
        ReleasesWriteService writes,
        JdbcTemplate jdbc,
        Path dataDir
    ) {
        this(reads, writes, jdbc, jdbc, dataDir);
    }

    public ReleaseAdminService(
        ReleasesService reads,
        ReleasesWriteService writes,
        JdbcTemplate jdbc,
        JdbcTemplate auditJdbc,
        Path dataDir
    ) {
        this.reads = reads;
        this.writes = writes;
        this.jdbc = jdbc;
        this.dataDir = dataDir.toAbsolutePath().normalize();
        this.paths = new PackagePathResolver(this.dataDir);
        this.audit = new AuditWriteService(auditJdbc);
    }

    public Release create(CreateRelease input) {
        if (input == null || empty(input.version())) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "version is required");
        }
        ActiveScript script;
        try {
            script = activeScript();
        } catch (RuntimeException ex) {
            throw new SidecarException(
                ErrorCode.BAD_REQUEST, "active script is required before creating release");
        }
        String channel = empty(input.channel()) ? "stable" : input.channel();
        int rollout = input.rolloutPercent() == 0 ? 100 : input.rolloutPercent();
        String id = newReleaseId(input.version(), channel);
        try {
            writes.save(new ReleaseWriteInput(
                id, input.version(), channel, ReleaseStatuses.DRAFT,
                value(input.minVersion()), input.forceUpdate(), rollout, value(input.notes()),
                input.targeting() == null ? TargetingRules.empty() : input.targeting(),
                script.id(), script.version(), script.sha256(), Instant.now(),
                null, null, null, ""
            ));
        } catch (DataAccessException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to create release");
        }
        Release release = reads.get(id);
        audit("release_created", release, "");
        return release;
    }

    public Release publish(String id) {
        Release release = reads.get(id);
        ActiveScript script;
        try {
            script = activeScript();
        } catch (RuntimeException ex) {
            throw new SidecarException(
                ErrorCode.BAD_REQUEST, "Cannot publish release: no active script available");
        }
        ensurePublishablePackage(id);
        Instant now = Instant.now();
        Release next = copy(
            release, ReleaseStatuses.PUBLISHED, script, now, null,
            release.rolledBackAt(), release.rolledBackTo());
        saveUpdated(next);
        audit("release_published", next, "status=published");
        return reads.get(id);
    }

    public Release pause(String id) {
        Release release = reads.get(id);
        Release next = copy(
            release, ReleaseStatuses.PAUSED,
            new ActiveScript(release.scriptId(), release.scriptVersion(), release.scriptSha256()),
            release.publishedAt(), Instant.now(), release.rolledBackAt(), release.rolledBackTo());
        saveUpdated(next);
        audit("release_paused", next, "status=paused");
        return reads.get(id);
    }

    public Release rollback(String id, String targetReleaseId) {
        Release release = reads.get(id);
        Release next = copy(
            release, ReleaseStatuses.ROLLED_BACK,
            new ActiveScript(release.scriptId(), release.scriptVersion(), release.scriptSha256()),
            release.publishedAt(), release.pausedAt(), Instant.now(), value(targetReleaseId));
        saveUpdated(next);
        audit("release_rolled_back", next, "target_release_id=" + value(targetReleaseId));
        return reads.get(id);
    }

    public EventsResult events(String id) {
        return new EventsResult(reads.listEvents(id, 100), reads.metrics(id));
    }

    public ReleasePackage uploadPackage(String releaseId, String kind, MultipartFile file) {
        Release release = reads.get(releaseId);
        if (file == null || file.getOriginalFilename() == null) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "file is required");
        }
        String original = file.getOriginalFilename();
        String extension = extension(original).toLowerCase(Locale.ROOT);
        if (!".exe".equals(extension) && !".msi".equals(extension)) {
            throw new SidecarException(
                ErrorCode.BAD_REQUEST, "only .exe or .msi packages are supported");
        }
        String effectiveKind = empty(kind) ? "bundle" : kind;
        String filename = sanitizePackageFilename(release.version(), extension);
        Path relative = Path.of("updates", releaseId, filename);
        Path destination = paths.resolve(relative.toString());
        long size = 0;
        String sha256;
        try {
            Files.createDirectories(destination.getParent());
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            try (InputStream input = file.getInputStream();
                 OutputStream output = Files.newOutputStream(
                     destination,
                     StandardOpenOption.CREATE,
                     StandardOpenOption.TRUNCATE_EXISTING,
                     StandardOpenOption.WRITE)) {
                byte[] buffer = new byte[64 * 1024];
                int read;
                while ((read = input.read(buffer)) >= 0) {
                    if (read == 0) {
                        continue;
                    }
                    output.write(buffer, 0, read);
                    digest.update(buffer, 0, read);
                    size += read;
                }
            }
            sha256 = HexFormat.of().formatHex(digest.digest());
        } catch (Exception ex) {
            try {
                Files.deleteIfExists(destination);
            } catch (Exception ignored) {
                // best effort, matching Go cleanup
            }
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to save package");
        }

        ReleasePackage pkg = new ReleasePackage(
            releaseId + "-" + effectiveKind, releaseId, effectiveKind, filename,
            relative.toString(), size, sha256, Instant.now());
        try {
            jdbc.update("""
                INSERT INTO release_packages
                    (id, release_id, kind, filename, path, file_size, sha256, created_at)
                VALUES (?,?,?,?,?,?,?,?)
                ON CONFLICT(id) DO UPDATE SET
                    release_id=excluded.release_id, kind=excluded.kind,
                    filename=excluded.filename, path=excluded.path,
                    file_size=excluded.file_size, sha256=excluded.sha256,
                    created_at=excluded.created_at
                """,
                pkg.id(), pkg.releaseId(), pkg.kind(), pkg.filename(), pkg.path(),
                pkg.fileSize(), pkg.sha256(), TimestampWriter.format(pkg.createdAt()));
        } catch (DataAccessException ex) {
            throw new SidecarException(
                ErrorCode.INTERNAL_ERROR, "failed to save package metadata");
        }
        audit("release_package_uploaded", release,
            "package_id=" + pkg.id() + " kind=" + pkg.kind()
                + " sha256=" + pkg.sha256() + " size=" + pkg.fileSize());
        return pkg;
    }

    private void ensurePublishablePackage(String releaseId) {
        List<ReleasePackage> packages = reads.listPackages(releaseId);
        if (packages.isEmpty()) {
            throw new SidecarException(
                ErrorCode.BAD_REQUEST, "Cannot publish release: no update package uploaded");
        }
        for (ReleasePackage pkg : packages) {
            try {
                if (Files.isRegularFile(paths.resolve(pkg.path()))) {
                    return;
                }
            } catch (RuntimeException ignored) {
                // Try remaining package records, as Go does.
            }
        }
        throw new SidecarException(
            ErrorCode.BAD_REQUEST, "Cannot publish release: package file not found");
    }

    private void saveUpdated(Release release) {
        try {
            writes.save(toInput(release));
        } catch (DataAccessException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to update release");
        }
    }

    private void audit(String action, Release release, String extra) {
        String detail = "id=" + release.id() + " version=" + release.version()
            + " channel=" + release.channel();
        if (!empty(extra)) {
            detail += " " + extra;
        }
        try {
            audit.append(new AuditEvent(
                Instant.now(), action, "", "", "", "", "", "", "", "",
                "", "", "", "", detail));
        } catch (RuntimeException ex) {
            throw new SidecarException(
                ErrorCode.INTERNAL_ERROR, "action succeeded but audit log failed");
        }
    }

    private ActiveScript activeScript() {
        Path scriptDir = dataDir.resolve("scripts");
        try {
            Path indexPath = scriptDir.resolve("index.json");
            if (Files.exists(indexPath)) {
                com.fasterxml.jackson.databind.JsonNode index = mapper.readTree(indexPath.toFile());
                String activeId = index.path("active_id").asText("");
                if (!activeId.isEmpty()) {
                    for (com.fasterxml.jackson.databind.JsonNode item : index.path("scripts")) {
                        if (activeId.equals(item.path("id").asText())) {
                            return new ActiveScript(
                                activeId,
                                item.path("version").asText(""),
                                item.path("sha256").asText(""));
                        }
                    }
                    Path modulePath = scriptDir.resolve(activeId + ".json").normalize();
                    if (modulePath.startsWith(scriptDir.normalize()) && Files.exists(modulePath)) {
                        ActiveScript module = mapper.readValue(modulePath.toFile(), ActiveScript.class);
                        if (module != null && !empty(module.id())) {
                            return module;
                        }
                    }
                }
            }
            ActiveScript script = mapper.readValue(
                scriptDir.resolve("active.json").toFile(), ActiveScript.class);
            if (script == null || empty(script.id())) {
                throw new IllegalStateException("active script is empty");
            }
            return script;
        } catch (Exception ex) {
            throw new IllegalStateException("active script unavailable", ex);
        }
    }

    private static Release copy(
        Release source,
        String status,
        ActiveScript script,
        Instant publishedAt,
        Instant pausedAt,
        Instant rolledBackAt,
        String rolledBackTo
    ) {
        return new Release(
            source.id(), source.version(), source.channel(), status, source.minVersion(),
            source.forceUpdate(), source.rolloutPercent(), source.notes(), source.targeting(),
            script.id(), script.version(), script.sha256(), source.createdAt(),
            publishedAt, pausedAt, rolledBackAt, rolledBackTo);
    }

    private static ReleaseWriteInput toInput(Release release) {
        return new ReleaseWriteInput(
            release.id(), release.version(), release.channel(), release.status(),
            release.minVersion(), release.forceUpdate(), release.rolloutPercent(),
            release.notes(), release.targeting(), release.scriptId(), release.scriptVersion(),
            release.scriptSha256(), release.createdAt(), release.publishedAt(),
            release.pausedAt(), release.rolledBackAt(), release.rolledBackTo());
    }

    private static String newReleaseId(String version, String channel) {
        String clean = version.replace('.', '-').replace(' ', '-')
            .replace('/', '-').replace('\\', '-');
        Instant now = Instant.now();
        long unixNanos = now.getEpochSecond() * 1_000_000_000L + now.getNano();
        return "rel-" + channel + "-" + clean + "-" + unixNanos;
    }

    private static String sanitizePackageFilename(String version, String extension) {
        String base = ("LingqiaoSetup-" + version)
            .replace('/', '-').replace('\\', '-').replace(' ', '-');
        return base + extension;
    }

    private static String extension(String filename) {
        int slash = Math.max(filename.lastIndexOf('/'), filename.lastIndexOf('\\'));
        int dot = filename.lastIndexOf('.');
        return dot > slash ? filename.substring(dot) : "";
    }

    private static boolean empty(String value) {
        return value == null || value.isEmpty();
    }

    private static String value(String value) {
        return value == null ? "" : value;
    }

    public record CreateRelease(
        String version,
        String channel,
        String minVersion,
        boolean forceUpdate,
        int rolloutPercent,
        String notes,
        TargetingRules targeting
    ) {
    }

    public record ActiveScript(String id, String version, String sha256) {
    }

    public record EventsResult(List<ReleaseEventView> events, ReleaseMetrics metrics) {
    }

    @com.fasterxml.jackson.annotation.JsonInclude(
        com.fasterxml.jackson.annotation.JsonInclude.Include.NON_EMPTY)
    public record ReleaseEventView(
        long id,
        @com.fasterxml.jackson.annotation.JsonProperty("release_id") String releaseId,
        String version,
        @com.fasterxml.jackson.annotation.JsonProperty("machine_id") String machineId,
        @com.fasterxml.jackson.annotation.JsonProperty("card_code") String cardCode,
        @com.fasterxml.jackson.annotation.JsonProperty("agent_id") String agentId,
        String type,
        @com.fasterxml.jackson.annotation.JsonProperty("error_code") String errorCode,
        String detail,
        @com.fasterxml.jackson.annotation.JsonProperty("created_at") Instant createdAt
    ) {
    }
}
