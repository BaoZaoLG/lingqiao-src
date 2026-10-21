package com.lingqiao.server.sessions;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/admin/api/sessions")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "sessions-enabled", havingValue = "true")
public class SessionsController {
    private final SessionsService service;
    private final AdminAuthService adminAuthService;

    public SessionsController(SessionsService service, AdminAuthService adminAuthService) {
        this.service = service;
        this.adminAuthService = adminAuthService;
    }

    @GetMapping
    public Map<String, Object> list(
        HttpServletRequest request,
        @RequestParam(name = "card", required = false) String card,
        @RequestParam(name = "machine", required = false) String machine,
        @RequestParam(name = "active", defaultValue = "1") String active,
        @RequestParam(defaultValue = "1") int page,
        @RequestParam(name = "per_page", defaultValue = "20") int perPage
    ) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "sessions.read");
        boolean activeOnly = !"0".equals(active);
        SessionListResult result = service.list(new SessionListFilter(card, machine, activeOnly, page, perPage));
        List<Map<String, Object>> items = new ArrayList<>();
        for (Session s : result.items()) {
            Map<String, Object> item = new LinkedHashMap<>();
            item.put("token", s.token());
            item.put("card", s.cardCode());
            item.put("card_code", s.cardCode());
            item.put("machine_id", s.machineId());
            item.put("fingerprint", s.fingerprint());
            item.put("client_version", s.clientVersion());
            item.put("created_at", s.createdAt().toString());
            item.put("last_seen", s.lastSeen().toString());
            item.put("expires_at", s.expiresAt().toString());
            item.put("remote_addr", s.remoteAddr());
            items.add(item);
        }
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("sessions", items);
        body.put("total", result.total());
        body.put("page", result.page());
        body.put("per_page", result.perPage());
        return CompatResponse.ok(body);
    }
}
