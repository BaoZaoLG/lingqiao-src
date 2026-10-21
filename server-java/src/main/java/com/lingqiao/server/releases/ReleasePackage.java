package com.lingqiao.server.releases;

import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.Instant;

public record ReleasePackage(
    String id,
    @JsonProperty("release_id") String releaseId,
    String kind,
    String filename,
    String path,
    @JsonProperty("file_size") long fileSize,
    String sha256,
    @JsonProperty("created_at") Instant createdAt
) {
}
