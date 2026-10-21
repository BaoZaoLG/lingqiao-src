package com.lingqiao.server.system;

import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.Instant;

public record BlacklistEntry(
    String type,
    String value,
    String reason,
    @JsonProperty("created_at") Instant createdAt
) {
}
