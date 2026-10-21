package com.lingqiao.server.payloads;

import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.Instant;

public record Payload(
    @JsonProperty("payload_id") String payloadId,
    @JsonProperty("exe_hash") String exeHash,
    @JsonProperty("chunk_count") int chunkCount,
    @JsonProperty("chunk_size") int chunkSize,
    @JsonProperty("total_size") int totalSize,
    @JsonProperty("created_at") Instant createdAt,
    boolean active
) {
}
