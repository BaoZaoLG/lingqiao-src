package com.lingqiao.server.authorizations;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.time.Instant;
import java.time.format.DateTimeParseException;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/admin/api/control/authorizations")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "cards-enabled", havingValue = "true")
public class AuthorizationReadController {
    private final AuthorizationControlService service;
    private final AdminAuthService adminAuthService;

    public AuthorizationReadController(
        AuthorizationControlService service, AdminAuthService adminAuthService
    ) {
        this.service = service;
        this.adminAuthService = adminAuthService;
    }

    @GetMapping
    public Map<String, Object> list(
        HttpServletRequest request,
        @RequestParam(name = "search", required = false) String search,
        @RequestParam(name = "effective_status", required = false) String effectiveStatus,
        @RequestParam(name = "agent_id", required = false) String agentId,
        @RequestParam(name = "machine_id", required = false) String machineId,
        @RequestParam(name = "bound", required = false) String bound,
        @RequestParam(name = "expires_after", required = false) String expiresAfter,
        @RequestParam(name = "expires_before", required = false) String expiresBefore,
        @RequestParam(name = "max_sessions", required = false) String maxSessions
    ) {
        requireCardsRead(request);
        List<AuthorizationControlService.AuthorizationSummary> items = service.list(
            new AuthorizationControlService.ListFilter(
                search, effectiveStatus, agentId, machineId, bound,
                parseTime(expiresAfter), parseTime(expiresBefore),
                parseInteger(maxSessions), false));
        return response("authorizations", items);
    }

    @GetMapping("/{code}")
    public Map<String, Object> detail(
        HttpServletRequest request, @PathVariable String code
    ) {
        requireCardsRead(request);
        return response("authorization", service.detail(code));
    }

    @PostMapping("/{code}/impact")
    public Map<String, Object> impact(
        HttpServletRequest request,
        @PathVariable String code,
        @RequestBody ActionRequest body
    ) {
        requireCardsRead(request);
        try {
            return response(
                "preview",
                service.preview(List.of(code), body.action(), body.extend_hours()));
        } catch (SidecarException ex) {
            if (ex.code() == ErrorCode.CARD_NOT_FOUND) {
                throw ex;
            }
            throw new SidecarException(
                ErrorCode.INTERNAL_ERROR, "failed to preview authorization action");
        }
    }

    @PostMapping("/bulk/impact")
    public Map<String, Object> bulkImpact(
        HttpServletRequest request, @RequestBody BulkActionRequest body
    ) {
        adminAuthService.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "cards.bulk");
        try {
            return response(
                "preview",
                service.preview(body.codes(), body.action(), body.extend_hours()));
        } catch (RuntimeException ex) {
            throw new SidecarException(
                ErrorCode.INTERNAL_ERROR, "failed to preview bulk authorization action");
        }
    }

    private void requireCardsRead(HttpServletRequest request) {
        adminAuthService.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "cards.read");
    }

    static Instant parseTime(String value) {
        if (value == null || value.isEmpty()) {
            return null;
        }
        try {
            return Instant.parse(value);
        } catch (DateTimeParseException ex) {
            return null;
        }
    }

    static int parseInteger(String value) {
        if (value == null || value.isEmpty()) {
            return 0;
        }
        try {
            return Integer.parseInt(value);
        } catch (NumberFormatException ex) {
            return 0;
        }
    }

    private static Map<String, Object> response(String key, Object value) {
        Map<String, Object> fields = new LinkedHashMap<>();
        fields.put(key, value);
        return CompatResponse.ok(fields);
    }

    public record ActionRequest(String action, int extend_hours) {
    }

    public record BulkActionRequest(List<String> codes, String action, int extend_hours) {
    }
}
