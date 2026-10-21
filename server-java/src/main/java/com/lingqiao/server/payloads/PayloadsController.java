package com.lingqiao.server.payloads;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/admin/api/payloads")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "payloads-enabled", havingValue = "true")
public class PayloadsController {
    private final PayloadsService service;
    private final AdminAuthService adminAuthService;

    public PayloadsController(PayloadsService service, AdminAuthService adminAuthService) {
        this.service = service;
        this.adminAuthService = adminAuthService;
    }

    @GetMapping
    public Map<String, Object> list(HttpServletRequest request) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "payloads.write");
        PayloadListResult result = service.list();
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("payloads", result.payloads());
        body.put("active_id", result.activeId());
        Payload active = result.payloads().stream()
            .filter(Payload::active)
            .findFirst()
            .orElse(null);
        body.put("active", active);
        body.put("latest", result.payloads().isEmpty() ? null : result.payloads().get(0));
        body.put("total", result.payloads().size());
        return CompatResponse.ok(body);
    }
}
