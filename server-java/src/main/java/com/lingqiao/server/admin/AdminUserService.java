package com.lingqiao.server.admin;

import com.fasterxml.jackson.annotation.JsonProperty;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.jdbc.TimestampReader;
import com.lingqiao.server.common.jdbc.TimestampWriter;
import java.nio.charset.StandardCharsets;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.time.Instant;
import java.util.List;
import java.util.Set;
import java.util.UUID;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.security.crypto.bcrypt.BCrypt;
import org.springframework.transaction.annotation.Transactional;

/** Admin-user CRUD with Go's role and last-active-owner invariants. */
public class AdminUserService {
    private static final Set<String> ROLES = Set.of(
        "owner", "operator", "support", "release_manager", "agent_manager");

    private final JdbcTemplate jdbc;

    public AdminUserService(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    public List<AdminUserView> list() {
        return jdbc.query("""
            SELECT id, username, role, disabled, created_at, last_login_at
            FROM admin_users ORDER BY created_at ASC
            """, (rs, rowNum) -> view(rs));
    }

    @Transactional
    public AdminUserView create(String rawUsername, String password, String role, boolean disabled) {
        String username = rawUsername == null ? "" : rawUsername.trim();
        if (username.isEmpty() || password == null || password.isEmpty() || role == null || role.isEmpty()) {
            throw new SidecarException(
                ErrorCode.BAD_REQUEST, "username, password, and role are required");
        }
        requireRole(role);
        requirePassword(password, "password length must be between 6 and 72 bytes");
        String id = UUID.randomUUID().toString();
        Instant createdAt = Instant.now();
        jdbc.update("""
            INSERT INTO admin_users (
              id, username, password_hash, role, disabled, created_at, last_login_at
            ) VALUES (?, ?, ?, ?, ?, ?, NULL)
            """,
            id, username, BCrypt.hashpw(password, BCrypt.gensalt()), role, disabled ? 1 : 0,
            TimestampWriter.format(createdAt)
        );
        return new AdminUserView(id, username, role, disabled, createdAt, null);
    }

    @Transactional
    public void update(UpdateInput input) {
        if (input == null || input.id() == null || input.id().isBlank()) {
            throw new SidecarException(ErrorCode.NOT_FOUND, "Admin not found");
        }
        List<AdminRow> rows = jdbc.query("""
            SELECT id, username, password_hash, role, disabled, created_at, last_login_at
            FROM admin_users WHERE id = ?
            """, (rs, rowNum) -> row(rs), input.id());
        if (rows.isEmpty()) {
            throw new SidecarException(ErrorCode.NOT_FOUND, "Admin not found");
        }
        AdminRow current = rows.get(0);
        String passwordHash = current.passwordHash();
        String role = current.role();
        boolean disabled = current.disabled();

        if (input.changePassword()) {
            requirePassword(input.newPassword(), "password length must be between 6 and 72 bytes");
            passwordHash = BCrypt.hashpw(input.newPassword(), BCrypt.gensalt());
        }
        if (input.changeRole()) {
            requireRole(input.role());
            role = input.role();
        }
        if (input.setDisabled()) {
            disabled = input.disabled();
        }
        if ("owner".equals(current.role()) && !current.disabled()
            && (!"owner".equals(role) || disabled)) {
            Integer activeOwners = jdbc.queryForObject("""
                SELECT COUNT(*) FROM admin_users WHERE role = 'owner' AND disabled = 0
                """, Integer.class);
            if (activeOwners == null || activeOwners <= 1) {
                throw new SidecarException(
                    ErrorCode.BAD_REQUEST, "cannot disable or demote the last active owner");
            }
        }
        jdbc.update("""
            UPDATE admin_users
            SET username = ?, password_hash = ?, role = ?, disabled = ?, last_login_at = ?
            WHERE id = ?
            """,
            current.username(), passwordHash, role, disabled ? 1 : 0,
            current.lastLoginAt() == null ? null : TimestampWriter.format(current.lastLoginAt()),
            current.id()
        );
    }

    private static void requireRole(String role) {
        if (!ROLES.contains(role)) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "invalid admin role");
        }
    }

    private static void requirePassword(String password, String message) {
        int bytes = password == null ? 0 : password.getBytes(StandardCharsets.UTF_8).length;
        if (bytes < 6 || bytes > 72) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, message);
        }
    }

    private static AdminUserView view(ResultSet rs) throws SQLException {
        return new AdminUserView(
            rs.getString("id"),
            rs.getString("username"),
            rs.getString("role"),
            rs.getInt("disabled") != 0,
            TimestampReader.readOrEpoch(rs, "created_at"),
            TimestampReader.read(rs, "last_login_at")
        );
    }

    private static AdminRow row(ResultSet rs) throws SQLException {
        return new AdminRow(
            rs.getString("id"),
            rs.getString("username"),
            rs.getString("password_hash"),
            rs.getString("role"),
            rs.getInt("disabled") != 0,
            TimestampReader.readOrEpoch(rs, "created_at"),
            TimestampReader.read(rs, "last_login_at")
        );
    }

    public record AdminUserView(
        String id,
        String username,
        String role,
        boolean disabled,
        @JsonProperty("created_at") Instant createdAt,
        @JsonProperty("last_login_at") Instant lastLoginAt
    ) {
    }

    public record UpdateInput(
        String id,
        boolean changePassword,
        String newPassword,
        boolean changeRole,
        String role,
        boolean setDisabled,
        boolean disabled
    ) {
    }

    private record AdminRow(
        String id,
        String username,
        String passwordHash,
        String role,
        boolean disabled,
        Instant createdAt,
        Instant lastLoginAt
    ) {
    }
}
