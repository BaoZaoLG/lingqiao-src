package com.lingqiao.server.releases;

import java.time.Instant;

public record ReleaseWriteInput(
    String id,
    String version,
    String channel,
    String status,
    String minVersion,
    boolean forceUpdate,
    int rolloutPercent,
    String notes,
    TargetingRules targeting,
    String scriptId,
    String scriptVersion,
    String scriptSha256,
    Instant createdAt,
    Instant publishedAt,
    Instant pausedAt,
    Instant rolledBackAt,
    String rolledBackTo
) {
}
