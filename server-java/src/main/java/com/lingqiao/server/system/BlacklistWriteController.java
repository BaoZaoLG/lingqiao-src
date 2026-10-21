package com.lingqiao.server.system;

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
@RequestMapping("/java/admin/api/blacklist")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "blacklist-write-enabled", havingValue = "true")
public class BlacklistWriteController {
    private final BlacklistWriteService service;
    private final AdminAuthService adminAuthService;

    public BlacklistWriteController(BlacklistWriteService service, AdminAuthService adminAuthService) {
        this.service = service;
        this.adminAuthService = adminAuthService;
    }

    @PostMapping
    public Map<String, Object> mutate(HttpServletRequest request, @RequestBody BlacklistMutationRequest body) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "system.read");
        String action = body.action() == null ? "" : body.action();
        switch (action) {
            case "add" -> service.add(body.type(), body.value(), body.reason());
            case "remove" -> service.remove(body.value());
            default -> throw new SidecarException(ErrorCode.BAD_REQUEST, "unknown action");
        }
        return CompatResponse.ok((Map<String, ?>) null);
    }

    public record BlacklistMutationRequest(String action, String type, String value, String reason) {
    }
}
