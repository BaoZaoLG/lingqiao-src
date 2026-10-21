package com.lingqiao.server.audit;

import java.time.Instant;
import java.time.OffsetDateTime;
import java.time.ZoneOffset;

public record AuditEvent(
    OffsetDateTime time,
    String action,
    String actorType,
    String actorId,
    String resourceType,
    String resourceId,
    String before,
    String after,
    String userAgent,
    String requestId,
    String card,
    String agentId,
    String machine,
    String ip,
    String detail
) {
    public AuditEvent(
        Instant time,
        String action,
        String actorType,
        String actorId,
        String resourceType,
        String resourceId,
        String before,
        String after,
        String userAgent,
        String requestId,
        String card,
        String agentId,
        String machine,
        String ip,
        String detail
    ) {
        this(time == null ? null : time.atOffset(ZoneOffset.UTC), action, actorType, actorId,
            resourceType, resourceId, before, after, userAgent, requestId, card, agentId,
            machine, ip, detail);
    }
}
