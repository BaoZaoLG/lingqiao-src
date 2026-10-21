package com.lingqiao.server.scripts;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.Instant;

public record ScriptModule(
    String id,
    String version,
    String sha256,
    String content,
    int size,
    @JsonInclude(JsonInclude.Include.NON_EMPTY) String note,
    String status,
    @JsonProperty("created_at") Instant createdAt,
    @JsonProperty("updated_at") Instant updatedAt,
    @JsonInclude(JsonInclude.Include.NON_NULL)
    @JsonProperty("activated_at") Instant activatedAt
) {
}
