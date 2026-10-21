package com.lingqiao.server.releases;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.time.Instant;
import java.time.format.DateTimeParseException;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;

public class ReleasesService {
    private static final String SELECT_COLUMNS = """
        SELECT id, version, channel, status, min_version, force_update, rollout_percent, notes,
               targeting_json, script_id, script_version, script_sha256,
               created_at, published_at, paused_at, rolled_back_at, rolled_back_to
        FROM releases
        """;

    private final JdbcTemplate jdbc;
    private final ObjectMapper mapper = new ObjectMapper();
    private final RowMapper<Release> rowMapper = (rs, rowNum) -> mapRelease(rs);
    private final RowMapper<ReleasePackage> packageMapper = (rs, rowNum) -> new ReleasePackage(
        nullToEmpty(rs.getString("id")),
        nullToEmpty(rs.getString("release_id")),
        nullToEmpty(rs.getString("kind")),
        nullToEmpty(rs.getString("filename")),
        nullToEmpty(rs.getString("path")),
        rs.getLong("file_size"),
        nullToEmpty(rs.getString("sha256")),
        parseInstant(rs.getString("created_at")));

    public ReleasesService(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    public List<Release> list() {
        return jdbc.query(SELECT_COLUMNS + " ORDER BY created_at DESC", rowMapper);
    }

    public Release get(String id) {
        List<Release> releases = jdbc.query(SELECT_COLUMNS + " WHERE id = ?", rowMapper, id);
        if (releases.isEmpty()) {
            throw new SidecarException(ErrorCode.RELEASE_NOT_FOUND, "release not found");
        }
        return releases.get(0);
    }

    /** Mirrors Go SQLiteStore.ListPackages ordering. */
    public List<ReleasePackage> listPackages(String releaseId) {
        return jdbc.query("""
            SELECT id, release_id, kind, filename, path, file_size, sha256, created_at
            FROM release_packages WHERE release_id = ? ORDER BY created_at DESC
            """, packageMapper, releaseId);
    }

    /** Mirrors Go SQLiteStore.ReleaseMetrics: per-type counts plus distinct machines. */
    public ReleaseMetrics metrics(String releaseId) {
        Map<String, Integer> counts = new LinkedHashMap<>();
        jdbc.query("SELECT type, COUNT(*) AS total FROM release_events WHERE release_id = ? GROUP BY type",
            rs -> {
                counts.put(nullToEmpty(rs.getString("type")), rs.getInt("total"));
            }, releaseId);

        Integer uniqueMachines = jdbc.queryForObject("""
            SELECT COUNT(DISTINCT machine_id) FROM release_events
            WHERE release_id = ? AND machine_id != ''
            """, Integer.class, releaseId);

        return new ReleaseMetrics(
            releaseId,
            counts.getOrDefault(ReleaseEventRecorder.OFFERED, 0),
            counts.getOrDefault(ReleaseEventRecorder.DOWNLOAD_STARTED, 0),
            counts.getOrDefault(ReleaseEventRecorder.DOWNLOAD_FAILED, 0),
            counts.getOrDefault(ReleaseEventRecorder.DOWNLOAD_COMPLETED, 0),
            counts.getOrDefault(ReleaseEventRecorder.INSTALL_STARTED, 0),
            counts.getOrDefault(ReleaseEventRecorder.INSTALL_SUCCESS, 0),
            counts.getOrDefault(ReleaseEventRecorder.INSTALL_FAILED, 0),
            counts.getOrDefault(ReleaseEventRecorder.ROLLBACK, 0),
            counts.getOrDefault(ReleaseEventRecorder.DISMISSED, 0),
            uniqueMachines == null ? 0 : uniqueMachines);
    }

    public List<ReleaseAdminService.ReleaseEventView> listEvents(String releaseId, int limit) {
        int effectiveLimit = limit <= 0 || limit > 500 ? 100 : limit;
        return jdbc.query("""
            SELECT id, release_id, version, machine_id, card_code, agent_id,
                   type, error_code, detail, created_at
            FROM release_events
            WHERE release_id = ?
            ORDER BY id DESC
            LIMIT ?
            """, (rs, rowNum) -> new ReleaseAdminService.ReleaseEventView(
                rs.getLong("id"),
                nullToEmpty(rs.getString("release_id")),
                nullToEmpty(rs.getString("version")),
                nullToEmpty(rs.getString("machine_id")),
                nullToEmpty(rs.getString("card_code")),
                nullToEmpty(rs.getString("agent_id")),
                nullToEmpty(rs.getString("type")),
                nullToEmpty(rs.getString("error_code")),
                nullToEmpty(rs.getString("detail")),
                parseInstant(rs.getString("created_at"))
            ), releaseId, effectiveLimit);
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

    private static Instant parseInstant(String value) {
        if (value == null || value.isBlank()) {
            return null;
        }
        try {
            return Instant.parse(value);
        } catch (DateTimeParseException ex) {
            return null;
        }
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }
}
