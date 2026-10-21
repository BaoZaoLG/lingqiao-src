package com.lingqiao.server.releases;

import static org.assertj.core.api.Assertions.assertThat;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.util.List;
import org.junit.jupiter.api.Test;

class ReleaseSelectorGoldenTest {
    @Test
    void rolloutBucketsAndVersionCompareMatchGo() throws Exception {
        Path path = Path.of("..", "docs", "api", "release-rollout-golden-vectors.json");
        JsonNode root = new ObjectMapper().readTree(Files.readString(path));
        assertThat(root.path("version").asInt()).isEqualTo(1);

        for (JsonNode bucketCase : root.path("rollout").path("buckets")) {
            String releaseId = bucketCase.path("release_id").asText();
            String key = bucketCase.path("key").asText();
            int expectedBucket = bucketCase.path("bucket").asInt();

            assertThat(bucketOf(releaseId, key)).as("bucket %s|%s", releaseId, key).isEqualTo(expectedBucket);

            ReleaseSelector.ClientContext client =
                new ReleaseSelector.ClientContext("1.0.0", "stable", "", key, "");
            assertThat(ReleaseSelector.inRollout(releaseId, expectedBucket + 1, client)).isTrue();
            assertThat(ReleaseSelector.inRollout(releaseId, expectedBucket, client)).isFalse();
        }

        for (JsonNode versionCase : root.path("version_compare")) {
            assertThat(ReleaseSelector.versionGreater(
                versionCase.path("candidate").asText(),
                versionCase.path("current").asText()))
                .as("versionGreater %s vs %s",
                    versionCase.path("candidate").asText(), versionCase.path("current").asText())
                .isEqualTo(versionCase.path("greater").asBoolean());
        }
    }

    @Test
    void rolloutBoundsMatchGoSemantics() {
        ReleaseSelector.ClientContext client =
            new ReleaseSelector.ClientContext("1.0.0", "stable", "", "MACHINE-A", "");
        assertThat(ReleaseSelector.inRollout("rel-001", 0, client)).isFalse();
        assertThat(ReleaseSelector.inRollout("rel-001", -5, client)).isFalse();
        assertThat(ReleaseSelector.inRollout("rel-001", 100, client)).isTrue();
        assertThat(ReleaseSelector.inRollout("rel-001", 500, client)).isTrue();
    }

    @Test
    void targetingDenyBeatsAllowAndEmptyAllowPassesAll() {
        ReleaseSelector.ClientContext client =
            new ReleaseSelector.ClientContext("1.0.0", "stable", "CARD-1", "M1", "agent-1");

        assertThat(ReleaseSelector.matchesTargeting(TargetingRules.empty(), client)).isTrue();
        assertThat(ReleaseSelector.matchesTargeting(null, client)).isTrue();

        TargetingRules denyMachine = new TargetingRules(
            List.of("CARD-1"), List.of(), List.of(), List.of("m1"), List.of(), List.of());
        assertThat(ReleaseSelector.matchesTargeting(denyMachine, client)).isFalse();

        TargetingRules allowOther = new TargetingRules(
            List.of("CARD-999"), List.of(), List.of(), List.of(), List.of(), List.of());
        assertThat(ReleaseSelector.matchesTargeting(allowOther, client)).isFalse();

        TargetingRules allowAgentCaseInsensitive = new TargetingRules(
            List.of(), List.of(), List.of(), List.of(), List.of(" AGENT-1 "), List.of());
        assertThat(ReleaseSelector.matchesTargeting(allowAgentCaseInsensitive, client)).isTrue();
    }

    @Test
    void rolloutKeyFallsBackThroughClientIdentity() throws Exception {
        assertThat(ReleaseSelector.firstNonEmpty("", "", "", "")).isEqualTo("anonymous");
        assertThat(ReleaseSelector.firstNonEmpty("", "CARD-1", "agent")).isEqualTo("CARD-1");

        ReleaseSelector.ClientContext anonymous =
            new ReleaseSelector.ClientContext("", "stable", "", "", "");
        assertThat(ReleaseSelector.inRollout("rel-002", bucketOf("rel-002", "anonymous") + 1, anonymous)).isTrue();
    }

    private static int bucketOf(String releaseId, String key) throws Exception {
        byte[] digest = MessageDigest.getInstance("SHA-256")
            .digest((releaseId + "|" + key).getBytes(StandardCharsets.UTF_8));
        long high = 0;
        for (int i = 0; i < 8; i++) {
            high = (high << 8) | (digest[i] & 0xFFL);
        }
        return (int) Long.remainderUnsigned(high, 100);
    }
}
