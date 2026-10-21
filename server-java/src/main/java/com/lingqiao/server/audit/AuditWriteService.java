package com.lingqiao.server.audit;

import com.lingqiao.server.common.jdbc.TimestampWriter;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.sql.Timestamp;
import java.time.Instant;
import java.util.Objects;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

/**
 * Java write path for audit events. Mirrors Go audit.Recorder.Append
 * including the synchronous insert and default timestamp behavior.
 */
public class AuditWriteService {
    private static final String INSERT_SQL = """
        INSERT INTO audit_log (
            time, action, card, machine, agent_id, detail, addr,
            actor_type, actor_id, resource_type, resource_id,
            before, after, user_agent, request_id
        ) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
        """;

    private final JdbcTemplate jdbc;
    private final TransactionTemplate transactions;

    public AuditWriteService(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
        this.transactions = new TransactionTemplate(new DataSourceTransactionManager(
            Objects.requireNonNull(jdbc.getDataSource(), "write datasource is required")));
    }

    public void append(AuditEvent event) {
        if (event == null) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "event is required");
        }
        if (event.action() == null || event.action().isBlank()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "action is required");
        }
        Instant time = event.time() == null || event.time().toInstant().equals(Instant.EPOCH)
            ? Instant.now()
            : event.time().toInstant();

        transactions.executeWithoutResult(status -> jdbc.update(INSERT_SQL,
            TimestampWriter.format(time),
            event.action(),
            nullToEmpty(event.card()),
            nullToEmpty(event.machine()),
            nullToEmpty(event.agentId()),
            nullToEmpty(event.detail()),
            nullToEmpty(event.ip()),
            nullToEmpty(event.actorType()),
            nullToEmpty(event.actorId()),
            nullToEmpty(event.resourceType()),
            nullToEmpty(event.resourceId()),
            nullToEmpty(event.before()),
            nullToEmpty(event.after()),
            nullToEmpty(event.userAgent()),
            nullToEmpty(event.requestId())
        ));
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }
}
