package com.lingqiao.server.agents;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

/**
 * Admin agent read endpoints mirroring Go GET /admin/api/agents.
 * Mutating operations live in AgentsWriteController behind the write flag.
 */
@RestController
@RequestMapping("/java/admin/api/agents")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "agents-enabled", havingValue = "true")
public class AgentsController {
    private final AgentsService service;
    private final AdminAuthService adminAuthService;

    public AgentsController(AgentsService service, AdminAuthService adminAuthService) {
        this.service = service;
        this.adminAuthService = adminAuthService;
    }

    @GetMapping
    public Object list(
        HttpServletRequest request,
        HttpServletResponse response,
        @RequestParam(name = "export", required = false) String export
    ) throws IOException {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "agents.read");
        List<Agent> agents = service.list();
        if ("csv".equals(export)) {
            response.setContentType("text/csv; charset=utf-8");
            response.setHeader("Content-Disposition", "attachment; filename=agents_export.csv");
            response.getOutputStream().write(AgentsCsv.export(agents));
            return null;
        }
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("agents", agents);
        return CompatResponse.ok(body);
    }

    @GetMapping("/detail")
    public Map<String, Object> detail(HttpServletRequest request, @RequestParam("agent_id") String agentId) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "agents.read");
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("agent", service.get(agentId));
        return CompatResponse.ok(body);
    }
}
