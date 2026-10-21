package com.lingqiao.server.sessions;

import com.lingqiao.server.common.jdbc.TimestampWriter;
import com.lingqiao.server.common.jdbc.TimestampReader;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Timestamp;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.core.RowMapper;

public class SessionsService {
    private static final RowMapper<Session> ROW_MAPPER = (rs, rowNum) -> mapSession(rs);

    private final JdbcTemplate jdbc;

    public SessionsService(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    public SessionListResult list(SessionListFilter filter) {
        List<Object> args = new ArrayList<>();
        String where = buildWhere(filter, args);
        Long total = jdbc.queryForObject("SELECT COUNT(*) FROM sessions " + where, Long.class, args.toArray());
        List<Object> pageArgs = new ArrayList<>(args);
        pageArgs.add(filter.perPage());
        pageArgs.add((filter.page() - 1L) * filter.perPage());
        List<Session> items = jdbc.query("""
            SELECT token, card_code, machine_id, fingerprint, client_version, created_at, last_seen, expires_at, remote_addr
            FROM sessions %s
            ORDER BY last_seen DESC
            LIMIT ? OFFSET ?
            """.formatted(where), ROW_MAPPER, pageArgs.toArray());
        return new SessionListResult(items, total == null ? 0 : total, filter.page(), filter.perPage());
    }

    public Session get(String token) {
        List<Session> sessions = jdbc.query("""
            SELECT token, card_code, machine_id, fingerprint, client_version, created_at, last_seen, expires_at, remote_addr
            FROM sessions WHERE token = ?
            """, ROW_MAPPER, token);
        if (sessions.isEmpty()) {
            throw new SidecarException(ErrorCode.SESSION_NOT_FOUND, "session not found");
        }
        return sessions.get(0);
    }

    private static String buildWhere(SessionListFilter filter, List<Object> args) {
        List<String> clauses = new ArrayList<>();
        if (filter.activeOnly()) {
            clauses.add("expires_at > ?");
            args.add(TimestampWriter.format(Instant.now()));
        }
        if (hasText(filter.cardCode())) {
            clauses.add("card_code = ?");
            args.add(filter.cardCode());
        }
        if (hasText(filter.machineId())) {
            clauses.add("machine_id = ?");
            args.add(filter.machineId());
        }
        return clauses.isEmpty() ? "" : "WHERE " + String.join(" AND ", clauses);
    }

    private static Session mapSession(ResultSet rs) throws SQLException {
        return new Session(
            nullToEmpty(rs.getString("token")),
            nullToEmpty(rs.getString("card_code")),
            nullToEmpty(rs.getString("machine_id")),
            nullToEmpty(rs.getString("fingerprint")),
            nullToEmpty(rs.getString("client_version")),
            TimestampReader.readOrEpoch(rs, "created_at"),
            TimestampReader.readOrEpoch(rs, "last_seen"),
            TimestampReader.readOrEpoch(rs, "expires_at"),
            nullToEmpty(rs.getString("remote_addr"))
        );
    }

    private static boolean hasText(String value) {
        return value != null && !value.isBlank();
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }
}
