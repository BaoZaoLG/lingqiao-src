package com.lingqiao.server.client;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import com.lingqiao.server.payloads.PayloadStateReader;
import com.lingqiao.server.payloads.PayloadsWriteService;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class DllSourceResolverTest {
    @TempDir
    Path tempDir;

    private DllSourceResolver resolver;

    @BeforeEach
    void setUp() {
        resolver = new DllSourceResolver(tempDir, new PayloadStateReader(tempDir));
    }

    @Test
    void activePayloadIsPreferredAndDecryptedBackToPlainDll() {
        byte[] dll = dllBytes(700);
        PayloadsWriteService writer = new PayloadsWriteService(tempDir);
        writer.upload("payload-dll", dll, 256);
        writer.activate("payload-dll");

        DllSourceResolver.DllSource source = resolver.resolve();

        assertThat(source.kind()).isEqualTo("active-payload");
        assertThat(source.payloadId()).isEqualTo("payload-dll");
        assertThat(source.bytes()).isEqualTo(dll);
        assertThat(source.sha256()).isEqualTo(DllSourceResolver.sha256Hex(dll));
    }

    @Test
    void legacyFileIsUsedWhenNoActivePayload() throws Exception {
        byte[] legacy = dllBytes(64);
        Files.write(tempDir.resolve("CefHook.dll"), legacy);

        DllSourceResolver.DllSource source = resolver.resolve();

        assertThat(source.kind()).isEqualTo("legacy-file");
        assertThat(source.payloadId()).isEmpty();
        assertThat(source.bytes()).isEqualTo(legacy);
    }

    @Test
    void brokenActivePayloadFallsBackToLegacyFile() throws Exception {
        byte[] dll = dllBytes(300);
        PayloadsWriteService writer = new PayloadsWriteService(tempDir);
        writer.upload("payload-broken", dll, 128);
        writer.activate("payload-broken");
        Files.delete(tempDir.resolve("payloads").resolve("payload-broken").resolve("chunk_0001.bin"));

        byte[] legacy = "MZ-legacy".getBytes(StandardCharsets.UTF_8);
        Files.write(tempDir.resolve("CefHook.dll"), legacy);

        DllSourceResolver.DllSource source = resolver.resolve();

        assertThat(source.kind()).isEqualTo("legacy-file");
        assertThat(source.bytes()).isEqualTo(legacy);
    }

    @Test
    void missingSourcesAreReported() {
        assertThatThrownBy(() -> resolver.resolve())
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("DLL not found on server");
    }

    private static byte[] dllBytes(int size) {
        byte[] dll = new byte[Math.max(size, 2)];
        dll[0] = 'M';
        dll[1] = 'Z';
        for (int i = 2; i < dll.length; i++) {
            dll[i] = (byte) (i % 199);
        }
        return dll;
    }
}
