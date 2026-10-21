package com.lingqiao.server.common.security;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.time.Instant;
import java.util.HexFormat;
import java.util.Map;
import java.util.Set;
import org.springframework.jdbc.core.JdbcTemplate;

public class AdminAuthService {
    private static final Map<String, Set<String>> ROLE_PERMISSIONS = Map.of(
        "owner", Set.of(
            "cards.read", "cards.write", "cards.bulk",
            "sessions.read", "sessions.force_logout",
            "agents.read", "agents.write",
            "invites.write",
            "releases.read", "releases.write",
            "scripts.write", "payloads.write",
            "announcement.read", "announcement.write",
            "chat.read", "chat.write",
            "audit.read", "system.read", "admins.write",
            "exam.read"
        ),
        "operator", Set.of(
            "cards.read", "cards.write", "cards.bulk",
            "sessions.read", "sessions.force_logout",
            "audit.read", "exam.read"
        ),
        "support", Set.of("cards.read", "sessions.read", "sessions.force_logout"),
        "release_manager", Set.of("releases.read", "releases.write", "scripts.write", "payloads.write"),
        "agent_manager", Set.of("agents.read", "agents.write", "invites.write")
    );

    private final JdbcTemplate jdbc;

    public AdminAuthService(JdbcTemplate jdbc) {
        this.jdbc = jdbc;
    }

    public AdminPrincipal requireAdmin(String token, String permission) {
        if (token == null || token.isBlank()) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "unauthorized");
        }
        String tokenHash = sha256Hex(token);
        Map<String, Object> row;
        try {
            row = jdbc.queryForMap("""
                SELECT s.expires_at, u.id, u.username, u.role, u.disabled
                FROM admin_sessions s
                JOIN admin_users u ON u.id = s.admin_id
                WHERE s.token_hash = ?
                """, tokenHash);
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "unauthorized");
        }

        Instant expiresAt = Instant.parse(String.valueOf(row.get("expires_at")));
        if (Instant.now().isAfter(expiresAt)) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "unauthorized");
        }
        Object disabled = row.get("disabled");
        if (disabled instanceof Number n && n.intValue() != 0) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "unauthorized");
        }
        if (disabled instanceof Boolean b && b) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "unauthorized");
        }

        String role = String.valueOf(row.get("role"));
        Set<String> permissions = ROLE_PERMISSIONS.getOrDefault(role, Set.of());
        if (permission != null && !permission.isBlank() && !permissions.contains(permission)) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "forbidden: requires permission " + permission);
        }
        return new AdminPrincipal(
            String.valueOf(row.get("id")),
            String.valueOf(row.get("username")),
            role,
            permissions
        );
    }

    private static String sha256Hex(String value) {
        try {
            MessageDigest digest = MessageDigest.getInstance("SHA-256");
            return HexFormat.of().formatHex(digest.digest(value.getBytes(StandardCharsets.UTF_8)));
        } catch (Exception ex) {
            throw new IllegalStateException("sha256 unavailable", ex);
        }
    }
}
