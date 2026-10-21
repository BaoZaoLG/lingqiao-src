package com.lingqiao.server.admin;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.errors.ErrorCode;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.sql.DriverManager;
import java.sql.Timestamp;
import java.time.Duration;
import java.time.Instant;
import java.util.HexFormat;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;
import org.springframework.security.crypto.bcrypt.BCrypt;

class AdminLoginServiceTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private AdminLoginService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE admin_users (
              id TEXT PRIMARY KEY,
              username TEXT NOT NULL UNIQUE,
              password_hash TEXT NOT NULL,
              role TEXT NOT NULL,
              disabled INTEGER NOT NULL DEFAULT 0,
              last_login_at DATETIME
            )
            """);
        jdbc.execute("""
            CREATE TABLE admin_sessions (
              token_hash TEXT PRIMARY KEY,
              admin_id TEXT NOT NULL,
              created_at DATETIME NOT NULL,
              expires_at DATETIME NOT NULL,
              remote_addr TEXT NOT NULL DEFAULT '',
              user_agent TEXT NOT NULL DEFAULT ''
            )
            """);
        service = new AdminLoginService(jdbc, Duration.ofHours(8));
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void bcryptLoginCreatesHashedSessionToken() {
        insertAdmin("a1", "admin", BCrypt.hashpw("secret123", BCrypt.gensalt()), "owner", false);

        AdminLoginService.LoginResult result = service.login("admin", "secret123", "10.0.0.1", "agent");

        assertThat(result.token()).startsWith("admin_");
        assertThat(result.role()).isEqualTo("owner");
        assertThat(result.maxAgeSeconds()).isEqualTo(28800);

        String storedHash = jdbc.queryForObject(
            "SELECT token_hash FROM admin_sessions WHERE admin_id = 'a1'", String.class);
        assertThat(storedHash).isEqualTo(sha256Hex(result.token()));
        assertThat(storedHash).isNotEqualTo(result.token());
        assertThat(jdbc.queryForObject(
            "SELECT remote_addr FROM admin_sessions WHERE admin_id='a1'", String.class)).isEqualTo("10.0.0.1");
    }

    @Test
    void legacySha256PasswordIsUpgradedToBcrypt() {
        insertAdmin("a2", "legacy", sha256Hex("oldpass"), "operator", false);

        service.login("legacy", "oldpass", "10.0.0.2", "agent");

        String stored = jdbc.queryForObject(
            "SELECT password_hash FROM admin_users WHERE id='a2'", String.class);
        assertThat(AdminLoginService.isBcryptHash(stored)).isTrue();
        assertThat(BCrypt.checkpw("oldpass", stored)).isTrue();
    }

    @Test
    void emptyUsernameFallsBackToDefaultOwner() {
        insertAdmin("a3", AdminLoginService.DEFAULT_OWNER_USERNAME,
            BCrypt.hashpw("pw", BCrypt.gensalt()), "owner", false);

        assertThat(service.login("", "pw", "10.0.0.3", "agent").username())
            .isEqualTo(AdminLoginService.DEFAULT_OWNER_USERNAME);
    }

    @Test
    void whitespaceUsernameIsNotTreatedAsEmptyLikeGo() {
        insertAdmin("a-space", AdminLoginService.DEFAULT_OWNER_USERNAME,
            BCrypt.hashpw("pw", BCrypt.gensalt()), "owner", false);

        assertThatThrownBy(() -> service.login(" ", "pw", "10.0.0.30", "agent"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("wrong username or password");
    }

    @Test
    void wrongPasswordDisabledAccountAndUnknownUserAreRejected() {
        insertAdmin("a4", "user4", BCrypt.hashpw("right", BCrypt.gensalt()), "operator", false);
        insertAdmin("a5", "user5", BCrypt.hashpw("right", BCrypt.gensalt()), "operator", true);

        assertThatThrownBy(() -> service.login("user4", "wrong", "10.0.0.4", ""))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("wrong username or password");
        assertThatThrownBy(() -> service.login("user5", "right", "10.0.0.5", ""))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("account disabled");
        assertThatThrownBy(() -> service.login("ghost", "right", "10.0.0.6", ""))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("wrong username or password");
        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM admin_sessions", Integer.class)).isZero();
    }

    @Test
    void loginRateLimitBlocksAfterFiveFailuresPerIp() {
        insertAdmin("a6", "user6", BCrypt.hashpw("right", BCrypt.gensalt()), "operator", false);

        for (int i = 0; i < AdminLoginService.LOGIN_ATTEMPT_LIMIT; i++) {
            assertThatThrownBy(() -> service.login("user6", "wrong", "10.0.0.7", ""))
                .hasMessageContaining("wrong username or password");
        }

        assertThatThrownBy(() -> service.login("user6", "right", "10.0.0.7", ""))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("登录尝试过于频繁");

        assertThat(service.login("user6", "right", "10.0.0.8", "").token()).startsWith("admin_");
    }

    @Test
    void successfulLoginClearsRateLimitCounter() {
        insertAdmin("a7", "user7", BCrypt.hashpw("right", BCrypt.gensalt()), "operator", false);

        for (int i = 0; i < AdminLoginService.LOGIN_ATTEMPT_LIMIT - 1; i++) {
            assertThatThrownBy(() -> service.login("user7", "wrong", "10.0.0.9", ""))
                .hasMessageContaining("wrong username or password");
        }
        service.login("user7", "right", "10.0.0.9", "");

        for (int i = 0; i < AdminLoginService.LOGIN_ATTEMPT_LIMIT - 1; i++) {
            assertThatThrownBy(() -> service.login("user7", "wrong", "10.0.0.9", ""))
                .hasMessageContaining("wrong username or password");
        }
    }

    @Test
    void logoutRemovesSessionByHashedToken() {
        insertAdmin("a8", "user8", BCrypt.hashpw("pw", BCrypt.gensalt()), "owner", false);
        String token = service.login("user8", "pw", "10.0.0.10", "").token();

        service.logout(token);

        assertThat(jdbc.queryForObject("SELECT COUNT(*) FROM admin_sessions", Integer.class)).isZero();
        service.logout("");
        service.logout(null);
    }

    @Test
    void malformedStoredHashDoesNotThrow() {
        insertAdmin("a9", "user9", "$2a$not-a-real-hash", "owner", false);

        assertThatThrownBy(() -> service.login("user9", "pw", "10.0.0.11", ""))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("wrong username or password");
    }

    @Test
    void defaultSessionTtlMatchesGoFourHours() {
        insertAdmin("a10", "user10", BCrypt.hashpw("pw", BCrypt.gensalt()), "owner", false);
        AdminLoginService defaultTtlService = new AdminLoginService(jdbc, null);

        assertThat(defaultTtlService.login("user10", "pw", "10.0.0.12", "").maxAgeSeconds())
            .isEqualTo(4 * 60 * 60);
    }

    @Test
    void passwordChangeValidatesOldPasswordAndUtf8ByteLength() {
        insertAdmin("a11", "user11", BCrypt.hashpw("old-pass", BCrypt.gensalt()), "owner", false);

        assertThatThrownBy(() ->
            service.changePassword("a11", "wrong", "new-pass", "10.0.0.13"))
            .isInstanceOfSatisfying(SidecarException.class, ex -> {
                assertThat(ex.code()).isEqualTo(ErrorCode.BAD_REQUEST);
                assertThat(ex).hasMessageContaining("旧密码错误");
            });

        service.changePassword("a11", "old-pass", "密码", "10.0.0.14");

        String stored = jdbc.queryForObject(
            "SELECT password_hash FROM admin_users WHERE id = 'a11'", String.class);
        assertThat(BCrypt.checkpw("密码", stored)).isTrue();
    }

    @Test
    void passwordChangeRateLimitsAfterTenAttempts() {
        insertAdmin("a12", "user12", BCrypt.hashpw("old-pass", BCrypt.gensalt()), "owner", false);

        for (int i = 0; i < AdminLoginService.PASSWORD_ATTEMPT_LIMIT; i++) {
            assertThatThrownBy(() ->
                service.changePassword("a12", "wrong", "new-pass", "10.0.0.15"))
                .hasMessageContaining("旧密码错误");
        }
        assertThatThrownBy(() ->
            service.changePassword("a12", "old-pass", "new-pass", "10.0.0.15"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("密码修改过于频繁");
    }

    private void insertAdmin(String id, String username, String hash, String role, boolean disabled) {
        jdbc.update("""
            INSERT INTO admin_users (id, username, password_hash, role, disabled, last_login_at)
            VALUES (?,?,?,?,?,?)
            """, id, username, hash, role, disabled ? 1 : 0, Timestamp.from(Instant.now()));
    }

    private static String sha256Hex(String value) {
        try {
            return HexFormat.of().formatHex(
                MessageDigest.getInstance("SHA-256").digest(value.getBytes(StandardCharsets.UTF_8)));
        } catch (Exception ex) {
            throw new AssertionError(ex);
        }
    }
}
