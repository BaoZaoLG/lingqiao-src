package com.lingqiao.server.agentportal;

import com.lingqiao.server.common.jdbc.TimestampWriter;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.SecureRandom;
import java.sql.Timestamp;
import java.time.Duration;
import java.time.Instant;
import java.time.format.DateTimeParseException;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.security.crypto.bcrypt.BCrypt;

/**
 * Agent portal authentication mirroring Go agents.AgentHandler:
 * in-memory session tokens prefixed with "agent_", bcrypt-with-legacy password
 * verification and per-IP login/registration rate limiting.
 */
public class AgentPortalAuthService {
    static final String TOKEN_PREFIX = "agent_";
    static final int LOGIN_ATTEMPT_LIMIT = 5;
    static final Duration LOGIN_WINDOW = Duration.ofMinutes(15);
    static final int REGISTER_ATTEMPT_LIMIT = 3;
    static final Duration REGISTER_WINDOW = Duration.ofHours(1);
    static final int PASSWORD_ATTEMPT_LIMIT = 3;
    static final Duration PASSWORD_WINDOW = Duration.ofHours(1);

    private final JdbcTemplate jdbc;
    private final Duration sessionTtl;
    private final AgentSessionStore sessionStore;
    private final com.lingqiao.server.admin.LoginRateLimiter loginLimiter =
        new com.lingqiao.server.admin.LoginRateLimiter(LOGIN_ATTEMPT_LIMIT, LOGIN_WINDOW);
    private final com.lingqiao.server.admin.LoginRateLimiter registerLimiter =
        new com.lingqiao.server.admin.LoginRateLimiter(REGISTER_ATTEMPT_LIMIT, REGISTER_WINDOW);
    private final com.lingqiao.server.admin.LoginRateLimiter passwordLimiter =
        new com.lingqiao.server.admin.LoginRateLimiter(PASSWORD_ATTEMPT_LIMIT, PASSWORD_WINDOW);
    private final SecureRandom random = new SecureRandom();

    public AgentPortalAuthService(JdbcTemplate jdbc, Duration sessionTtl) {
        this(jdbc, sessionTtl, AgentSessionStore.inMemory());
    }

    public AgentPortalAuthService(
        JdbcTemplate jdbc,
        Duration sessionTtl,
        AgentSessionStore sessionStore
    ) {
        this.jdbc = jdbc;
        this.sessionTtl = sessionTtl == null || sessionTtl.isZero() || sessionTtl.isNegative()
            ? Duration.ofHours(4)
            : sessionTtl;
        this.sessionStore = sessionStore;
    }

