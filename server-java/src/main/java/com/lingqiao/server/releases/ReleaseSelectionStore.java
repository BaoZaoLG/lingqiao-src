package com.lingqiao.server.releases;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.time.Instant;
import java.time.format.DateTimeParseException;
import java.util.List;
import java.util.Optional;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;

/**
 * Read-side release selection store mirroring Go releases.SQLiteStore
 * ListPublishedReleases / ListPackages / GetPackage.
 */
public class ReleaseSelectionStore {
    private static final String RELEASE_COLUMNS = """
        SELECT id, version, channel, status, min_version, force_update, rollout_percent, notes,
               targeting_json, script_id, script_version, script_sha256,
               created_at, published_at, paused_at, rolled_back_at, rolled_back_to
        FROM releases
        """;

    private static final String PACKAGE_COLUMNS = """
        SELECT id, release_id, kind, filename, path, file_size, sha256, created_at
        FROM release_packages
        """;

    private final JdbcTemplate jdbc;
    private final ObjectMapper mapper = new ObjectMapper();
    private final RowMapper<Release> releaseMapper = (rs, rowNum) -> mapRelease(rs);
    private final RowMapper<ReleasePackage> packageMapper = (rs, rowNum) -> mapPackage(rs);

    public ReleaseSelectionStore(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    public List<Release> listPublished(String channel) {
        String effective = channel == null || channel.isBlank() ? "stable" : channel;
        return jdbc.query(RELEASE_COLUMNS
            + " WHERE channel = ? AND status = ? ORDER BY published_at DESC, created_at DESC",
            releaseMapper, effective, ReleaseStatuses.PUBLISHED);
    }

    public List<ReleasePackage> listPackages(String releaseId) {
        return jdbc.query(PACKAGE_COLUMNS + " WHERE release_id = ? ORDER BY created_at DESC",
            packageMapper, releaseId);
    }

    public Optional<ReleasePackage> findPackage(String packageId) {
        List<ReleasePackage> packages = jdbc.query(PACKAGE_COLUMNS + " WHERE id = ?", packageMapper, packageId);
        return packages.isEmpty() ? Optional.empty() : Optional.of(packages.get(0));
    }

    public ReleasePackage requirePackage(String packageId) {
        return findPackage(packageId)
            .orElseThrow(() -> new SidecarException(ErrorCode.NOT_FOUND, "package not found"));
    }

    private Release mapRelease(ResultSet rs) throws SQLException {
        return new Release(
            nullToEmpty(rs.getString("id")),
            nullToEmpty(rs.getString("version")),
            nullToEmpty(rs.getString("channel")),
            nullToEmpty(rs.getString("status")),
            nullToEmpty(rs.getString("min_version")),
            rs.getInt("force_update") != 0,
            rs.getInt("rollout_percent"),
            nullToEmpty(rs.getString("notes")),
            parseTargeting(rs.getString("targeting_json")),
            nullToEmpty(rs.getString("script_id")),
            nullToEmpty(rs.getString("script_version")),
            nullToEmpty(rs.getString("script_sha256")),
            parseInstant(rs.getString("created_at")),
            parseInstant(rs.getString("published_at")),
            parseInstant(rs.getString("paused_at")),
            parseInstant(rs.getString("rolled_back_at")),
            nullToEmpty(rs.getString("rolled_back_to"))
        );
    }

    private static ReleasePackage mapPackage(ResultSet rs) throws SQLException {
        return new ReleasePackage(
            nullToEmpty(rs.getString("id")),
            nullToEmpty(rs.getString("release_id")),
            nullToEmpty(rs.getString("kind")),
            nullToEmpty(rs.getString("filename")),
            nullToEmpty(rs.getString("path")),
            rs.getLong("file_size"),
            nullToEmpty(rs.getString("sha256")),
            parseInstant(rs.getString("created_at"))
        );
    }

    private TargetingRules parseTargeting(String json) {
        if (json == null || json.isBlank()) {
            return TargetingRules.empty();
        }
        try {
            TargetingRules parsed = mapper.readValue(json, TargetingRules.class);
            return parsed == null ? TargetingRules.empty() : parsed;
        } catch (Exception ex) {
            return TargetingRules.empty();
        }
    }

    static Instant parseInstant(String value) {
        if (value == null || value.isBlank()) {
            return null;
        }
        try {
            return Instant.parse(value.trim());
        } catch (DateTimeParseException ex) {
            return null;
        }
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }
}
