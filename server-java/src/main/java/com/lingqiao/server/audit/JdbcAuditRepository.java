package com.lingqiao.server.audit;

import com.lingqiao.server.common.jdbc.TimestampWriter;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Timestamp;
import java.time.Instant;
import java.time.OffsetDateTime;
import java.time.ZoneOffset;
import java.time.format.DateTimeParseException;
import java.util.ArrayList;
import java.util.List;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;

public final class JdbcAuditRepository implements AuditRepository {
    private static final RowMapper<AuditEvent> ROW_MAPPER = JdbcAuditRepository::mapRow;
    private final JdbcTemplate jdbc;

    public JdbcAuditRepository(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    @Override
    public List<AuditEvent> list(AuditFilter filter, int limit) {
        List<Object> args = new ArrayList<>();
        String where = buildWhere(filter, args);
        args.add(Math.max(1, Math.min(limit, 10000)));
        return jdbc.query("""
            SELECT time, action, card, machine, agent_id, detail, addr,
                   actor_type, actor_id, resource_type, resource_id,
                   before, after, user_agent, request_id
            FROM audit_log %s
            ORDER BY time DESC
            LIMIT ?
            """.formatted(where), ROW_MAPPER, args.toArray());
    }

    private static String buildWhere(AuditFilter filter, List<Object> args) {
        List<String> clauses = new ArrayList<>();
        addEquals(clauses, args, "action", filter.action());
        addEquals(clauses, args, "actor_type", filter.actorType());
        addEquals(clauses, args, "actor_id", filter.actorId());
        addEquals(clauses, args, "resource_type", filter.resourceType());
        addEquals(clauses, args, "resource_id", filter.resourceId());
        addEquals(clauses, args, "card", filter.card());
        addEquals(clauses, args, "agent_id", filter.agentId());
        addEquals(clauses, args, "machine", filter.machine());
        addEquals(clauses, args, "addr", filter.ip());
        if (filter.from() != null) {
            clauses.add("time >= ?");
            args.add(TimestampWriter.format(filter.from()));
        }
        if (filter.to() != null) {
            clauses.add("time <= ?");
            args.add(TimestampWriter.format(filter.to()));
        }
        if (hasText(filter.query())) {
            clauses.add("(detail LIKE ? OR action LIKE ?)");
            String like = "%" + filter.query() + "%";
            args.add(like);
            args.add(like);
        }
        return clauses.isEmpty() ? "" : "WHERE " + String.join(" AND ", clauses);
    }

    private static void addEquals(List<String> clauses, List<Object> args, String column, String value) {
        if (hasText(value)) {
            clauses.add(column + " = ?");
            args.add(value);
        }
    }

    private static boolean hasText(String value) {
        return value != null && !value.isBlank();
    }

    private static AuditEvent mapRow(ResultSet rs, int rowNum) throws SQLException {
        return new AuditEvent(
            readTime(rs),
            rs.getString("action"),
            rs.getString("actor_type"),
            rs.getString("actor_id"),
            rs.getString("resource_type"),
            rs.getString("resource_id"),
            rs.getString("before"),
            rs.getString("after"),
            rs.getString("user_agent"),
            rs.getString("request_id"),
            rs.getString("card"),
            rs.getString("agent_id"),
            rs.getString("machine"),
            rs.getString("addr"),
            rs.getString("detail")
        );
    }

    private static OffsetDateTime readTime(ResultSet rs) throws SQLException {
        String raw = rs.getString("time");
        if (raw != null && !raw.isBlank()) {
            OffsetDateTime parsed = parseStoredTime(raw);
            if (parsed != null) {
                return parsed;
            }
        }
        Timestamp timestamp = rs.getTimestamp("time");
        return timestamp == null
            ? Instant.EPOCH.atOffset(ZoneOffset.UTC)
            : timestamp.toInstant().atOffset(ZoneOffset.UTC);
    }

    static OffsetDateTime parseStoredTime(String raw) {
        if (raw == null || raw.isBlank()) {
            return null;
        }
        try {
            return OffsetDateTime.parse(raw);
        } catch (DateTimeParseException ignored) {
            // Older Go builds persisted time.Time.String(), including a zone name
            // and monotonic suffix: "2026-07-30 18:57:26.1 +0800 CST m=+...".
        }
        String[] fields = raw.trim().split("\\s+");
        if (fields.length >= 3 && fields[0].matches("\\d{4}-\\d{2}-\\d{2}")
            && fields[2].matches("[+-]\\d{4}")) {
            String offset = fields[2].substring(0, 3) + ":" + fields[2].substring(3);
            try {
                return OffsetDateTime.parse(fields[0] + "T" + fields[1] + offset);
            } catch (DateTimeParseException ignored) {
                // Fall through to the driver's legacy timestamp parser.
            }
        }
        return null;
    }
}
