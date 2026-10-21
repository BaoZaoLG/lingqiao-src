package com.lingqiao.server.config;

import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "lingqiao.sidecar")
public record SidecarProperties(
    boolean examEnabled,
    boolean examWriteEnabled,
    boolean auditEnabled,
    boolean auditWriteEnabled,
    boolean scriptsEnabled,
    boolean scriptsWriteEnabled,
    boolean cardsEnabled,
    boolean cardsWriteEnabled,
    boolean sessionsEnabled,
    boolean sessionsWriteEnabled,
    boolean agentsEnabled,
    boolean agentsWriteEnabled,
    boolean chatEnabled,
    boolean chatWriteEnabled,
    boolean blacklistEnabled,
    boolean blacklistWriteEnabled,
    boolean announcementEnabled,
    boolean announcementWriteEnabled,
    boolean releasesEnabled,
    boolean releasesWriteEnabled,
    boolean payloadsEnabled,
    boolean payloadsWriteEnabled,
    boolean clientWriteEnabled,
    boolean clientDownloadEnabled,
    boolean clientUpdateEnabled,
    boolean clientChatEnabled,
    boolean clientAnnouncementEnabled,
    boolean adminAuthEnabled,
    boolean adminOverviewEnabled,
    boolean agentPortalEnabled,
    boolean agentPortalWriteEnabled,
    boolean productionEntrypointReplaced
) {
}
