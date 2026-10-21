package com.lingqiao.server.releases;

import static org.assertj.core.api.Assertions.assertThat;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.HexFormat;
import org.junit.jupiter.api.Test;

class ManifestSignerGoldenTest {
    @Test
    void matchesGoCanonicalFormAndSignatures() throws Exception {
        Path path = Path.of("..", "docs", "api", "release-manifest-golden-vectors.json");
        JsonNode root = new ObjectMapper().readTree(Files.readString(path));

        assertThat(root.path("version").asInt()).isEqualTo(1);
        assertThat(root.path("algorithm").asText()).isEqualTo("Ed25519");

        byte[] seed = HexFormat.of().parseHex(root.path("seed_hex").asText());
        ManifestSigner signer = new ManifestSigner(seed);
        assertThat(signer.publicKeyHex()).isEqualTo(root.path("public_key_hex").asText());

        for (JsonNode testCase : root.path("cases")) {
            JsonNode m = testCase.path("manifest");
            UpdateManifest manifest = new UpdateManifest(
                m.path("release_id").asText(),
                m.path("version").asText(),
                m.path("channel").asText(),
                m.path("min_version").asText(""),
                m.path("force_update").asBoolean(),
                m.path("release_notes").asText(""),
                m.path("package_id").asText(),
                m.path("package_kind").asText(""),
                m.path("package_url").asText(),
                m.path("package_size").asLong(),
                m.path("package_sha256").asText(),
                m.path("script_id").asText(""),
                m.path("script_version").asText(""),
                m.path("script_sha256").asText(""),
                m.path("rollout_percent").asInt(),
                Instant.parse(m.path("created_at").asText())
            );

            String canonical = new String(ManifestSigner.canonical(manifest), StandardCharsets.UTF_8);
            assertThat(canonical)
                .as("canonical %s", testCase.path("name").asText())
                .isEqualTo(testCase.path("canonical").asText());

            SignedUpdateManifest signed = signer.sign(manifest);
            assertThat(signed.signature())
                .as("signature %s", testCase.path("name").asText())
                .isEqualTo(testCase.path("signature").asText());
            assertThat(ManifestSigner.verify(root.path("public_key_hex").asText(), signed)).isTrue();

            SignedUpdateManifest tampered = new SignedUpdateManifest(
                new UpdateManifest(
                    manifest.releaseId(), manifest.version(), manifest.channel(), manifest.minVersion(),
                    manifest.forceUpdate(), manifest.releaseNotes(), manifest.packageId(), manifest.packageKind(),
                    manifest.packageUrl(), manifest.packageSize(), manifest.packageSha256(), manifest.scriptId(),
                    manifest.scriptVersion(), manifest.scriptSha256(), manifest.rolloutPercent() + 1,
                    manifest.createdAt()),
                signed.signature());
            assertThat(ManifestSigner.verify(root.path("public_key_hex").asText(), tampered)).isFalse();
        }
    }
}
