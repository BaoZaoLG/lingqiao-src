package com.lingqiao.server.system;

import com.lingqiao.server.common.api.CompatResponse;
import com.lingqiao.server.config.CutoverProperties;
import com.lingqiao.server.config.SidecarProperties;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/java/api")
@EnableConfigurationProperties({SidecarProperties.class, CutoverProperties.class})
public class CutoverController {
    private final SidecarProperties sidecarProperties;
    private final CutoverProperties cutoverProperties;

    public CutoverController(SidecarProperties sidecarProperties, CutoverProperties cutoverProperties) {
        this.sidecarProperties = sidecarProperties;
        this.cutoverProperties = cutoverProperties;
    }

    @GetMapping("/cutover")
    public Map<String, Object> cutover() {
        Map<String, Object> modules = new LinkedHashMap<>();
        modules.put("admin", Map.of(
            "enabled", sidecarProperties.adminAuthEnabled() || sidecarProperties.adminOverviewEnabled(),
            "cutover", cutoverProperties.admin()));
        modules.put("exam", Map.of("enabled", sidecarProperties.examEnabled(), "cutover", cutoverProperties.exam()));
        modules.put("audit", Map.of("enabled", sidecarProperties.auditEnabled(), "cutover", cutoverProperties.audit()));
        modules.put("scripts", Map.of("enabled", sidecarProperties.scriptsEnabled(), "cutover", cutoverProperties.scripts()));
        modules.put("cards", Map.of("enabled", sidecarProperties.cardsEnabled(), "cutover", cutoverProperties.cards()));
        modules.put("sessions", Map.of("enabled", sidecarProperties.sessionsEnabled(), "cutover", cutoverProperties.sessions()));
        modules.put("agents", Map.of("enabled", sidecarProperties.agentsEnabled(), "cutover", cutoverProperties.agents()));
        modules.put("chat", Map.of("enabled", sidecarProperties.chatEnabled(), "cutover", cutoverProperties.chat()));
        modules.put("blacklist", Map.of("enabled", sidecarProperties.blacklistEnabled(), "cutover", cutoverProperties.blacklist()));
        modules.put("announcement", Map.of("enabled", sidecarProperties.announcementEnabled(), "cutover", cutoverProperties.announcement()));
        modules.put("releases", Map.of("enabled", sidecarProperties.releasesEnabled(), "cutover", cutoverProperties.releases()));
        modules.put("payloads", Map.of("enabled", sidecarProperties.payloadsEnabled(), "cutover", cutoverProperties.payloads()));

        Map<String, Object> body = new LinkedHashMap<>();
        body.put("enabled", cutoverProperties.enabled());
        body.put("production_entrypoint_replaced", sidecarProperties.productionEntrypointReplaced());
        body.put("modules", modules);
        body.put("rollback", "disable lingqiao.cutover.* flags and keep Go routes");
        return CompatResponse.ok(body);
    }
}
