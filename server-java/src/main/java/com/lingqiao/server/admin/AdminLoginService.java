package com.lingqiao.server.admin;

import com.lingqiao.server.common.jdbc.TimestampWriter;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.SecureRandom;
import java.sql.Timestamp;
import java.time.Duration;
import java.time.Instant;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.security.crypto.bcrypt.BCrypt;
import org.springframework.transaction.support.TransactionTemplate;

/**
 * Admin authentication mirroring Go admin.Service:
 * bcrypt-with-legacy-sha256 password verification, opaque session tokens stored
 * as SHA-256 hashes, and login rate limiting per client IP.
 */
public class AdminLoginService {
    public static final String DEFAULT_OWNER_USERNAME = "admin";
    static final int LOGIN_ATTEMPT_LIMIT = 5;
    static final Duration LOGIN_ATTEMPT_WINDOW = Duration.ofMinutes(15);
    static final int PASSWORD_ATTEMPT_LIMIT = 10;
    static final Duration PASSWORD_ATTEMPT_WINDOW = Duration.ofHours(1);

    private final JdbcTemplate jdbc;
    private final TransactionTemplate transactions;
    private final Duration sessionTtl;
    private final LoginRateLimiter limiter = new LoginRateLimiter(LOGIN_ATTEMPT_LIMIT, LOGIN_ATTEMPT_WINDOW);
    private final LoginRateLimiter passwordLimiter =
        new LoginRateLimiter(PASSWORD_ATTEMPT_LIMIT, PASSWORD_ATTEMPT_WINDOW);
    private final SecureRandom random = new SecureRandom();

    public AdminLoginService(JdbcTemplate jdbc, Duration sessionTtl) {
        this.jdbc = jdbc;
        this.sessionTtl = sessionTtl == null || sessionTtl.isZero() || sessionTtl.isNegative()
            ? Duration.ofHours(4)
            : sessionTtl;
        this.transactions = new TransactionTemplate(new DataSourceTransactionManager(
            Objects.requireNonNull(jdbc.getDataSource(), "write datasource is required")));
    }

    public LoginResult login(String username, String password, String clientIp, String userAgent) {
        if (!limiter.allow(clientIp)) {
            throw new SidecarException(ErrorCode.TOO_MANY_REQUESTS, "登录尝试过于频繁，请15分钟后再试");
        }
        String effectiveUsername = username == null || username.isEmpty()
            ? DEFAULT_OWNER_USERNAME
            : username;

        return transactions.execute(status -> {
            Map<String, Object> admin = loadByUsername(effectiveUsername);
            if (isDisabled(admin)) {
                throw new SidecarException(ErrorCode.UNAUTHORIZED, "account disabled");
            }

            String storedHash = str(admin.get("password_hash"));
            PasswordCheck check = verifyPassword(password, storedHash);
            if (!check.match()) {
                throw new SidecarException(ErrorCode.UNAUTHORIZED, "wrong username or password");
            }

            String adminId = str(admin.get("id"));
            if (!check.upgradedHash().isEmpty()) {
                jdbc.update("UPDATE admin_users SET password_hash = ? WHERE id = ?",
                    check.upgradedHash(), adminId);
            }

            Instant now = Instant.now();
            jdbc.update("UPDATE admin_users SET last_login_at = ? WHERE id = ?",
                TimestampWriter.format(now), adminId);

            limiter.clear(clientIp);

            String token = "admin_" + HexFormat.of().formatHex(randomBytes(32));
            jdbc.update("""
                INSERT INTO admin_sessions (token_hash, admin_id, created_at, expires_at, remote_addr, user_agent)
                VALUES (?,?,?,?,?,?)
                """,
                hashToken(token), adminId, TimestampWriter.format(now),
                TimestampWriter.format(now.plus(sessionTtl)),
                nullToEmpty(clientIp), nullToEmpty(userAgent));

            return new LoginResult(token, (int) sessionTtl.toSeconds(),
                adminId, str(admin.get("username")), str(admin.get("role")));
        });
    }

