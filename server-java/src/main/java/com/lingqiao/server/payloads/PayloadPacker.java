package com.lingqiao.server.payloads;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.security.MessageDigest;
import java.security.SecureRandom;
import java.time.Instant;
import java.util.ArrayList;
import java.util.List;

/**
 * Packs a raw DLL into AES-256-CTR encrypted chunks.
 * Mirrors Go payloads.PackPlainDLL including MZ validation and chunk layout.
 */
public final class PayloadPacker {
    public static final int DEFAULT_CHUNK_SIZE = 1 << 20;
    public static final int MAX_PAYLOAD_ID_LENGTH = 64;

    private final SecureRandom random = new SecureRandom();

    public PackedPayload pack(String payloadId, byte[] dll, int chunkSize) {
        validatePayloadId(payloadId);
        if (dll == null || dll.length < 2 || dll[0] != 'M' || dll[1] != 'Z') {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "invalid DLL payload");
        }
        int effectiveChunkSize = chunkSize <= 0 ? DEFAULT_CHUNK_SIZE : chunkSize;

        byte[] aesKey = randomBytes(32);
        byte[] hmacKey = randomBytes(32);
        byte[] iv = randomBytes(16);
        byte[] encrypted = PayloadCrypto.encryptPayload(aesKey, iv, dll);

        List<byte[]> chunks = new ArrayList<>();
        for (int offset = 0; offset < encrypted.length; offset += effectiveChunkSize) {
            int end = Math.min(offset + effectiveChunkSize, encrypted.length);
            byte[] chunk = new byte[end - offset];
            System.arraycopy(encrypted, offset, chunk, 0, chunk.length);
            chunks.add(chunk);
        }

        PayloadKeyMaterial material = new PayloadKeyMaterial(
            PayloadCrypto.hex(aesKey), PayloadCrypto.hex(hmacKey), PayloadCrypto.hex(iv));
        PayloadDocument document = new PayloadDocument();
        document.setPayloadId(payloadId);
        document.setExeHash(sha256Hex(dll));
        document.setChunkCount(chunks.size());
        document.setChunkSize(effectiveChunkSize);
        document.setTotalSize(encrypted.length);
        document.setCreatedAt(Instant.now());
        document.setAesKey(material.aesKeyHex());
        document.setHmacKey(material.hmacKeyHex());
        document.setIv(material.ivHex());
        return new PackedPayload(document, material, chunks);
    }

    public static void validatePayloadId(String id) {
        if (id == null || id.isEmpty()) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "payload_id cannot be empty");
        }
        if (id.length() > MAX_PAYLOAD_ID_LENGTH) {
            throw new SidecarException(ErrorCode.BAD_REQUEST,
                "payload_id exceeds " + MAX_PAYLOAD_ID_LENGTH + " characters");
        }
        if (id.contains("..")) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "payload_id cannot contain ..");
        }
        for (int i = 0; i < id.length(); i++) {
            char c = id.charAt(i);
            boolean allowed = (c >= 'A' && c <= 'Z')
                || (c >= 'a' && c <= 'z')
                || (c >= '0' && c <= '9')
                || c == '.' || c == '_' || c == '-';
            if (!allowed) {
                throw new SidecarException(ErrorCode.BAD_REQUEST,
                    "payload_id contains invalid character: " + c);
            }
        }
    }

    public String generatePayloadId() {
        byte[] suffix = randomBytes(8);
        String timestamp = java.time.format.DateTimeFormatter
            .ofPattern("yyyyMMdd-HHmmss")
            .withZone(java.time.ZoneOffset.UTC)
            .format(Instant.now());
        return "payload-" + timestamp + "-" + PayloadCrypto.hex(suffix);
    }

    static String sha256Hex(byte[] data) {
        try {
            return PayloadCrypto.hex(MessageDigest.getInstance("SHA-256").digest(data));
        } catch (Exception ex) {
            throw new IllegalStateException("sha256 unavailable", ex);
        }
    }

    private byte[] randomBytes(int size) {
        byte[] value = new byte[size];
        random.nextBytes(value);
        return value;
    }

    public record PayloadKeyMaterial(String aesKeyHex, String hmacKeyHex, String ivHex) {
    }

    public record PackedPayload(PayloadDocument document, PayloadKeyMaterial keyMaterial, List<byte[]> chunks) {
    }
}
