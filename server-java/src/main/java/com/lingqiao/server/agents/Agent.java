package com.lingqiao.server.agents;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.Instant;

/**
 * Admin agent summary mirroring Go agents.adminAgentSummary, including its
 * JSON field names and omit-empty behaviour.
 */
@JsonInclude(JsonInclude.Include.NON_NULL)
public record Agent(
    String id,
    String username,
    @JsonInclude(JsonInclude.Include.NON_EMPTY) String prefix,
    @JsonProperty("created_at") Instant createdAt,
    boolean disabled,
    @JsonProperty("total_cards") int totalCards,
    @JsonProperty("active_cards") int activeCards,
    @JsonProperty("expired_cards") int expiredCards,
    @JsonProperty("bound_machines") int boundMachines,
    @JsonProperty("last_card_created_at") Instant lastCardCreatedAt
) {
}
