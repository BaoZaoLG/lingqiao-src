package com.lingqiao.server.audit;

import com.fasterxml.jackson.annotation.JsonProperty;
import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.dao.DataAccessException;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/admin/api/audit")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "audit-enabled", havingValue = "true")
public class AuditReadController {
    private final AuditService service;
    private final AdminAuthService adminAuthService;

    public AuditReadController(AuditService service, AdminAuthService adminAuthService) {
        this.service = service;
        this.adminAuthService = adminAuthService;
    }

    @GetMapping
    public Object list(
        HttpServletRequest request,
        HttpServletResponse response,
        @RequestParam(required = false) String action,
        @RequestParam(name = "actor_type", required = false) String actorType,
        @RequestParam(name = "actor_id", required = false) String actorId,
        @RequestParam(name = "resource_type", required = false) String resourceType,
        @RequestParam(name = "resource_id", required = false) String resourceId,
        @RequestParam(required = false) String card,
        @RequestParam(name = "agent_id", required = false) String agentId,
        @RequestParam(required = false) String machine,
        @RequestParam(required = false) String ip,
        @RequestParam(required = false) String from,
        @RequestParam(required = false) String to,
        @RequestParam(name = "q", required = false) String query,
        @RequestParam(required = false) String export,
        @RequestParam(defaultValue = "1") int page,
        @RequestParam(name = "per_page", defaultValue = "50") int perPage
    ) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "audit.read");
        AuditFilter filter = new AuditFilter(
            action, actorType, actorId, resourceType, resourceId, card, agentId, machine, ip,
            parseInstant(from), parseInstant(to), query, page, perPage
        );
        if ("csv".equals(export)) {
            try {
                response.setHeader("Content-Type", "text/csv; charset=utf-8");
                response.setHeader("Content-Disposition", "attachment; filename=audit_export.csv");
                response.getOutputStream().write(AuditCsv.export(service.queryAll(filter)));
                return null;
            } catch (DataAccessException ex) {
                throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to query audit log");
            } catch (IOException ex) {
                throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to write audit export");
            }
        }
        AuditPage result;
        try {
            result = service.query(filter);
        } catch (DataAccessException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to query audit log");
        }
        List<Map<String, Object>> entries = result.entries().stream().map(this::toMap).toList();
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("entries", entries);
        body.put("total", result.total());
        body.put("page", result.page());
        body.put("per_page", result.perPage());
        return CompatResponse.ok(body);
    }

    private Map<String, Object> toMap(AuditEvent event) {
        Map<String, Object> map = new LinkedHashMap<>();
        map.put("time", event.time());
        map.put("action", event.action());
        putIfPresent(map, "actor_type", event.actorType());
        putIfPresent(map, "actor_id", event.actorId());
        putIfPresent(map, "resource_type", event.resourceType());
        putIfPresent(map, "resource_id", event.resourceId());
        putIfPresent(map, "before", event.before());
        putIfPresent(map, "after", event.after());
        putIfPresent(map, "user_agent", event.userAgent());
        putIfPresent(map, "request_id", event.requestId());
        putIfPresent(map, "card", event.card());
        putIfPresent(map, "agent_id", event.agentId());
        putIfPresent(map, "machine", event.machine());
        putIfPresent(map, "ip", event.ip());
        putIfPresent(map, "detail", event.detail());
        return map;
    }

    private static void putIfPresent(Map<String, Object> map, String key, String value) {
        if (value != null && !value.isBlank()) {
            map.put(key, value);
        }
    }

    private static Instant parseInstant(String value) {
        if (value == null || value.isBlank()) {
            return null;
        }
        try {
            return Instant.parse(value);
        } catch (java.time.format.DateTimeParseException ignored) {
            return null;
        }
    }
}
