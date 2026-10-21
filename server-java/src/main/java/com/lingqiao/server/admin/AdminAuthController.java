package com.lingqiao.server.admin;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.AdminPrincipal;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.http.HttpHeaders;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Admin authentication endpoints mirroring Go /admin/api/{login,logout,me}.
 * The session cookie keeps Go's HttpOnly/Secure/SameSite=Strict attributes.
 */
@RestController
@RequestMapping("/java/admin/api")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "admin-auth-enabled", havingValue = "true")
public class AdminAuthController {
    private final AdminLoginService logins;
    private final AdminAuthService adminAuthService;
    private final ObjectMapper mapper = new ObjectMapper();

    public AdminAuthController(AdminLoginService logins, AdminAuthService adminAuthService) {
        this.logins = logins;
        this.adminAuthService = adminAuthService;
    }

    @PostMapping(path = "/login", consumes = MediaType.APPLICATION_JSON_VALUE)
    public ResponseEntity<Map<String, Object>> login(HttpServletRequest request, @RequestBody String rawBody) {
        LoginRequest req = parse(rawBody);
        AdminLoginService.LoginResult result = logins.login(
            req.username(), req.password(), clientIp(request), request.getHeader("User-Agent"));

        Map<String, Object> body = new LinkedHashMap<>();
        body.put("token", result.token());
        return ResponseEntity.ok()
            .header(HttpHeaders.SET_COOKIE, sessionCookie(result.token(), result.maxAgeSeconds()))
            .header(HttpHeaders.CACHE_CONTROL, "no-store")
            .body(CompatResponse.ok(body));
    }

    @PostMapping("/logout")
    public ResponseEntity<Map<String, Object>> logout(HttpServletRequest request) {
        logins.logout(RequestAuthSupport.extractToken(request, "admin_token"));
        return ResponseEntity.ok()
            .header(HttpHeaders.SET_COOKIE, sessionCookie("", 0))
            .header(HttpHeaders.CACHE_CONTROL, "no-store")
            .body(CompatResponse.ok((Map<String, ?>) null));
    }

    @PostMapping(path = "/password", consumes = MediaType.APPLICATION_JSON_VALUE)
    public Map<String, Object> changePassword(
        HttpServletRequest request,
        @RequestBody PasswordRequest body
    ) {
        AdminPrincipal principal = adminAuthService.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "");
        logins.changePassword(
            principal.id(), body.old_password(), body.new_password(), clientIp(request));
        return CompatResponse.ok(Map.of("message", "密码修改成功"));
    }

    @GetMapping("/me")
    public Map<String, Object> me(HttpServletRequest request) {
        AdminPrincipal principal = adminAuthService.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "");

        Map<String, Object> body = new LinkedHashMap<>();
        body.put("id", principal.id());
        body.put("username", principal.username());
        body.put("role", principal.role());
        body.put("permissions", principal.permissions());
        return CompatResponse.ok(body);
    }

    private static String sessionCookie(String token, int maxAgeSeconds) {
        return "admin_token=" + token
            + "; Path=/; HttpOnly; Secure; SameSite=Strict; Max-Age=" + maxAgeSeconds;
    }

    private LoginRequest parse(String rawBody) {
        try {
            LoginRequest parsed = mapper.readValue(rawBody, LoginRequest.class);
            return parsed == null ? new LoginRequest("", "") : parsed;
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.INVALID_JSON, "invalid json");
        }
    }

    private static String clientIp(HttpServletRequest request) {
        String forwarded = request.getHeader("X-Forwarded-For");
        if (forwarded != null && !forwarded.isBlank()) {
            int comma = forwarded.indexOf(',');
            return comma > 0 ? forwarded.substring(0, comma).trim() : forwarded.trim();
        }
        return request.getRemoteAddr() == null ? "" : request.getRemoteAddr();
    }

    public record LoginRequest(String username, String password) {
    }

    public record PasswordRequest(String old_password, String new_password) {
    }
}
