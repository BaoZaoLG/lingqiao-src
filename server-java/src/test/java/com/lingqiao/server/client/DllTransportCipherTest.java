package com.lingqiao.server.client;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.lingqiao.server.common.errors.SidecarException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HexFormat;
import org.junit.jupiter.api.Test;

class DllTransportCipherTest {
    @Test
    void keyDerivationMatchesGoGoldenVectors() throws Exception {
        Path path = Path.of("..", "docs", "api", "dll-transport-golden-vectors.json");
        JsonNode root = new ObjectMapper().readTree(Files.readString(path));

        assertThat(root.path("version").asInt()).isEqualTo(1);
        JsonNode dllKey = root.path("dll_key");
        assertThat(dllKey.path("salt").asText()).isEqualTo(DllTransportCipher.DLL_SALT);
        assertThat(dllKey.path("iterations").asInt()).isEqualTo(DllTransportCipher.PBKDF2_ITERATIONS);
        assertThat(dllKey.path("key_length").asInt()).isEqualTo(DllTransportCipher.KEY_LENGTH_BYTES);

        for (JsonNode testCase : dllKey.path("cases")) {
            byte[] key = DllTransportCipher.deriveKey(testCase.path("client_secret").asText());
            assertThat(HexFormat.of().formatHex(key))
                .isEqualTo(testCase.path("expected_key_hex").asText());
        }

        JsonNode transport = root.path("dll_transport");
        assertThat(transport.path("nonce_bytes").asInt()).isEqualTo(DllTransportCipher.GCM_NONCE_BYTES);
        assertThat(transport.path("tag_bytes").asInt()).isEqualTo(DllTransportCipher.GCM_TAG_BITS / 8);
    }

    @Test
    void wireFormatIsNoncePlusSealedPayload() {
        byte[] key = DllTransportCipher.deriveKey("test-secret");
        byte[] plaintext = "MZ-dll-body".getBytes(StandardCharsets.UTF_8);

        byte[] wire = DllTransportCipher.encrypt(plaintext, key);

        assertThat(wire).hasSize(DllTransportCipher.GCM_NONCE_BYTES
            + plaintext.length
            + DllTransportCipher.GCM_TAG_BITS / 8);
        assertThat(DllTransportCipher.decrypt(wire, key)).isEqualTo(plaintext);
    }

    @Test
    void fixedNonceProducesDeterministicCiphertext() {
        byte[] key = DllTransportCipher.deriveKey("test-secret");
        byte[] nonce = HexFormat.of().parseHex("000102030405060708090a0b");
        byte[] plaintext = "deterministic".getBytes(StandardCharsets.UTF_8);

        byte[] first = DllTransportCipher.encryptWithNonce(plaintext, key, nonce);
        byte[] second = DllTransportCipher.encryptWithNonce(plaintext, key, nonce);

        assertThat(first).isEqualTo(second);
        assertThat(HexFormat.of().formatHex(first)).startsWith("000102030405060708090a0b");
    }

    @Test
    void tamperedPayloadAndBadInputsAreRejected() {
        byte[] key = DllTransportCipher.deriveKey("test-secret");
        byte[] wire = DllTransportCipher.encrypt("body".getBytes(StandardCharsets.UTF_8), key);
        wire[wire.length - 1] ^= 0x01;

        assertThatThrownBy(() -> DllTransportCipher.decrypt(wire, key))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("DLL decryption failed");
        assertThatThrownBy(() -> DllTransportCipher.decrypt(new byte[4], key))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid DLL payload");
        assertThatThrownBy(() -> DllTransportCipher.encrypt(new byte[1], new byte[7]))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid AES key length");
        assertThatThrownBy(() -> DllTransportCipher.deriveKey(""))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("client key unavailable");
    }
}
