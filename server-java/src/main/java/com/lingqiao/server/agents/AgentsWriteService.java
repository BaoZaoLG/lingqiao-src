package com.lingqiao.server.agents;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.util.Objects;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.security.crypto.bcrypt.BCrypt;
import org.springframework.transaction.support.TransactionTemplate;

/**
 * Java write path for agent administration. Mirrors Go agents.Service
 * for status, limits, password reset, and deletion.
 */
public class AgentsWriteService {
    private static final int MIN_PASSWORD_LENGTH = 6;
    private static final int MAX_PASSWORD_BYTES = 72;

    private final JdbcTemplate jdbc;
    private final TransactionTemplate transactions;

    public AgentsWriteService(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
        this.transactions = new TransactionTemplate(new DataSourceTransactionManager(
            Objects.requireNonNull(jdbc.getDataSource(), "write datasource is required")));
    }

    public void setStatus(String agentId, boolean disabled) {
        requireId(agentId);
        transactions.executeWithoutResult(status ->
            requireUpdated(jdbc.update("UPDATE agents SET disabled = ? WHERE id = ?", disabled ? 1 : 0, agentId)));
    }

    public void setLimits(String agentId, int maxSessionDurationMinutes, int maxDailySessions, int maxConcurrentSessions) {
        requireId(agentId);
        if (maxSessionDurationMinutes < 0 || maxDailySessions < 0 || maxConcurrentSessions < 0) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "agent limits must not be negative");
        }
        transactions.executeWithoutResult(status -> requireUpdated(jdbc.update("""
            UPDATE agents
            SET max_session_duration_minutes = ?, max_daily_sessions = ?, max_concurrent_sessions = ?
            WHERE id = ?
            """, maxSessionDurationMinutes, maxDailySessions, maxConcurrentSessions, agentId)));
    }

    public void forceResetPassword(String agentId, String newPassword) {
        requireId(agentId);
        if (newPassword == null
            || newPassword.length() < MIN_PASSWORD_LENGTH
            || newPassword.getBytes(java.nio.charset.StandardCharsets.UTF_8).length > MAX_PASSWORD_BYTES) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "密码长度必须在6-72位之间");
        }
        String hash = BCrypt.hashpw(newPassword, BCrypt.gensalt());
        transactions.executeWithoutResult(status ->
            requireUpdated(jdbc.update("UPDATE agents SET password = ? WHERE id = ?", hash, agentId)));
    }

    public void delete(String agentId) {
        requireId(agentId);
        transactions.executeWithoutResult(status ->
            requireUpdated(jdbc.update("DELETE FROM agents WHERE id = ?", agentId)));
    }

    private static void requireId(String agentId) {
        if (agentId == null || agentId.isBlank()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "agent_id is required");
        }
    }

    private static void requireUpdated(int rows) {
        if (rows == 0) {
            throw new SidecarException(ErrorCode.AGENT_NOT_FOUND, "agent not found");
        }
    }
}
