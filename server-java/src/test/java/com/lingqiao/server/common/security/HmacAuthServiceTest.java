package com.lingqiao.server.common.security;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatCode;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.HexFormat;
import javax.crypto.Mac;
import javax.crypto.SecretKeyFactory;
import javax.crypto.spec.PBEKeySpec;
import javax.crypto.spec.SecretKeySpec;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class HmacAuthServiceTest {
    @TempDir
    Path tempDir;

    @Test
    void acceptsValidSignatureAndRejectsReplay() throws Exception {
        Path secretFile = tempDir.resolve("hmac_secret.key");
        Files.writeString(secretFile, "test-secret");
        HmacAuthService service = new HmacAuthService("", secretFile.toString());

        String timestamp = String.valueOf(Instant.now().getEpochSecond());
        String nonce = "nonce-1";
        String body = "{\"ok\":true}";
        String signature = sign("test-secret", timestamp + "|" + nonce + "|" + body);

        assertThatCode(() -> service.verify("injector_v1", body, signature, timestamp, nonce))
            .doesNotThrowAnyException();
        assertThatThrownBy(() -> service.verify("injector_v1", body, signature, timestamp, nonce))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("nonce already used");
    }

    @Test
    void signsManifestWithSecretLoadedFromFile() throws Exception {
        Path secretFile = tempDir.resolve("hmac_secret.key");
        Files.writeString(secretFile, "file-secret");
        HmacAuthService service = new HmacAuthService("", secretFile.toString());

        assertThat(service.signDerived("injector_v1", "{\"version\":\"1.0.0\"}"))
            .isEqualTo(sign("file-secret", "{\"version\":\"1.0.0\"}"));
    }

    private static String sign(String secret, String data) throws Exception {
        PBEKeySpec spec = new PBEKeySpec(
            secret.toCharArray(),
            "CefBridge-HMAC-Salt-v2".getBytes(StandardCharsets.UTF_8),
            100_000,
            256
        );
        byte[] key = SecretKeyFactory.getInstance("PBKDF2WithHmacSHA256").generateSecret(spec).getEncoded();
        Mac mac = Mac.getInstance("HmacSHA256");
        mac.init(new SecretKeySpec(key, "HmacSHA256"));
        return HexFormat.of().formatHex(mac.doFinal(data.getBytes(StandardCharsets.UTF_8)));
    }
}
