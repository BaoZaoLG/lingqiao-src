package com.lingqiao.server.system;

import com.lingqiao.server.common.api.CompatResponse;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Public client announcement endpoint mirroring Go /api/v1/announcement.
 * Go serves this without authentication, so the sidecar keeps the same contract
 * and only ever exposes the currently active announcement.
 */
@RestController
@RequestMapping("/java/api/v1/announcement")
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "client-announcement-enabled", havingValue = "true")
public class ClientAnnouncementController {
    private final AnnouncementReadSupport announcements;

    public ClientAnnouncementController(AnnouncementReadSupport announcements) {
        this.announcements = announcements;
    }

    @GetMapping
    public Map<String, Object> announcement() {
        AnnouncementIndex index = announcements.read();
        AnnouncementDocument active = index.getAnnouncements().stream()
            .filter(a -> a.getId().equals(index.getActiveId()))
            .filter(a -> !a.getContent().isEmpty())
            .findFirst()
            .orElse(null);

        Map<String, Object> body = new LinkedHashMap<>();
        body.put("announcement", active);
        return CompatResponse.ok(body);
    }
}
