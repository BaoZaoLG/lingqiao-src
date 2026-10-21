package com.lingqiao.server.system;

import com.lingqiao.server.common.jdbc.TimestampReader;
import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.sql.Timestamp;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/admin/api/blacklist")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "blacklist-enabled", havingValue = "true")
public class BlacklistReadController {
    private final JdbcTemplate jdbc;
    private final AdminAuthService adminAuthService;

    public BlacklistReadController(JdbcTemplate jdbc, AdminAuthService adminAuthService) {
        this.jdbc = jdbc;
        this.adminAuthService = adminAuthService;
    }

    @GetMapping
    public Map<String, Object> list(HttpServletRequest request) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "system.read");
        List<BlacklistEntry> entries = jdbc.query("""
            SELECT type, value, reason, created_at FROM blacklist ORDER BY created_at DESC
            """, (rs, rowNum) -> new BlacklistEntry(
                rs.getString("type"), rs.getString("value"), rs.getString("reason"),
                TimestampReader.readOrEpoch(rs, "created_at")));
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("entries", entries.isEmpty() ? null : entries);
        return CompatResponse.ok(body);
    }
}
