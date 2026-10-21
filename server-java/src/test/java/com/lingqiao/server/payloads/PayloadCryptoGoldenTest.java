package com.lingqiao.server.payloads;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.lingqiao.server.common.errors.SidecarException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;

class PayloadCryptoGoldenTest {
    @Test
    void matchesGoPayloadCryptoVectors() throws Exception {
        Path path = Path.of("..", "docs", "api", "payload-crypto-golden-vectors.json");
        JsonNode root = new ObjectMapper().readTree(Files.readString(path));
        assertThat(root.path("version").asInt()).isEqualTo(1);

        JsonNode c = root.path("case");
        byte[] aesKey = PayloadCrypto.parseHex(c.path("aes_key_hex").asText(), 32, "aes_key");
        byte[] hmacKey = PayloadCrypto.parseHex(c.path("hmac_key_hex").asText(), 32, "hmac_key");
        byte[] iv = PayloadCrypto.parseHex(c.path("iv_hex").asText(), 16, "iv");
        byte[] encryptionIv = PayloadCrypto.parseHex(c.path("encryption_iv_hex").asText(), 16, "encryption_iv");

        byte[] kek = PayloadCrypto.deriveKek(
            c.path("normalized_card_code").asText(),
            c.path("machine_id").asText(),
            c.path("payload_id").asText());
        assertThat(PayloadCrypto.hex(kek)).isEqualTo(c.path("expected_kek_hex").asText());

        byte[] combined = PayloadCrypto.combineKeyMaterial(aesKey, hmacKey, iv);
        assertThat(combined).hasSize(80);

        byte[] wrapped = PayloadCrypto.wrapKeyMaterial(kek, combined, encryptionIv);
        assertThat(wrapped).hasSize(c.path("expected_padded_length").asInt());
        assertThat(PayloadCrypto.hex(wrapped)).isEqualTo(c.path("expected_encrypted_key_hex").asText());

        byte[] mac = PayloadCrypto.macKeyMaterial(kek, wrapped);
        assertThat(PayloadCrypto.hex(mac)).isEqualTo(c.path("expected_encrypted_hmac_hex").asText());

        byte[] ciphertext = PayloadCrypto.encryptPayload(
            aesKey, iv, c.path("plaintext_dll_utf8").asText().getBytes(StandardCharsets.UTF_8));
        assertThat(PayloadCrypto.hex(ciphertext))
            .isEqualTo(c.path("expected_ctr_ciphertext_hex").asText());
    }

    @Test
    void rejectsInvalidKeyMaterialLengths() {
        assertThatThrownBy(() -> PayloadCrypto.parseHex("00", 32, "aes_key"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("aes_key must be 32 bytes");
        assertThatThrownBy(() -> PayloadCrypto.parseHex("zz", 16, "iv"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("iv must be valid hex");
        assertThatThrownBy(() -> PayloadCrypto.combineKeyMaterial(new byte[31], new byte[32], new byte[16]))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("aes_key must be 32 bytes");
    }
}
