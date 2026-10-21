package com.lingqiao.server.system;

import com.lingqiao.server.common.jdbc.TimestampWriter;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.sql.Timestamp;
import java.time.Instant;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.transaction.annotation.Transactional;

/**
 * Java write path for the blacklist module. Mirrors Go blacklist.Service semantics.
 */
public class BlacklistWriteService {
    private final JdbcTemplate jdbc;

    public BlacklistWriteService(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    @Transactional
    public void add(String type, String value, String reason) {
        String trimmed = value == null ? "" : value.trim();
        if (trimmed.isEmpty()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "value is required");
        }
        jdbc.update("INSERT OR REPLACE INTO blacklist (value, type, reason, created_at) VALUES (?, ?, ?, ?)",
            trimmed,
            type == null ? "" : type,
            reason == null ? "" : reason,
            TimestampWriter.format(Instant.now())
        );
    }

    @Transactional
    public void remove(String value) {
        String trimmed = value == null ? "" : value.trim();
        if (trimmed.isEmpty()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "value is required");
        }
        jdbc.update("DELETE FROM blacklist WHERE value = ?", trimmed);
    }
}