    public void logout(String token) {
        if (token == null || token.isBlank()) {
            return;
        }
        jdbc.update("DELETE FROM admin_sessions WHERE token_hash = ?", hashToken(token));
    }

    public void changePassword(
        String adminId,
        String oldPassword,
        String newPassword,
        String clientIp
    ) {
        if (!passwordLimiter.allow("pwchg_" + nullToEmpty(clientIp))) {
            throw new SidecarException(ErrorCode.TOO_MANY_REQUESTS, "密码修改过于频繁，请稍后再试");
        }
        int passwordBytes = newPassword == null
            ? 0
            : newPassword.getBytes(StandardCharsets.UTF_8).length;
        if (passwordBytes < 6 || passwordBytes > 72) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "新密码长度必须在6-72字节之间");
        }
        List<Map<String, Object>> rows = jdbc.queryForList(
            "SELECT password_hash FROM admin_users WHERE id = ?", nullToEmpty(adminId));
        if (rows.isEmpty()) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "unauthorized");
        }
        PasswordCheck check = verifyPassword(oldPassword, str(rows.get(0).get("password_hash")));
        if (!check.match()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "旧密码错误");
        }
        jdbc.update(
            "UPDATE admin_users SET password_hash = ? WHERE id = ?",
            BCrypt.hashpw(newPassword, BCrypt.gensalt()),
            adminId
        );
    }

    public static PasswordCheck verifyPassword(String password, String storedHash) {        String candidate = password == null ? "" : password;
        String stored = storedHash == null ? "" : storedHash;
        if (isBcryptHash(stored)) {
            return new PasswordCheck(safeBcryptCheck(candidate, stored), "");
        }
        String legacy = sha256Hex(candidate);
        if (!MessageDigest.isEqual(
            legacy.getBytes(StandardCharsets.UTF_8), stored.getBytes(StandardCharsets.UTF_8))) {
            return new PasswordCheck(false, "");
        }
        return new PasswordCheck(true, BCrypt.hashpw(candidate, BCrypt.gensalt()));
    }

    public static boolean isBcryptHash(String hash) {
        return hash != null && (hash.startsWith("$2a$") || hash.startsWith("$2b$") || hash.startsWith("$2y$"));
    }

    static String hashToken(String token) {
        return sha256Hex(token);
    }

    private Map<String, Object> loadByUsername(String username) {
        List<Map<String, Object>> rows = jdbc.queryForList(
            "SELECT id, username, password_hash, role, disabled FROM admin_users WHERE username = ?", username);
        if (rows.isEmpty()) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "wrong username or password");
        }
        return rows.get(0);
    }

    private static boolean isDisabled(Map<String, Object> admin) {
        Object disabled = admin.get("disabled");
        if (disabled instanceof Number number) {
            return number.intValue() != 0;
        }
        return disabled instanceof Boolean flag && flag;
    }

    private static boolean safeBcryptCheck(String password, String hash) {
        try {
            return BCrypt.checkpw(password, hash);
        } catch (IllegalArgumentException ex) {
            return false;
        }
    }

    private static String sha256Hex(String value) {
        try {
            return HexFormat.of().formatHex(
                MessageDigest.getInstance("SHA-256").digest(value.getBytes(StandardCharsets.UTF_8)));
        } catch (Exception ex) {
            throw new IllegalStateException("sha256 unavailable", ex);
        }
    }

    private byte[] randomBytes(int size) {
        byte[] value = new byte[size];
        random.nextBytes(value);
        return value;
    }

    private static String str(Object value) {
        return value == null ? "" : String.valueOf(value);
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }

    public record PasswordCheck(boolean match, String upgradedHash) {
    }

    public record LoginResult(String token, int maxAgeSeconds, String adminId, String username, String role) {
    }
}
