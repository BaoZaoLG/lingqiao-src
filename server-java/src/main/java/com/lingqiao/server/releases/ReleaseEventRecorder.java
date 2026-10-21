package com.lingqiao.server.releases;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.time.Instant;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.Objects;
import java.util.Set;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

/**
 * Records release lifecycle events, mirroring Go releases.SQLiteStore.RecordEvent.
 */
public class ReleaseEventRecorder {
    public static final String OFFERED = "offered";
    public static final String DOWNLOAD_STARTED = "download_started";
    public static final String DOWNLOAD_FAILED = "download_failed";
    public static final String DOWNLOAD_COMPLETED = "download_completed";
    public static final String INSTALL_STARTED = "install_started";
    public static final String INSTALL_SUCCESS = "install_success";
    public static final String INSTALL_FAILED = "install_failed";
    public static final String ROLLBACK = "rollback";
    public static final String DISMISSED = "dismissed";

    private static final Set<String> VALID_TYPES = Set.of(
        OFFERED, DOWNLOAD_STARTED, DOWNLOAD_FAILED, DOWNLOAD_COMPLETED,
        INSTALL_STARTED, INSTALL_SUCCESS, INSTALL_FAILED, ROLLBACK, DISMISSED);

    private static final DateTimeFormatter TIME_FORMAT =
        DateTimeFormatter.ofPattern("yyyy-MM-dd'T'HH:mm:ss'Z'").withZone(ZoneOffset.UTC);

    private final JdbcTemplate jdbc;
    private final TransactionTemplate transactions;

    public ReleaseEventRecorder(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
        this.transactions = new TransactionTemplate(new DataSourceTransactionManager(
            Objects.requireNonNull(jdbc.getDataSource(), "write datasource is required")));
    }

    public static boolean isValidType(String type) {
        return type != null && VALID_TYPES.contains(type);
    }

    public void record(ReleaseEvent event) {
        if (event == null || isBlank(event.releaseId()) || isBlank(event.type())) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "release id and event type are required");
        }
        Instant createdAt = event.createdAt() == null ? Instant.now() : event.createdAt();
        transactions.executeWithoutResult(status -> jdbc.update("""
            INSERT INTO release_events (
                release_id, version, machine_id, card_code, agent_id, type, error_code, detail, created_at
            ) VALUES (?,?,?,?,?,?,?,?,?)
            """,
            event.releaseId(),
            nullToEmpty(event.version()),
            nullToEmpty(event.machineId()),
            nullToEmpty(event.cardCode()),
            nullToEmpty(event.agentId()),
            event.type(),
            nullToEmpty(event.errorCode()),
            nullToEmpty(event.detail()),
            TIME_FORMAT.format(createdAt)));
    }

    /** Best-effort variant used on the offer path, matching Go's ignored error. */
    public void recordQuietly(ReleaseEvent event) {
        try {
            record(event);
        } catch (RuntimeException ignored) {
            // Go ignores this error so an audit failure never blocks an update offer.
        }
    }

    private static boolean isBlank(String value) {
        return value == null || value.isBlank();
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }

    public record ReleaseEvent(
        String releaseId,
        String version,
        String machineId,
        String cardCode,
        String agentId,
        String type,
        String errorCode,
        String detail,
        Instant createdAt
    ) {
    }
}
