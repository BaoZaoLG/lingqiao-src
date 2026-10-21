package com.lingqiao.server.scripts;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/admin/api/scripts")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "scripts-write-enabled", havingValue = "true")
public class ScriptsWriteController {
    private final ScriptsWriteService service;
    private final AdminAuthService adminAuthService;

    public ScriptsWriteController(ScriptsWriteService service, AdminAuthService adminAuthService) {
        this.service = service;
        this.adminAuthService = adminAuthService;
    }

    @PostMapping("/create")
    public Map<String, Object> create(HttpServletRequest request, @RequestBody ScriptWriteRequest body) {
        requireWrite(request);
        return CompatResponse.ok(Map.of("script", service.create(body.version(), body.content(), body.note())));
    }

    @PostMapping("/publish")
    public Map<String, Object> publish(HttpServletRequest request, @RequestBody ScriptWriteRequest body) {
        requireWrite(request);
        return CompatResponse.ok(Map.of("script", service.publish(body.version(), body.content(), body.note())));
    }

    @PostMapping("/{id}/activate")
    public Map<String, Object> activate(HttpServletRequest request, @PathVariable String id) {
        requireWrite(request);
        return CompatResponse.ok(Map.of("script", service.activate(id)));
    }

    @DeleteMapping("/{id}/delete")
    public Map<String, Object> delete(HttpServletRequest request, @PathVariable String id) {
        requireWrite(request);
        service.delete(id);
        return CompatResponse.ok(Map.of("id", id));
    }

    private void requireWrite(HttpServletRequest request) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "scripts.write");
    }

    public record ScriptWriteRequest(String version, String content, String note) {
    }
}
