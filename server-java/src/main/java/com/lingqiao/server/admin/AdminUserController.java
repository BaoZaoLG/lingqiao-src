package com.lingqiao.server.admin;

import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
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
public class AdminUserController {
    private final AdminUserService users;
    private final AdminAuthService auth;

    public AdminUserController(AdminUserService users, AdminAuthService auth) {
        this.users = users;
        this.auth = auth;
    }

    @GetMapping("/admin-users")
    public Map<String, Object> list(HttpServletRequest request) {
        requireWrite(request);
        return Map.of("admins", users.list());
    }

    @PostMapping(path = "/admin-users", consumes = MediaType.APPLICATION_JSON_VALUE)
    public AdminUserService.AdminUserView create(
        HttpServletRequest request,
        @RequestBody CreateRequest body
    ) {
        requireWrite(request);
        return users.create(body.username(), body.password(), body.role(), body.disabled());
    }

    @PostMapping(path = "/admin-user/update", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> update(
        HttpServletRequest request,
        @RequestBody UpdateRequest body
    ) {
        requireWrite(request);
        users.update(new AdminUserService.UpdateInput(
            body.id(), body.change_password(), body.new_password(), body.change_role(),
            body.role(), body.set_disabled(), body.disabled()
        ));
        return Map.of("success", true);
    }

    private void requireWrite(HttpServletRequest request) {
        auth.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "admins.write");
    }

    public record CreateRequest(String username, String password, String role, boolean disabled) {
    }

    public record UpdateRequest(
        String id,
        boolean change_password,
        String new_password,
        boolean change_role,
        String role,
        boolean set_disabled,
        boolean disabled
    ) {
    }
}
