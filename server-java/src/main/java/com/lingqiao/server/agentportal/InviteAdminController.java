package com.lingqiao.server.agentportal;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.http.MediaType;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/admin/api")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "admin-auth-enabled", havingValue = "true")
public class InviteAdminController {
    private final InviteAdminService invites;
    private final AdminAuthService auth;

    public InviteAdminController(InviteAdminService invites, AdminAuthService auth) {
        this.invites = invites;
        this.auth = auth;
    }

    @GetMapping("/invites")
    public Map<String, Object> list(HttpServletRequest request) {
        requireWrite(request);
        return CompatResponse.ok(Map.of("invites", invites.list()));
    }

    @PostMapping(path = "/invite/create", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> create(
        HttpServletRequest request,
        @RequestBody CreateRequest body
    ) {
        requireWrite(request);
        int count = clamp(body.count(), 1, 50, 1);
        int maxUses = clamp(body.max_uses(), 1, 1000, 1);
        List<InviteAdminService.InviteView> created = invites.create(count, maxUses);
        Map<String, Object> result = new LinkedHashMap<>();
        result.put("invites", created);
        result.put("count", created.size());
        return CompatResponse.ok(result);
    }

    @PostMapping(path = "/invite/delete", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> delete(
        HttpServletRequest request,
        @RequestBody DeleteRequest body
    ) {
        requireWrite(request);
        invites.delete(body.code());
        return CompatResponse.ok((Map<String, ?>) null);
    }

    private void requireWrite(HttpServletRequest request) {
        auth.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "invites.write");
    }

    private static int clamp(Integer value, int min, int max, int fallback) {
        if (value == null || value == 0) {
            return fallback;
        }
        if (value < min) {
            return min;
        }
        return Math.min(value, max);
    }

    public record CreateRequest(Integer count, Integer max_uses) {
    }

    public record DeleteRequest(String code) {
    }
}
