package com.lingqiao.server.scripts;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

/** Backwards-compatible single-endpoint script API mirroring Go HandleScriptAdmin. */
@RestController
@RequestMapping("/java/admin/api/script")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "scripts-enabled", havingValue = "true")
public class ScriptsLegacyController {
    private final ScriptsService reads;
    private final ObjectProvider<ScriptsWriteService> writes;
    private final AdminAuthService auth;

    public ScriptsLegacyController(
        ScriptsService reads,
        ObjectProvider<ScriptsWriteService> writes,
        AdminAuthService auth
    ) {
        this.reads = reads;
        this.writes = writes;
        this.auth = auth;
    }

    @GetMapping
    public Map<String, Object> get(
        HttpServletRequest request,
        @RequestParam(value = "id", required = false) String id
    ) {
        requireWritePermission(request);
        if (id != null && !id.isEmpty()) {
            return CompatResponse.ok(Map.of("script", reads.get(id)));
        }
        ScriptListResult result = reads.list();
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("scripts", result.scripts());
        body.put("active_id", result.activeId());
        ScriptModule active = result.active();
        if (active != null) {
            body.put("script", active);
            body.put("id", active.id());
            body.put("version", active.version());
            body.put("sha256", active.sha256());
            body.put("content", active.content());
            body.put("size", active.size());
            body.put("updated_at", active.updatedAt());
        }
        return CompatResponse.ok(body);
    }

    @PostMapping
    public Map<String, Object> mutate(
        HttpServletRequest request,
        @RequestBody LegacyScriptRequest body
    ) {
        requireWritePermission(request);
        ScriptsWriteService service = writes.getIfAvailable();
        if (service == null) {
            throw new SidecarException(ErrorCode.FEATURE_DISABLED, "scripts write module disabled");
        }
        String action = body.action() == null ? "" : body.action();
        return switch (action) {
            case "activate" -> CompatResponse.ok(
                Map.of("script", activate(service, body.id())));
            case "delete" -> {
                delete(service, body.id());
                yield CompatResponse.ok(Map.of("id", body.id()));
            }
            case "save" -> CompatResponse.ok(
                Map.of("script", create(service, body)));
            default -> publish(service, body);
        };
    }

    private static ScriptModule activate(ScriptsWriteService service, String id) {
        try {
            return service.activate(id);
        } catch (SidecarException ex) {
            if (ex.code() == ErrorCode.NOT_FOUND) {
                throw new SidecarException(ErrorCode.NOT_FOUND, "script not found");
            }
            throw new SidecarException(ErrorCode.BAD_REQUEST, "failed to activate script");
        }
    }

    private static void delete(ScriptsWriteService service, String id) {
        try {
            service.delete(id);
        } catch (SidecarException ex) {
            if (ex.code() == ErrorCode.NOT_FOUND) {
                throw new SidecarException(ErrorCode.NOT_FOUND, "script not found");
            }
            if (ex.getMessage().contains("active")) {
                throw new SidecarException(ErrorCode.BAD_REQUEST, "cannot delete active script");
            }
            throw new SidecarException(ErrorCode.BAD_REQUEST, "failed to delete script");
        }
    }

    private static ScriptModule create(ScriptsWriteService service, LegacyScriptRequest body) {
        try {
            return service.create(body.version(), body.content(), body.note());
        } catch (RuntimeException ex) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "failed to create script");
        }
    }

    private static Map<String, Object> publish(
        ScriptsWriteService service, LegacyScriptRequest body
    ) {
        ScriptModule module;
        try {
            module = service.publish(body.version(), body.content(), body.note());
        } catch (RuntimeException ex) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "failed to publish script");
        }
        Map<String, Object> result = new LinkedHashMap<>();
        result.put("id", module.id());
        result.put("version", module.version());
        result.put("sha256", module.sha256());
        result.put("content", module.content());
        result.put("size", module.size());
        result.put("updated_at", module.updatedAt());
        return CompatResponse.ok(result);
    }

    private void requireWritePermission(HttpServletRequest request) {
        auth.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "scripts.write");
    }

    public record LegacyScriptRequest(
        String version,
        String content,
        String note,
        String action,
        String id
    ) {
    }
}
