package com.lingqiao.server.releases;

import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.annotation.JsonInclude;
import java.time.Instant;

public record Release(
    String id,
    String version,
    String channel,
    String status,
    @JsonProperty("min_version") @JsonInclude(JsonInclude.Include.NON_EMPTY) String minVersion,
    @JsonProperty("force_update") boolean forceUpdate,
    @JsonProperty("rollout_percent") int rolloutPercent,
    @JsonInclude(JsonInclude.Include.NON_EMPTY) String notes,
    TargetingRules targeting,
    @JsonProperty("script_id") @JsonInclude(JsonInclude.Include.NON_EMPTY) String scriptId,
    @JsonProperty("script_version") @JsonInclude(JsonInclude.Include.NON_EMPTY) String scriptVersion,
    @JsonProperty("script_sha256") @JsonInclude(JsonInclude.Include.NON_EMPTY) String scriptSha256,
    @JsonProperty("created_at") Instant createdAt,
    @JsonProperty("published_at") @JsonInclude(JsonInclude.Include.NON_NULL) Instant publishedAt,
    @JsonProperty("paused_at") @JsonInclude(JsonInclude.Include.NON_NULL) Instant pausedAt,
    @JsonProperty("rolled_back_at") @JsonInclude(JsonInclude.Include.NON_NULL) Instant rolledBackAt,
    @JsonProperty("rolled_back_to") @JsonInclude(JsonInclude.Include.NON_EMPTY) String rolledBackTo
) {
}
