package com.lingqiao.server.sessions;

import com.lingqiao.server.common.api.CompatResponse;
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
@RequestMapping("/java/admin/api")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "sessions-write-enabled", havingValue = "true")
public class SessionsWriteController {
    private final SessionsWriteService service;
    private final AdminAuthService adminAuthService;

    public SessionsWriteController(SessionsWriteService service, AdminAuthService adminAuthService) {
        this.service = service;
        this.adminAuthService = adminAuthService;
    }

    @PostMapping("/force-logout")
    public Map<String, Object> forceLogout(HttpServletRequest request, @RequestBody ForceLogoutRequest body) {
        adminAuthService.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "sessions.force_logout");
        service.forceLogout(body.session_token());
        return CompatResponse.ok(Map.of("message", "session terminated"));
    }

    public record ForceLogoutRequest(String session_token) {
    }
}
