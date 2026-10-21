package com.lingqiao.server.releases;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonProperty;
import com.fasterxml.jackson.annotation.JsonPropertyOrder;
import java.time.Instant;

/**
 * Update manifest mirroring Go releases.Manifest, including field order and
 * omitempty semantics so canonical JSON matches byte-for-byte for signing.
 */
@JsonInclude(JsonInclude.Include.ALWAYS)
@JsonPropertyOrder({
    "release_id", "version", "channel", "min_version", "force_update", "release_notes",
    "package_id", "package_kind", "package_url", "package_size", "package_sha256",
    "script_id", "script_version", "script_sha256", "rollout_percent", "created_at"
})
public record UpdateManifest(
    @JsonProperty("release_id") String releaseId,
    @JsonProperty("version") String version,
    @JsonProperty("channel") String channel,
    @JsonInclude(JsonInclude.Include.NON_EMPTY) @JsonProperty("min_version") String minVersion,
    @JsonProperty("force_update") boolean forceUpdate,
    @JsonInclude(JsonInclude.Include.NON_EMPTY) @JsonProperty("release_notes") String releaseNotes,
    @JsonProperty("package_id") String packageId,
    @JsonInclude(JsonInclude.Include.NON_EMPTY) @JsonProperty("package_kind") String packageKind,
    @JsonProperty("package_url") String packageUrl,
    @JsonProperty("package_size") long packageSize,
    @JsonProperty("package_sha256") String packageSha256,
    @JsonInclude(JsonInclude.Include.NON_EMPTY) @JsonProperty("script_id") String scriptId,
    @JsonInclude(JsonInclude.Include.NON_EMPTY) @JsonProperty("script_version") String scriptVersion,
    @JsonInclude(JsonInclude.Include.NON_EMPTY) @JsonProperty("script_sha256") String scriptSha256,
    @JsonProperty("rollout_percent") int rolloutPercent,
    @JsonProperty("created_at") Instant createdAt
) {
}
