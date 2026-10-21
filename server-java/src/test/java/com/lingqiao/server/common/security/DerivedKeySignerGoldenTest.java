package com.lingqiao.server.common.security;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.lingqiao.server.common.errors.SidecarException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HexFormat;
import org.junit.jupiter.api.Test;

class DerivedKeySignerGoldenTest {
    @Test
    void derivedKeyMatchesGoGoldenVectors() throws Exception {
        JsonNode root = golden();
        JsonNode derived = root.path("derived_key");

        assertThat(derived.path("salt").asText()).isEqualTo(DerivedKeySigner.HMAC_SALT);
        assertThat(derived.path("iterations").asInt()).isEqualTo(DerivedKeySigner.PBKDF2_ITERATIONS);
        assertThat(derived.path("key_length").asInt()).isEqualTo(DerivedKeySigner.KEY_LENGTH_BYTES);

        for (JsonNode testCase : derived.path("cases")) {
            byte[] key = DerivedKeySigner.deriveKey(testCase.path("client_secret").asText());
            assertThat(HexFormat.of().formatHex(key))
                .isEqualTo(testCase.path("expected_key_hex").asText());
        }
    }

    @Test
    void manifestHmacMatchesGoGoldenVectors() throws Exception {
        for (JsonNode testCase : golden().path("manifest_hmac").path("cases")) {
            byte[] key = DerivedKeySigner.deriveKey(testCase.path("client_secret").asText());
            String signature = DerivedKeySigner.signWithDerivedKey(key, testCase.path("payload").asText());
            assertThat(signature)
                .as("manifest hmac for %s", testCase.path("payload").asText())
                .isEqualTo(testCase.path("expected_hmac_hex").asText());
        }
    }

    @Test
    void blankSecretIsRejected() {
        assertThatThrownBy(() -> DerivedKeySigner.deriveKey(""))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("client key unavailable");
    }

    private static JsonNode golden() throws Exception {
        Path path = Path.of("..", "docs", "api", "manifest-hmac-golden-vectors.json");
        JsonNode root = new ObjectMapper().readTree(Files.readString(path));
        assertThat(root.path("version").asInt()).isEqualTo(1);
        return root;
    }
}
