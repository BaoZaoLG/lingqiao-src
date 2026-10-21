package com.lingqiao.server.releases;

import com.fasterxml.jackson.annotation.JsonProperty;

public record SignedUpdateManifest(
    @JsonProperty("manifest") UpdateManifest manifest,
    @JsonProperty("signature") String signature
) {
}
