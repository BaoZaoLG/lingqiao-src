package com.lingqiao.server.agents;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/admin/api/agent")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "agents-write-enabled", havingValue = "true")
public class AgentsWriteController {
    private final AgentsWriteService service;
    private final AdminAuthService adminAuthService;

    public AgentsWriteController(AgentsWriteService service, AdminAuthService adminAuthService) {
        this.service = service;
        this.adminAuthService = adminAuthService;
    }

    @PostMapping("/update")
    public Map<String, Object> update(HttpServletRequest request, @RequestBody UpdateAgentRequest body) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "agents.write");
        String action = body.action() == null ? "" : body.action();
        switch (action) {
            case "disable" -> service.setStatus(body.agent_id(), true);
            case "enable" -> service.setStatus(body.agent_id(), false);
            case "delete" -> service.delete(body.agent_id());
            case "update_limits" -> service.setLimits(
                body.agent_id(),
                body.max_session_duration_minutes(),
                body.max_daily_sessions(),
                body.max_concurrent_sessions()
            );
            case "reset_password" -> service.forceResetPassword(body.agent_id(), body.password());
            default -> throw new SidecarException(ErrorCode.BAD_REQUEST, "unknown action");
        }
        return CompatResponse.ok((Map<String, ?>) null);
    }

    public record UpdateAgentRequest(
        String agent_id,
        String action,
        String password,
        int max_session_duration_minutes,
        int max_daily_sessions,
        int max_concurrent_sessions
    ) {
    }
}
