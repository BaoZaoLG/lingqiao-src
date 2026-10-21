package com.lingqiao.server.payloads;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Instant;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class PayloadsWriteServiceTest {
    @TempDir
    Path tempDir;

    private PayloadsWriteService service;

    @BeforeEach
    void setUp() {
        service = new PayloadsWriteService(tempDir);
    }

    @Test
    void uploadWritesChunksAndMetadata() throws Exception {
        byte[] dll = dllBytes(2500);

        PayloadPacker.PackedPayload packed = service.upload("payload-a", dll, 1024);

        assertThat(packed.document().getChunkCount()).isEqualTo(3);
        assertThat(packed.document().getChunkSize()).isEqualTo(1024);
        assertThat(packed.document().getTotalSize()).isEqualTo(dll.length);
        assertThat(packed.document().getExeHash()).isEqualTo(PayloadPacker.sha256Hex(dll));
        assertThat(packed.keyMaterial().aesKeyHex()).hasSize(64);
        assertThat(packed.keyMaterial().ivHex()).hasSize(32);

        Path dir = tempDir.resolve("payloads").resolve("payload-a");
        assertThat(Files.exists(dir.resolve("chunk_0000.bin"))).isTrue();
        assertThat(Files.exists(dir.resolve("chunk_0002.bin"))).isTrue();
        assertThat(Files.exists(dir.resolveSibling("payload-a.tmp"))).isFalse();
        assertThat(Files.readString(tempDir.resolve("payloads.json"))).contains("payload-a");
    }

    @Test
    void chunkCiphertextDecryptsBackToOriginalDll() throws Exception {
        byte[] dll = dllBytes(3000);
        PayloadPacker.PackedPayload packed = service.upload("payload-b", dll, 1024);

        byte[] ciphertext = new byte[packed.document().getTotalSize()];
        int offset = 0;
        for (int i = 0; i < packed.document().getChunkCount(); i++) {
            byte[] chunk = Files.readAllBytes(
                tempDir.resolve("payloads").resolve("payload-b").resolve(String.format("chunk_%04d.bin", i)));
            System.arraycopy(chunk, 0, ciphertext, offset, chunk.length);
            offset += chunk.length;
        }

        byte[] aesKey = PayloadCrypto.parseHex(packed.keyMaterial().aesKeyHex(), 32, "aes_key");
        byte[] iv = PayloadCrypto.parseHex(packed.keyMaterial().ivHex(), 16, "iv");
        assertThat(PayloadCrypto.encryptPayload(aesKey, iv, ciphertext)).isEqualTo(dll);
    }

    @Test
    void activateAndDeleteEnforceGoRules() {
        service.upload("payload-c", dllBytes(100), 0);
        service.upload("payload-d", dllBytes(100), 0);

        service.activate("payload-c");
        assertThat(service.list()).anySatisfy(p -> {
            if (p.payloadId().equals("payload-c")) {
                assertThat(p.active()).isTrue();
            }
        });

        assertThatThrownBy(() -> service.delete("payload-c"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("cannot delete active payload");

        service.delete("payload-d");
        assertThat(Files.exists(tempDir.resolve("payloads").resolve("payload-d"))).isFalse();
        assertThatThrownBy(() -> service.delete("payload-d"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("payload not found");
        assertThatThrownBy(() -> service.activate("payload-missing"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("payload not found");
    }

    @Test
    void rejectsInvalidDllAndPayloadIds() {
        assertThatThrownBy(() -> service.upload("payload-e", "NOTMZ".getBytes(StandardCharsets.UTF_8), 0))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid DLL payload");
        assertThatThrownBy(() -> service.upload("../escape", dllBytes(10), 0))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("payload_id cannot contain ..");
        assertThatThrownBy(() -> service.upload("bad id", dllBytes(10), 0))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid character");
        assertThatThrownBy(() -> service.upload("x".repeat(65), dllBytes(10), 0))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("exceeds 64 characters");
    }

    @Test
    void generatedPayloadIdIsValid() {
        PayloadPacker.PackedPayload packed = service.upload(null, dllBytes(10), 0);
        assertThat(packed.document().getPayloadId()).startsWith("payload-");
        PayloadPacker.validatePayloadId(packed.document().getPayloadId());
    }

    @Test
    void adminUploadRejectsDuplicateIdInsteadOfOverwritingChunks() {
        service.upload("payload-duplicate", dllBytes(10), 4);

        assertThatThrownBy(() -> service.upload("payload-duplicate", dllBytes(20), 4))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("payload_id already exists");
    }

    @Test
    void legacyImportPersistsProvidedCiphertextAndKeyMaterial() throws Exception {
        PayloadDocument document = new PayloadDocument();
        document.setPayloadId("legacy-a");
        document.setAesKey("00".repeat(32));
        document.setHmacKey("11".repeat(32));
        document.setIv("22".repeat(16));
        document.setExeHash("33".repeat(32));
        document.setChunkCount(2);
        document.setChunkSize(3);
        document.setTotalSize(5);
        document.setCreatedAt(Instant.parse("2026-07-01T00:00:00Z"));

        Payload result = service.importEncrypted(
            document, List.of(new byte[]{1, 2, 3}, new byte[]{4, 5}));

        assertThat(result.payloadId()).isEqualTo("legacy-a");
        assertThat(Files.readAllBytes(
            tempDir.resolve("payloads/legacy-a/chunk_0000.bin"))).containsExactly(1, 2, 3);
        String state = Files.readString(tempDir.resolve("payloads.json"));
        assertThat(state).contains("\"aes_key\" : \"" + "00".repeat(32) + "\"");
    }

    @Test
    void concurrentAdminUploadsCannotCorruptTheSamePayloadId() throws Exception {
        var executor = Executors.newFixedThreadPool(2);
        try {
            CountDownLatch start = new CountDownLatch(1);
            java.util.concurrent.Callable<Boolean> upload = () -> {
                start.await();
                try {
                    service.upload("payload-race", dllBytes(64), 16);
                    return true;
                } catch (SidecarException ex) {
                    return false;
                }
            };
            Future<Boolean> first = executor.submit(upload);
            Future<Boolean> second = executor.submit(upload);
            start.countDown();

            assertThat(List.of(first.get(), second.get()))
                .containsExactlyInAnyOrder(true, false);
            assertThat(service.list()).extracting(Payload::payloadId)
                .containsExactly("payload-race");
            assertThat(Files.exists(
                tempDir.resolve("payloads/payload-race/chunk_0000.bin"))).isTrue();
        } finally {
            executor.shutdownNow();
        }
    }

    private static byte[] dllBytes(int size) {
        byte[] dll = new byte[Math.max(size, 2)];
        dll[0] = 'M';
        dll[1] = 'Z';
        for (int i = 2; i < dll.length; i++) {
            dll[i] = (byte) (i % 251);
        }
        return dll;
    }
}
