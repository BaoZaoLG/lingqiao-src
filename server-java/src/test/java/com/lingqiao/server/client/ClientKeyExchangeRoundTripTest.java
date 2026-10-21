package com.lingqiao.server.client;

import static org.assertj.core.api.Assertions.assertThat;

import com.lingqiao.server.cards.CardCodes;
import com.lingqiao.server.payloads.PayloadCrypto;
import com.lingqiao.server.payloads.PayloadPacker;
import com.lingqiao.server.payloads.PayloadsWriteService;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

/**
 * End-to-end check that a Java-packed payload can be unwrapped with the KEK
 * derived from card code + machine id, exactly like the Go client flow.
 */
class ClientKeyExchangeRoundTripTest {
    @TempDir
    Path tempDir;

    @Test
    void packedPayloadKeyMaterialUnwrapsAndDecryptsDll() throws Exception {
        PayloadsWriteService writer = new PayloadsWriteService(tempDir);
        byte[] dll = new byte[512];
        dll[0] = 'M';
        dll[1] = 'Z';
        for (int i = 2; i < dll.length; i++) {
            dll[i] = (byte) (i % 97);
        }

        PayloadPacker.PackedPayload packed = writer.upload("payload-kx", dll, 128);
        String cardCode = "ABCDEF-GHJKMN-PQRSTV";
        String machineId = "MACHINE-KX";

        byte[] kek = PayloadCrypto.deriveKek(
            CardCodes.normalize(cardCode), machineId, packed.document().getPayloadId());
        byte[] aesKey = PayloadCrypto.parseHex(packed.keyMaterial().aesKeyHex(), 32, "aes_key");
        byte[] hmacKey = PayloadCrypto.parseHex(packed.keyMaterial().hmacKeyHex(), 32, "hmac_key");
        byte[] iv = PayloadCrypto.parseHex(packed.keyMaterial().ivHex(), 16, "iv");

        byte[] combined = PayloadCrypto.combineKeyMaterial(aesKey, hmacKey, iv);
        byte[] encryptionIv = new byte[16];
        for (int i = 0; i < encryptionIv.length; i++) {
            encryptionIv[i] = (byte) (0x30 + i);
        }
        byte[] wrapped = PayloadCrypto.wrapKeyMaterial(kek, combined, encryptionIv);
        byte[] tag = PayloadCrypto.macKeyMaterial(kek, wrapped);

        // Client side: recompute KEK, verify tag, unwrap, then decrypt chunks.
        byte[] clientKek = PayloadCrypto.deriveKek(
            CardCodes.normalize("abcdef ghjkmn pqrstv"), machineId, packed.document().getPayloadId());
        assertThat(clientKek).isEqualTo(kek);
        assertThat(PayloadCrypto.macKeyMaterial(clientKek, wrapped)).isEqualTo(tag);

        javax.crypto.Cipher cipher = javax.crypto.Cipher.getInstance("AES/CBC/PKCS5Padding");
        cipher.init(javax.crypto.Cipher.DECRYPT_MODE,
            new javax.crypto.spec.SecretKeySpec(clientKek, "AES"),
            new javax.crypto.spec.IvParameterSpec(encryptionIv));
        byte[] unwrapped = cipher.doFinal(wrapped);
        assertThat(unwrapped).hasSize(80).isEqualTo(combined);

        byte[] recoveredAesKey = java.util.Arrays.copyOfRange(unwrapped, 0, 32);
        byte[] recoveredIv = java.util.Arrays.copyOfRange(unwrapped, 64, 80);

        byte[] ciphertext = new byte[packed.document().getTotalSize()];
        int offset = 0;
        for (int i = 0; i < packed.document().getChunkCount(); i++) {
            byte[] chunk = java.nio.file.Files.readAllBytes(
                tempDir.resolve("payloads").resolve("payload-kx")
                    .resolve(String.format("chunk_%04d.bin", i)));
            System.arraycopy(chunk, 0, ciphertext, offset, chunk.length);
            offset += chunk.length;
        }

        assertThat(PayloadCrypto.encryptPayload(recoveredAesKey, recoveredIv, ciphertext)).isEqualTo(dll);
    }

    @Test
    void packedMetadataPersistsKeyMaterialForKeyExchange() {
        PayloadsWriteService writer = new PayloadsWriteService(tempDir);
        byte[] dll = {'M', 'Z', 1, 2, 3};

        PayloadPacker.PackedPayload packed = writer.upload("payload-meta", dll, 0);

        assertThat(packed.document().getAesKey()).isEqualTo(packed.keyMaterial().aesKeyHex());
        assertThat(packed.document().getHmacKey()).isEqualTo(packed.keyMaterial().hmacKeyHex());
        assertThat(packed.document().getIv()).isEqualTo(packed.keyMaterial().ivHex());

        var reader = new com.lingqiao.server.payloads.PayloadStateReader(tempDir);
        var stored = reader.read().getPayloads().get("payload-meta");
        assertThat(stored).isNotNull();
        assertThat(stored.getAesKey()).hasSize(64);
        assertThat(stored.getIv()).hasSize(32);
    }
}
