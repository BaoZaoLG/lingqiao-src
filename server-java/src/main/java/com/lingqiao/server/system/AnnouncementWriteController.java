package com.lingqiao.server.system;

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
@RequestMapping("/java/admin/api/announcement")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "announcement-write-enabled", havingValue = "true")
public class AnnouncementWriteController {
    private final AnnouncementWriteService service;
    private final AdminAuthService adminAuthService;

    public AnnouncementWriteController(AnnouncementWriteService service, AdminAuthService adminAuthService) {
        this.service = service;
        this.adminAuthService = adminAuthService;
    }

    @PostMapping
    public Map<String, Object> set(HttpServletRequest request, @RequestBody AnnouncementSetRequest body) {
        adminAuthService.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "announcement.write");
        AnnouncementDocument saved = service.saveRevision(body.content(), true);
        return CompatResponse.ok(Map.of("announcement", saved));
    }

    public record AnnouncementSetRequest(String content) {
    }
}
