package com.lingqiao.server.releases;

import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.annotation.JsonInclude;
import java.util.List;

@JsonInclude(JsonInclude.Include.NON_EMPTY)
public record TargetingRules(
    @JsonProperty("allow_cards") List<String> allowCards,
    @JsonProperty("deny_cards") List<String> denyCards,
    @JsonProperty("allow_machines") List<String> allowMachines,
    @JsonProperty("deny_machines") List<String> denyMachines,
    @JsonProperty("allow_agents") List<String> allowAgents,
    @JsonProperty("deny_agents") List<String> denyAgents
) {
    public static TargetingRules empty() {
        return new TargetingRules(List.of(), List.of(), List.of(), List.of(), List.of(), List.of());
    }
}
