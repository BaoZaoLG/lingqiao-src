package com.lingqiao.server.releases;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.time.Instant;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.Objects;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

/**
 * Java write path for release lifecycle. Mirrors Go releases.SQLiteStore.SaveRelease
 * upsert semantics plus publish/pause/rollback state transitions.
 */
public class ReleasesWriteService {
    private static final DateTimeFormatter TIME_FORMAT =
        DateTimeFormatter.ofPattern("yyyy-MM-dd'T'HH:mm:ss'Z'").withZone(ZoneOffset.UTC);

    private static final String UPSERT_SQL = """
        INSERT INTO releases (
            id, version, channel, status, min_version, force_update, rollout_percent, notes,
            targeting_json, script_id, script_version, script_sha256,
            created_at, published_at, paused_at, rolled_back_at, rolled_back_to
        ) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
        ON CONFLICT(id) DO UPDATE SET
            version = excluded.version,
            channel = excluded.channel,
            status = excluded.status,
            min_version = excluded.min_version,
            force_update = excluded.force_update,
            rollout_percent = excluded.rollout_percent,
            notes = excluded.notes,
            targeting_json = excluded.targeting_json,
            script_id = excluded.script_id,
            script_version = excluded.script_version,
            script_sha256 = excluded.script_sha256,
            created_at = excluded.created_at,
            published_at = excluded.published_at,
            paused_at = excluded.paused_at,
            rolled_back_at = excluded.rolled_back_at,
            rolled_back_to = excluded.rolled_back_to
        """;

    private final JdbcTemplate jdbc;
    private final TransactionTemplate transactions;
    private final ObjectMapper mapper = new ObjectMapper();

    public ReleasesWriteService(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
        this.transactions = new TransactionTemplate(new DataSourceTransactionManager(
            Objects.requireNonNull(jdbc.getDataSource(), "write datasource is required")));
    }

    public void save(ReleaseWriteInput input) {
        if (input == null || isBlank(input.id()) || isBlank(input.version())) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "release id and version are required");
        }
        String channel = isBlank(input.channel()) ? "stable" : input.channel();
        String status = isBlank(input.status()) ? ReleaseStatuses.DRAFT : input.status();
        if (!ReleaseStatuses.isValid(status)) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "invalid release status");
        }
        int rollout = clampRollout(input.rolloutPercent());
        Instant createdAt = input.createdAt() == null ? Instant.now() : input.createdAt();

        transactions.executeWithoutResult(tx -> jdbc.update(UPSERT_SQL,
            input.id(), input.version(), channel, status,
            nullToEmpty(input.minVersion()), input.forceUpdate() ? 1 : 0, rollout,
            nullToEmpty(input.notes()), serializeTargeting(input.targeting()),
            nullToEmpty(input.scriptId()), nullToEmpty(input.scriptVersion()), nullToEmpty(input.scriptSha256()),
            formatTime(createdAt), formatTimeOrEmpty(input.publishedAt()),
            formatTimeOrEmpty(input.pausedAt()), formatTimeOrEmpty(input.rolledBackAt()),
            nullToEmpty(input.rolledBackTo())));
    }

    public void publish(String id) {
        Instant now = Instant.now();
        requireUpdated(transactions.execute(tx -> jdbc.update("""
            UPDATE releases SET status = ?, published_at = ?, paused_at = '', rolled_back_at = '', rolled_back_to = ''
            WHERE id = ?
            """, ReleaseStatuses.PUBLISHED, formatTime(now), requireId(id))));
    }

    public void pause(String id) {
        Instant now = Instant.now();
        requireUpdated(transactions.execute(tx -> jdbc.update(
            "UPDATE releases SET status = ?, paused_at = ? WHERE id = ?",
            ReleaseStatuses.PAUSED, formatTime(now), requireId(id))));
    }

    public void rollback(String id, String rolledBackTo) {
        Instant now = Instant.now();
        requireUpdated(transactions.execute(tx -> jdbc.update("""
            UPDATE releases SET status = ?, rolled_back_at = ?, rolled_back_to = ? WHERE id = ?
            """, ReleaseStatuses.ROLLED_BACK, formatTime(now), nullToEmpty(rolledBackTo), requireId(id))));
    }

    public void setRolloutPercent(String id, int rolloutPercent) {
        requireUpdated(transactions.execute(tx -> jdbc.update(
            "UPDATE releases SET rollout_percent = ? WHERE id = ?",
            clampRollout(rolloutPercent), requireId(id))));
    }

    static int clampRollout(int value) {
        if (value < 0) {
            return 0;
        }
        return Math.min(value, 100);
    }

    static String formatTime(Instant instant) {
        return TIME_FORMAT.format(instant);
    }

    private static String formatTimeOrEmpty(Instant instant) {
        return instant == null ? "" : formatTime(instant);
    }

    private String serializeTargeting(TargetingRules targeting) {
        try {
            return mapper.writeValueAsString(targeting == null ? TargetingRules.empty() : targeting);
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to serialize targeting rules");
        }
    }

    private static String requireId(String id) {
        if (isBlank(id)) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "release id is required");
        }
        return id;
    }

    private static void requireUpdated(Integer rows) {
        if (rows == null || rows == 0) {
            throw new SidecarException(ErrorCode.RELEASE_NOT_FOUND, "release not found");
        }
    }

    private static boolean isBlank(String value) {
        return value == null || value.isBlank();
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }
}
