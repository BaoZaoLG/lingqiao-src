package com.lingqiao.server.admin;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.concurrent.atomic.AtomicLong;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Admin dashboard endpoints mirroring Go /admin/api/dashboard,
 * /admin/api/stats and /admin/api/ops/overview.
 */
@RestController
@RequestMapping("/java/admin/api")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "admin-overview-enabled", havingValue = "true")
public class AdminOverviewController {
    private final AdminOverviewService overview;
    private final AdminAuthService adminAuthService;
    private final AtomicLong requestCount;

    public AdminOverviewController(
        AdminOverviewService overview,
        AdminAuthService adminAuthService,
        AtomicLong requestCount
    ) {
        this.overview = overview;
        this.adminAuthService = adminAuthService;
        this.requestCount = requestCount;
    }

    @GetMapping("/dashboard")
    public Map<String, Object> dashboard(HttpServletRequest request) {
        requireRead(request);
        AdminOverviewService.DashboardStats stats = overview.dashboard();

        Map<String, Object> body = new LinkedHashMap<>();
        body.put("total_cards", stats.totalCards());
        body.put("active_cards", stats.activeCards());
        body.put("expired_cards", stats.expiredCards());
        body.put("active_sessions", stats.activeSessions());
        return CompatResponse.ok(body);
    }

    @GetMapping({"/stats", "/server-stats"})
    public Map<String, Object> serverStats(HttpServletRequest request) {
        requireRead(request);
        AdminOverviewService.ServerStats stats = overview.serverStats(requestCount.get());

        Map<String, Object> body = new LinkedHashMap<>();
        body.put("uptime_seconds", stats.uptimeSeconds());
        body.put("total_cards", stats.totalCards());
        body.put("active_sessions", stats.activeSessions());
        body.put("request_count", stats.requestCount());
        return CompatResponse.ok(body);
    }

    @GetMapping({"/ops/overview", "/ops-overview"})
    public Map<String, Object> opsOverview(HttpServletRequest request) {
        requireRead(request);
        AdminOverviewService.OpsOverview ops = overview.opsOverview();

        Map<String, Object> body = new LinkedHashMap<>();
        body.put("today_activations", ops.todayActivations());
        body.put("today_new_machines", ops.newMachines());
        body.put("active_sessions", ops.activeSessions());
        body.put("expiring_cards", ops.expiringCards());
        body.put("risky_machines", ops.riskyMachines());
        body.put("agent_leaderboard", ops.agentLeaderboard());
        body.put("recent_audit", ops.recentAudit());
        return CompatResponse.ok(body);
    }

    @GetMapping("/modules/overview")
    public Map<String, Object> modulesOverview(HttpServletRequest request) {
        requireRead(request);
        AdminOverviewService.ModulesOverview result = overview.modulesOverview();
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("generated_at", result.generatedAt());
        body.put("modules", result.modules());
        return CompatResponse.ok(body);
    }

    private void requireRead(HttpServletRequest request) {
        adminAuthService.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "");
    }
}
