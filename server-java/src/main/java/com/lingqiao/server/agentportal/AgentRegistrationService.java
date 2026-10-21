package com.lingqiao.server.agentportal;

import com.lingqiao.server.common.jdbc.TimestampWriter;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.nio.charset.StandardCharsets;
import java.security.SecureRandom;
import java.time.Instant;
import java.util.HexFormat;
import java.util.Objects;
import org.springframework.dao.DuplicateKeyException;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;
import org.springframework.security.crypto.bcrypt.BCrypt;

/**
 * Agent self-registration mirroring Go agents.Service#RegisterAgent:
 * username/password validation, then a single transaction that consumes the
 * invite code and inserts the agent, so an exhausted invite can never leak a row.
 */
public class AgentRegistrationService {
    static final int USERNAME_MIN = 3;
    static final int USERNAME_MAX = 20;
    static final int PASSWORD_MIN = 6;
    static final int PASSWORD_MAX_BYTES = 72;
    private static final int ID_ATTEMPTS = 5;

    private final JdbcTemplate jdbc;
    private final TransactionTemplate transactions;
    private final SecureRandom random = new SecureRandom();

    public AgentRegistrationService(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
        this.transactions = new TransactionTemplate(new DataSourceTransactionManager(
            Objects.requireNonNull(jdbc.getDataSource(), "write datasource is required")));
    }

    public RegisteredAgent register(String rawUsername, String password, String inviteCode) {
        String username = rawUsername == null ? "" : rawUsername.trim();
        validateUsername(username);
        validatePassword(password);

        Integer existing = jdbc.queryForObject(
            "SELECT COUNT(*) FROM agents WHERE username = ?", Integer.class, username);
        if (existing != null && existing > 0) {
            throw new SidecarException(ErrorCode.CONFLICT, "用户名已存在");
        }

        String hash = BCrypt.hashpw(password, BCrypt.gensalt());
        return transactions.execute(status -> {
            consumeInvite(inviteCode, username);
            return insertAgent(username, hash);
        });
    }

    static void validateUsername(String username) {
        if (username.length() < USERNAME_MIN || username.length() > USERNAME_MAX) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "用户名长度必须在3-20位之间");
        }
        for (int i = 0; i < username.length(); i++) {
            char c = username.charAt(i);
            boolean allowed = (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
                || (c >= '0' && c <= '9') || c == '_';
            if (!allowed) {
                throw new SidecarException(ErrorCode.BAD_REQUEST, "用户名只能包含字母、数字和下划线");
            }
        }
    }

    static void validatePassword(String password) {
        int bytes = password == null ? 0 : password.getBytes(StandardCharsets.UTF_8).length;
        if (bytes < PASSWORD_MIN || bytes > PASSWORD_MAX_BYTES) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "密码长度必须在6-72位之间");
        }
    }

    private void consumeInvite(String code, String usedBy) {
        if (code == null || code.isBlank()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "邀请码不能为空");
        }
        int affected = jdbc.update("""
            UPDATE invites
            SET use_count = use_count + 1,
                used_by = CASE WHEN used_by = '' OR used_by IS NULL THEN ? ELSE used_by END,
                used_at = ?
            WHERE code = ? AND (max_uses = 0 OR use_count < max_uses)
            """, usedBy, TimestampWriter.format(Instant.now()), code);
        if (affected > 0) {
            return;
        }
        Integer exists = jdbc.queryForObject(
            "SELECT COUNT(*) FROM invites WHERE code = ?", Integer.class, code);
        if (exists == null || exists == 0) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "邀请码无效");
        }
        throw new SidecarException(ErrorCode.BAD_REQUEST, "邀请码已用完");
    }

    private RegisteredAgent insertAgent(String username, String passwordHash) {
        for (int attempt = 0; attempt < ID_ATTEMPTS; attempt++) {
            String id = "AGT-" + shortId();
            try {
                jdbc.update("""
                    INSERT INTO agents (
                        id, username, password, prefix, created_at, disabled,
                        max_session_duration_minutes, max_daily_sessions, max_concurrent_sessions
                    ) VALUES (?, ?, ?, '', ?, 0, 0, 0, 0)
                    """, id, username, passwordHash, TimestampWriter.format(Instant.now()));
                return new RegisteredAgent(id, username);
            } catch (DuplicateKeyException ex) {
                if (isUsernameConflict(username)) {
                    throw new SidecarException(ErrorCode.CONFLICT, "用户名已存在");
                }
            }
        }
        throw new SidecarException(ErrorCode.INTERNAL_ERROR, "注册失败，请稍后重试");
    }

    private boolean isUsernameConflict(String username) {
        Integer count = jdbc.queryForObject(
            "SELECT COUNT(*) FROM agents WHERE username = ?", Integer.class, username);
        return count != null && count > 0;
    }

    String shortId() {
        byte[] buf = new byte[6];
        random.nextBytes(buf);
        return HexFormat.of().formatHex(buf);
    }

    public record RegisteredAgent(String agentId, String username) {
    }
}
