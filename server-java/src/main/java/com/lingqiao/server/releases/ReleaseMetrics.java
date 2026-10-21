package com.lingqiao.server.releases;

import com.fasterxml.jackson.annotation.JsonProperty;

/**
 * Aggregated release event counters mirroring Go releases.ReleaseMetrics,
 * including its JSON field names.
 */
public record ReleaseMetrics(
    @JsonProperty("release_id") String releaseId,
    int offered,
    @JsonProperty("download_started") int downloadStarted,
    @JsonProperty("download_failed") int downloadFailed,
    @JsonProperty("download_completed") int downloadCompleted,
    @JsonProperty("install_started") int installStarted,
    @JsonProperty("install_success") int installSuccess,
    @JsonProperty("install_failed") int installFailed,
    int rollback,
    int dismissed,
    @JsonProperty("unique_machines") int uniqueMachines
) {
    public static ReleaseMetrics empty(String releaseId) {
        return new ReleaseMetrics(releaseId, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0);
    }
}
