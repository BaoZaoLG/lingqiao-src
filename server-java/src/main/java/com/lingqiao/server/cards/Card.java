package com.lingqiao.server.cards;

import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.Instant;

public record Card(
    String code,
    @JsonProperty("machine_id") String machineId,
    @JsonProperty("agent_id") String agentId,
    @JsonProperty("created_at") Instant createdAt,
    @JsonProperty("activated_at") Instant activatedAt,
    @JsonProperty("expires_at") Instant expiresAt,
    String status,
    String note,
    @JsonProperty("max_sessions") int maxSessions
) {
}
