package com.lingqiao.server.audit;

import java.time.Instant;

public record AuditFilter(
    String action,
    String actorType,
    String actorId,
    String resourceType,
    String resourceId,
    String card,
    String agentId,
    String machine,
    String ip,
    Instant from,
    Instant to,
    String query,
    int page,
    int perPage
) {
    public AuditFilter {
        page = page < 1 ? 1 : page;
        perPage = perPage < 1 || perPage > 200 ? 50 : perPage;
    }
}
