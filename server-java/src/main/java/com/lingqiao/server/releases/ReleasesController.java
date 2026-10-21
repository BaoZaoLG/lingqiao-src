package com.lingqiao.server.releases;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.common.security.AdminAuthService;
import com.lingqiao.server.common.security.RequestAuthSupport;
import jakarta.servlet.http.HttpServletRequest;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.dao.DataAccessException;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/admin/api/releases")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "releases-enabled", havingValue = "true")
public class ReleasesController {
    private final ReleasesService service;
    private final AdminAuthService adminAuthService;
    private final ManifestSeedLoader seedLoader;

    public ReleasesController(ReleasesService service,
                              AdminAuthService adminAuthService,
                              ManifestSeedLoader seedLoader) {
        this.service = service;
        this.adminAuthService = adminAuthService;
        this.seedLoader = seedLoader;
    }

    @GetMapping
    public Map<String, Object> list(HttpServletRequest request) {
        adminAuthService.requireAdmin(RequestAuthSupport.extractToken(request, "admin_token"), "releases.write");

        List<Release> releases;
        try {
            releases = service.list();
        } catch (DataAccessException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to list releases");
        }
        List<Map<String, Object>> items = new ArrayList<>();
        for (Release release : releases) {
            Map<String, Object> item = new LinkedHashMap<>();
            item.put("release", release);
            try {
                item.put("packages", service.listPackages(release.id()));
            } catch (DataAccessException ignored) {
                item.put("packages", null);
            }
            try {
                item.put("metrics", service.metrics(release.id()));
            } catch (DataAccessException ignored) {
                item.put("metrics", ReleaseMetrics.empty(release.id()));
            }
            items.add(item);
        }

        Map<String, Object> body = new LinkedHashMap<>();
        body.put("releases", items);
        body.put("public_key", publicKeyOrEmpty());
        return CompatResponse.ok(body);
    }

    @GetMapping("/{id}/events")
    public Map<String, Object> events(HttpServletRequest request, @PathVariable String id) {
        adminAuthService.requireAdmin(
            RequestAuthSupport.extractToken(request, "admin_token"), "releases.write");
        List<ReleaseAdminService.ReleaseEventView> events;
        try {
            events = service.listEvents(id, 100);
        } catch (DataAccessException ex) {
            throw new SidecarException(
                ErrorCode.INTERNAL_ERROR, "failed to list release events");
        }
        ReleaseMetrics metrics;
        try {
            metrics = service.metrics(id);
        } catch (DataAccessException ex) {
            throw new SidecarException(
                ErrorCode.INTERNAL_ERROR, "failed to load release metrics");
        }
        return CompatResponse.ok(Map.of("events", events, "metrics", metrics));
    }

    private String publicKeyOrEmpty() {
        try {
            return new ManifestSigner(seedLoader.load()).publicKeyHex();
        } catch (RuntimeException ex) {
            return "";
        }
    }
}
