package com.lingqiao.server.releases;

import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RequestPart;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.multipart.MultipartFile;

@RestController
@RequestMapping("/java/admin/api/releases")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "releases-write-enabled", havingValue = "true")
public class ReleasesWriteController {
    private final ReleaseAdminService service;
    private final AdminAuthService adminAuthService;
    private final ObjectMapper mapper = new ObjectMapper();

    public ReleasesWriteController(
        ReleaseAdminService service,
        AdminAuthService adminAuthService
    ) {
        this.service = service;
        this.adminAuthService = adminAuthService;
    }

    @PostMapping
    public Map<String, Object> create(
        HttpServletRequest request,
        @RequestBody CreateReleaseRequest body
    ) {
        requireWrite(request);
        Release release = service.create(new ReleaseAdminService.CreateRelease(
            body.version(), body.channel(), body.minVersion(), body.forceUpdate(),
            body.rolloutPercent(), body.notes(), body.targeting()));
        return CompatResponse.ok(Map.of("release", release));
    }

    @PostMapping("/{id}/publish")
    public Map<String, Object> publish(HttpServletRequest request, @PathVariable String id) {
        requireWrite(request);
        return CompatResponse.ok(Map.of("release", service.publish(id)));
    }

    @PostMapping("/{id}/pause")
    public Map<String, Object> pause(HttpServletRequest request, @PathVariable String id) {
        requireWrite(request);
        return CompatResponse.ok(Map.of("release", service.pause(id)));
    }

    @PostMapping("/{id}/rollback")
    public Map<String, Object> rollback(
        HttpServletRequest request,
        @PathVariable String id,
        @RequestBody(required = false) String rawBody
    ) {
        requireWrite(request);
        return CompatResponse.ok(Map.of(
            "release", service.rollback(id, rollbackTarget(rawBody))));
    }

    @PostMapping(path = "/{id}/packages", consumes = "multipart/form-data")
    public Map<String, Object> uploadPackage(
        HttpServletRequest request,
        @PathVariable String id,
        @RequestParam(name = "kind", required = false) String kind,
        @RequestPart(name = "file", required = false) MultipartFile file
    ) {
        requireWrite(request);
        return CompatResponse.ok(Map.of("package", service.uploadPackage(id, kind, file)));
    }

    private String rollbackTarget(String rawBody) {
        if (rawBody == null || rawBody.isEmpty()) {
            return "";
        }
        try {
            RollbackRequest body = mapper.readValue(rawBody, RollbackRequest.class);
            return body == null || body.targetReleaseId() == null ? "" : body.targetReleaseId();
        } catch (Exception ignored) {
            return "";
        }
    }

    private void requireWrite(HttpServletRequest request) {
        adminAuthService.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "releases.write");
    }

    public record CreateReleaseRequest(
        String version,
        String channel,
        @JsonProperty("min_version") String minVersion,
        @JsonProperty("force_update") boolean forceUpdate,
        @JsonProperty("rollout_percent") int rolloutPercent,
        String notes,
        TargetingRules targeting
    ) {
    }

    public record RollbackRequest(
        @JsonProperty("target_release_id") String targetReleaseId
    ) {
    }
}
