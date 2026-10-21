package com.lingqiao.server.admin;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.jdbc.TimestampWriter;
import java.sql.DriverManager;
import java.time.Instant;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.SingleConnectionDataSource;
import org.springframework.security.crypto.bcrypt.BCrypt;

class AdminUserServiceTest {
    private SingleConnectionDataSource dataSource;
    private JdbcTemplate jdbc;
    private AdminUserService service;

    @BeforeEach
    void setUp() throws Exception {
        dataSource = new SingleConnectionDataSource(
            DriverManager.getConnection("jdbc:sqlite::memory:"), true);
        jdbc = new JdbcTemplate(dataSource);
        jdbc.execute("""
            CREATE TABLE admin_users (
              id TEXT PRIMARY KEY,
              username TEXT NOT NULL UNIQUE,
              password_hash TEXT NOT NULL,
              role TEXT NOT NULL,
              disabled INTEGER NOT NULL DEFAULT 0,
              created_at TEXT NOT NULL,
              last_login_at TEXT
            )
            """);
        service = new AdminUserService(jdbc);
    }

    @AfterEach
    void tearDown() {
        if (dataSource != null) {
            dataSource.destroy();
        }
    }

    @Test
    void createAndListNeverExposePasswordHash() {
        AdminUserService.AdminUserView created =
            service.create("  alice  ", "secret123", "operator", false);

        assertThat(created.username()).isEqualTo("alice");
        assertThat(created.role()).isEqualTo("operator");
        assertThat(service.list()).containsExactly(created);
        String storedHash = jdbc.queryForObject(
            "SELECT password_hash FROM admin_users WHERE id = ?", String.class, created.id());
        assertThat(BCrypt.checkpw("secret123", storedHash)).isTrue();
        assertThat(AdminUserService.AdminUserView.class.getRecordComponents())
            .extracting(component -> component.getName())
            .doesNotContain("passwordHash", "password_hash");
    }

    @Test
    void createValidatesRequiredFieldsRoleAndUtf8PasswordBytes() {
        assertThatThrownBy(() -> service.create("", "secret123", "operator", false))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("required");
        assertThatThrownBy(() -> service.create("alice", "secret123", "invalid", false))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid admin role");

        assertThat(service.create("unicode", "密码", "support", false).username())
            .isEqualTo("unicode");
    }

    @Test
    void updateChangesSelectedFieldsAndPreservesOthers() {
        insertAdmin("owner-1", "owner", "old-pass", "owner", false);
        insertAdmin("owner-2", "owner2", "owner-pass", "owner", false);

        service.update(new AdminUserService.UpdateInput(
            "owner-1", true, "new-pass", true, "support", true, true));

        var updated = service.list().stream()
            .filter(admin -> admin.id().equals("owner-1"))
            .findFirst()
            .orElseThrow();
        assertThat(updated.username()).isEqualTo("owner");
        assertThat(updated.role()).isEqualTo("support");
        assertThat(updated.disabled()).isTrue();
        String hash = jdbc.queryForObject(
            "SELECT password_hash FROM admin_users WHERE id = 'owner-1'", String.class);
        assertThat(BCrypt.checkpw("new-pass", hash)).isTrue();
    }

    @Test
    void updateProtectsLastActiveOwner() {
        insertAdmin("owner-only", "owner", "old-pass", "owner", false);

        assertThatThrownBy(() -> service.update(new AdminUserService.UpdateInput(
            "owner-only", false, "", true, "operator", false, false)))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("last active owner");
    }

    @Test
    void updateRejectsUnknownAdminAndInvalidPassword() {
        assertThatThrownBy(() -> service.update(new AdminUserService.UpdateInput(
            "missing", false, "", false, "", false, false)))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("Admin not found");

        insertAdmin("admin-1", "admin1", "old-pass", "operator", false);
        assertThatThrownBy(() -> service.update(new AdminUserService.UpdateInput(
            "admin-1", true, "short", false, "", false, false)))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("between 6 and 72 bytes");
    }

    private void insertAdmin(
        String id,
        String username,
        String password,
        String role,
        boolean disabled
    ) {
        jdbc.update("""
            INSERT INTO admin_users (
              id, username, password_hash, role, disabled, created_at, last_login_at
            ) VALUES (?, ?, ?, ?, ?, ?, NULL)
            """,
            id, username, BCrypt.hashpw(password, BCrypt.gensalt()), role, disabled ? 1 : 0,
            TimestampWriter.format(Instant.now())
        );
    }
}
