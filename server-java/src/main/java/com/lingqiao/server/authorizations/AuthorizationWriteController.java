package com.lingqiao.server.authorizations;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/admin/api/control/authorizations")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "cards-write-enabled", havingValue = "true")
public class AuthorizationWriteController {
    private final AuthorizationControlService service;
    private final AdminAuthService adminAuthService;

    public AuthorizationWriteController(
        AuthorizationControlService service, AdminAuthService adminAuthService
    ) {
        this.service = service;
        this.adminAuthService = adminAuthService;
    }

    @PostMapping("/{code}/actions")
    public Map<String, Object> action(
        HttpServletRequest request,
        @PathVariable String code,
        @RequestBody AuthorizationReadController.ActionRequest body
    ) {
        // Go currently registers the dynamic single-authorization route under
        // cards.read; keep permission parity until that public contract changes.
        adminAuthService.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "cards.read");
        AuthorizationControlService.ActionResult result =
            service.execute(List.of(code), body.action(), body.extend_hours());
        if (result.updated() > 0) {
            service.recordAudit(
                auditAction(body.action()), code, "Action: " + nullToEmpty(body.action()));
        }
        return response(result);
    }

    @PostMapping("/bulk/actions")
    public Map<String, Object> bulkAction(
        HttpServletRequest request,
        @RequestBody AuthorizationReadController.BulkActionRequest body
    ) {
        adminAuthService.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "cards.bulk");
        AuthorizationControlService.ActionResult result =
            service.execute(body.codes(), body.action(), body.extend_hours());
        if (result.updated() > 0) {
            service.recordAudit(
                "authorization_bulk_updated", "", "Action: " + nullToEmpty(body.action()));
        }
        return response(result);
    }

    private static Map<String, Object> response(
        AuthorizationControlService.ActionResult result
    ) {
        Map<String, Object> fields = new LinkedHashMap<>();
        fields.put("result", result);
        return CompatResponse.ok(fields);
    }

    static String auditAction(String action) {
        return switch (nullToEmpty(action)) {
            case "extend" -> "authorization_extended";
            case "unbind_machine" -> "authorization_unbound";
            case "freeze" -> "authorization_frozen";
            case "restore" -> "authorization_restored";
            case "archive_delete" -> "authorization_archived";
            default -> "authorization_updated";
        };
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }
}
