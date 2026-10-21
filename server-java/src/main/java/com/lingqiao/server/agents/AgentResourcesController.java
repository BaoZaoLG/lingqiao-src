package com.lingqiao.server.agents;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/admin/api")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "agents-enabled", havingValue = "true")
public class AgentResourcesController {
    private final AgentResourcesService service;
    private final AdminAuthService adminAuthService;

    public AgentResourcesController(
        AgentResourcesService service, AdminAuthService adminAuthService
    ) {
        this.service = service;
        this.adminAuthService = adminAuthService;
    }

    @GetMapping("/machines")
    public Map<String, Object> machines(HttpServletRequest request) {
        requireRead(request);
        return response("machines", service.machines());
    }

    @GetMapping("/machine/cards")
    public Map<String, Object> machineCards(
        HttpServletRequest request, @RequestParam(name = "id", required = false) String machineId
    ) {
        requireRead(request);
        return response("cards", service.machineCards(machineId));
    }

    @GetMapping("/agent/cards")
    public Map<String, Object> agentCards(
        HttpServletRequest request,
        @RequestParam(name = "agent_id", required = false) String agentId
    ) {
        requireRead(request);
        return response("cards", service.agentCards(agentId));
    }

    private void requireRead(HttpServletRequest request) {
        adminAuthService.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "agents.read");
    }

    private static Map<String, Object> response(String key, Object value) {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put(key, value);
        return CompatResponse.ok(body);
    }
}