    public LoginResult login(String username, String password, String clientIp) {
        if (!loginLimiter.allow("login_" + nullToEmpty(clientIp))) {
            throw new SidecarException(ErrorCode.TOO_MANY_REQUESTS, "登录尝试过于频繁，请15分钟后再试");
        }
        Map<String, Object> agent = findByUsername(username);
        if (agent == null) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "用户名或密码错误");
        }
        if (isDisabled(agent)) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "账号已被禁用，请联系管理员");
        }

        String storedHash = str(agent.get("password"));
        var check = com.lingqiao.server.admin.AdminLoginService.verifyPassword(password, storedHash);
        if (!check.match()) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "用户名或密码错误");
        }
        String agentId = str(agent.get("id"));
        if (!check.upgradedHash().isEmpty()) {
            jdbc.update("UPDATE agents SET password = ? WHERE id = ?", check.upgradedHash(), agentId);
        }

        loginLimiter.clear("login_" + nullToEmpty(clientIp));
        String token = TOKEN_PREFIX + HexFormat.of().formatHex(randomBytes(32));
        sessionStore.put(hashToken(token), agentId, Instant.now().plus(sessionTtl));
        return new LoginResult(token, agentId, str(agent.get("username")), (int) sessionTtl.toSeconds());
    }

    public String requireSession(String token) {
        if (token == null || token.length() < 7 || !token.startsWith(TOKEN_PREFIX)) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "unauthorized");
        }
        AgentSessionStore.StoredSession entry = sessionStore.find(hashToken(token), Instant.now());
        if (entry == null) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "unauthorized");
        }
        Map<String, Object> agent = findById(entry.agentId());
        if (agent == null || isDisabled(agent)) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "unauthorized");
        }
        return entry.agentId();
    }

    public void logout(String token) {
        if (token == null || token.isBlank()) {
            return;
        }
        sessionStore.remove(hashToken(token));
    }

    public boolean allowRegistration(String clientIp) {
        return registerLimiter.allow("reg_" + nullToEmpty(clientIp));
    }

    public AgentStats stats(String agentId) {
        Instant now = Instant.now();
        Integer total = jdbc.queryForObject(
            "SELECT COUNT(*) FROM cards WHERE agent_id = ?", Integer.class, agentId);
        Integer active = jdbc.queryForObject("""
            SELECT COUNT(*) FROM cards WHERE agent_id = ? AND status = 'active' AND expires_at > ?
            """, Integer.class, agentId, TimestampWriter.format(now));
        Integer expired = jdbc.queryForObject("""
            SELECT COUNT(*) FROM cards WHERE agent_id = ? AND (status = 'expired' OR expires_at < ?)
            """, Integer.class, agentId, TimestampWriter.format(now));
        return new AgentStats(zero(total), zero(active), zero(expired));
    }

    public AgentProfile profile(String agentId) {
        Map<String, Object> agent = findById(agentId);
        if (agent == null) {
            throw new SidecarException(ErrorCode.AGENT_NOT_FOUND, "agent not found");
        }
        return new AgentProfile(
            str(agent.get("id")),
            str(agent.get("username")),
            str(agent.get("prefix")),
            instant(agent.get("created_at"))
        );
    }

    public void changePassword(String agentId, String oldPassword, String newPassword) {
        changePasswordInternal(agentId, oldPassword, newPassword);
    }

    public void changePasswordFromIp(
        String agentId,
        String oldPassword,
        String newPassword,
        String clientIp
    ) {
        if (!passwordLimiter.allow("pwchg_" + nullToEmpty(clientIp))) {
            throw new SidecarException(ErrorCode.TOO_MANY_REQUESTS, "密码修改过于频繁，请稍后再试");
        }
        changePasswordInternal(agentId, oldPassword, newPassword);
    }

    private void changePasswordInternal(String agentId, String oldPassword, String newPassword) {
        int passwordBytes = newPassword == null
            ? 0
            : newPassword.getBytes(StandardCharsets.UTF_8).length;
        if (passwordBytes < 6 || passwordBytes > 72) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "密码长度必须在6-72位之间");
        }
        Map<String, Object> agent = findById(agentId);
        if (agent == null) {
            throw new SidecarException(ErrorCode.AGENT_NOT_FOUND, "agent not found");
        }
        var check = com.lingqiao.server.admin.AdminLoginService.verifyPassword(
            oldPassword, str(agent.get("password")));
        if (!check.match()) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "旧密码错误");
        }
        jdbc.update("UPDATE agents SET password = ? WHERE id = ?",
            BCrypt.hashpw(newPassword, BCrypt.gensalt()), agentId);
    }

    private Map<String, Object> findByUsername(String username) {
        List<Map<String, Object>> rows = jdbc.queryForList(
            "SELECT id, username, password, disabled FROM agents WHERE username = ?",
            nullToEmpty(username));
        return rows.isEmpty() ? null : rows.get(0);
    }

    private Map<String, Object> findById(String agentId) {
        List<Map<String, Object>> rows = jdbc.queryForList(
            "SELECT id, username, password, prefix, created_at, disabled FROM agents WHERE id = ?",
            nullToEmpty(agentId));
        return rows.isEmpty() ? null : rows.get(0);
    }

    private static boolean isDisabled(Map<String, Object> agent) {
        Object disabled = agent.get("disabled");
        if (disabled instanceof Number number) {
            return number.intValue() != 0;
        }
        return disabled instanceof Boolean flag && flag;
    }

    static String hashToken(String token) {
        try {
            return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256")
                .digest(token.getBytes(StandardCharsets.UTF_8)));
        } catch (Exception ex) {
            throw new IllegalStateException("sha256 unavailable", ex);
        }
    }

    private byte[] randomBytes(int size) {
        byte[] value = new byte[size];
        random.nextBytes(value);
        return value;
    }

    private static int zero(Integer value) {
        return value == null ? 0 : value;
    }

    private static String str(Object value) {
        return value == null ? "" : String.valueOf(value);
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }

    private static Instant instant(Object value) {
        if (value instanceof Instant instant) {
            return instant;
        }
        if (value instanceof Timestamp timestamp) {
            return timestamp.toInstant();
        }
        if (value == null || String.valueOf(value).isBlank()) {
            return Instant.EPOCH;
        }
        try {
            return Instant.parse(String.valueOf(value));
        } catch (DateTimeParseException ex) {
            return Instant.EPOCH;
        }
    }

    public record LoginResult(String token, String agentId, String username, int maxAgeSeconds) {
    }

    public record AgentStats(int totalCards, int activeCards, int expiredCards) {
    }

    public record AgentProfile(String id, String username, String prefix, Instant createdAt) {
    }
}
