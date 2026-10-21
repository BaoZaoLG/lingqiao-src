package com.lingqiao.server.scripts;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/admin/api/scripts")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "scripts-enabled", havingValue = "true")
public class ScriptsReadController {
    private final ScriptsService service;
    private final AdminAuthService adminAuthService;

    public ScriptsReadController(ScriptsService service, AdminAuthService adminAuthService) {
        this.service = service;
        this.adminAuthService = adminAuthService;
    }

    @GetMapping
    public Map<String, Object> list(HttpServletRequest request) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "scripts.write");
        ScriptListResult result = service.list();
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("scripts", result.scripts());
        body.put("active_id", result.activeId());
        body.put("active", result.active());
        return CompatResponse.ok(body);
    }

    @GetMapping("/{id}")
    public Map<String, Object> get(HttpServletRequest request, @PathVariable String id) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "scripts.write");
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("script", service.get(id));
        return CompatResponse.ok(body);
    }
}
