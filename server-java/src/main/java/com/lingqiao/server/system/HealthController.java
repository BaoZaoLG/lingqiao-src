package com.lingqiao.server.system;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.config.SidecarProperties;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/api")
@EnableConfigurationProperties(SidecarProperties.class)
public class HealthController {
    private final SidecarProperties properties;

    public HealthController(SidecarProperties properties) {
        this.properties = properties;
    }

    @GetMapping("/health")
    public Map<String, Object> health() {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("service", "lingqiao-java-sidecar");
        body.put("mode", "sidecar");
        body.put("time", Instant.now().toString());
        body.put("exam_enabled", properties.examEnabled());
        body.put("exam_write_enabled", properties.examWriteEnabled());
        body.put("audit_enabled", properties.auditEnabled());
        body.put("audit_write_enabled", properties.auditWriteEnabled());
        body.put("scripts_enabled", properties.scriptsEnabled());
        body.put("scripts_write_enabled", properties.scriptsWriteEnabled());
        body.put("cards_enabled", properties.cardsEnabled());
        body.put("cards_write_enabled", properties.cardsWriteEnabled());
        body.put("sessions_enabled", properties.sessionsEnabled());
        body.put("sessions_write_enabled", properties.sessionsWriteEnabled());
        body.put("agents_enabled", properties.agentsEnabled());
        body.put("agents_write_enabled", properties.agentsWriteEnabled());
        body.put("chat_enabled", properties.chatEnabled());
        body.put("chat_write_enabled", properties.chatWriteEnabled());
        body.put("blacklist_enabled", properties.blacklistEnabled());
        body.put("blacklist_write_enabled", properties.blacklistWriteEnabled());
        body.put("announcement_enabled", properties.announcementEnabled());
        body.put("announcement_write_enabled", properties.announcementWriteEnabled());
        body.put("releases_enabled", properties.releasesEnabled());
        body.put("releases_write_enabled", properties.releasesWriteEnabled());
        body.put("payloads_enabled", properties.payloadsEnabled());
        body.put("payloads_write_enabled", properties.payloadsWriteEnabled());
        body.put("client_write_enabled", properties.clientWriteEnabled());
        body.put("client_download_enabled", properties.clientDownloadEnabled());
        body.put("client_update_enabled", properties.clientUpdateEnabled());
        body.put("client_chat_enabled", properties.clientChatEnabled());
        body.put("client_announcement_enabled", properties.clientAnnouncementEnabled());
        body.put("admin_auth_enabled", properties.adminAuthEnabled());
        body.put("admin_overview_enabled", properties.adminOverviewEnabled());
        body.put("agent_portal_enabled", properties.agentPortalEnabled());
        body.put("agent_portal_write_enabled", properties.agentPortalWriteEnabled());
        body.put("production_entrypoint_replaced", properties.productionEntrypointReplaced());
        return CompatResponse.ok(body);
    }
}
