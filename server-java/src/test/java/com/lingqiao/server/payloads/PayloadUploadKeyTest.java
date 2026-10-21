package com.lingqiao.server.payloads;

import static org.assertj.core.api.Assertions.assertThat;

import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class PayloadUploadKeyTest {
    @TempDir
    Path tempDir;

    @Test
    void configuredKeyWinsWithoutWritingAFile() {
        PayloadUploadKey key = PayloadUploadKey.load(tempDir, "configured");

        assertThat(key.matches("configured")).isTrue();
        assertThat(key.matches("wrong")).isFalse();
        assertThat(Files.exists(tempDir.resolve("upload_key"))).isFalse();
    }

    @Test
    void generatedKeyIsPersistedAndReusedByBothBackends() throws Exception {
        PayloadUploadKey generated = PayloadUploadKey.load(tempDir, "");
        String stored = Files.readString(tempDir.resolve("upload_key"));
        PayloadUploadKey reloaded = PayloadUploadKey.load(tempDir, "");

        assertThat(stored).hasSize(48);
        assertThat(generated.matches(stored)).isTrue();
        assertThat(reloaded.matches(stored)).isTrue();
    }

    @Test
    void blankLegacyKeyFileIsRepairedLikeGo() throws Exception {
        Files.writeString(tempDir.resolve("upload_key"), " \r\n");

        PayloadUploadKey key = PayloadUploadKey.load(tempDir, "");
        String stored = Files.readString(tempDir.resolve("upload_key"));

        assertThat(stored).hasSize(48);
        assertThat(key.matches(stored)).isTrue();
    }
}
