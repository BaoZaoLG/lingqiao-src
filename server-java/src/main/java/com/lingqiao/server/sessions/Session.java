package com.lingqiao.server.sessions;

import com.fasterxml.jackson.annotation.JsonProperty;
import java.time.Instant;

public record Session(
    String token,
    @JsonProperty("card_code") String cardCode,
    @JsonProperty("machine_id") String machineId,
    String fingerprint,
    @JsonProperty("client_version") String clientVersion,
    @JsonProperty("created_at") Instant createdAt,
    @JsonProperty("last_seen") Instant lastSeen,
    @JsonProperty("expires_at") Instant expiresAt,
    @JsonProperty("remote_addr") String remoteAddr
) {
}
