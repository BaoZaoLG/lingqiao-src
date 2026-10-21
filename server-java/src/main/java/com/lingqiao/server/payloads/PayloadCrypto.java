package com.lingqiao.server.payloads;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.nio.charset.StandardCharsets;
import java.util.HexFormat;
import javax.crypto.Cipher;
import javax.crypto.Mac;
import javax.crypto.SecretKeyFactory;
import javax.crypto.spec.IvParameterSpec;
import javax.crypto.spec.PBEKeySpec;
import javax.crypto.spec.SecretKeySpec;

/**
 * Payload key-exchange crypto mirroring Go payloads handler:
 * PBKDF2-HMAC-SHA256 KEK, AES-256-CBC/PKCS7 key wrapping, HMAC-SHA256 tag,
 * and AES-256-CTR payload encryption.
 */
public final class PayloadCrypto {
    public static final int PBKDF2_ITERATIONS = 100_000;
    public static final int KEY_LENGTH_BYTES = 32;

    private PayloadCrypto() {
    }

    public static byte[] deriveKek(String normalizedCardCode, String machineId, String payloadId) {
        String material = nullToEmpty(normalizedCardCode) + nullToEmpty(machineId);
        byte[] salt = ("payload_key_" + nullToEmpty(payloadId)).getBytes(StandardCharsets.UTF_8);
        try {
            PBEKeySpec spec = new PBEKeySpec(
                material.toCharArray(), salt, PBKDF2_ITERATIONS, KEY_LENGTH_BYTES * 8);
            return SecretKeyFactory.getInstance("PBKDF2WithHmacSHA256").generateSecret(spec).getEncoded();
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to derive payload KEK");
        }
    }

    public static byte[] combineKeyMaterial(byte[] aesKey, byte[] hmacKey, byte[] iv) {
        requireLength(aesKey, 32, "aes_key");
        requireLength(hmacKey, 32, "hmac_key");
        requireLength(iv, 16, "iv");
        byte[] combined = new byte[80];
        System.arraycopy(aesKey, 0, combined, 0, 32);
        System.arraycopy(hmacKey, 0, combined, 32, 32);
        System.arraycopy(iv, 0, combined, 64, 16);
        return combined;
    }

    public static byte[] wrapKeyMaterial(byte[] kek, byte[] combined, byte[] encryptionIv) {
        requireLength(kek, 32, "kek");
        requireLength(encryptionIv, 16, "encryption_iv");
        try {
            Cipher cipher = Cipher.getInstance("AES/CBC/PKCS5Padding");
            cipher.init(Cipher.ENCRYPT_MODE, new SecretKeySpec(kek, "AES"), new IvParameterSpec(encryptionIv));
            return cipher.doFinal(combined);
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to wrap payload key material");
        }
    }

    public static byte[] macKeyMaterial(byte[] kek, byte[] wrapped) {
        try {
            Mac mac = Mac.getInstance("HmacSHA256");
            mac.init(new SecretKeySpec(kek, "HmacSHA256"));
            return mac.doFinal(wrapped);
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to mac payload key material");
        }
    }

    public static byte[] encryptPayload(byte[] aesKey, byte[] iv, byte[] plaintext) {
        requireLength(aesKey, 32, "aes_key");
        requireLength(iv, 16, "iv");
        try {
            Cipher cipher = Cipher.getInstance("AES/CTR/NoPadding");
            cipher.init(Cipher.ENCRYPT_MODE, new SecretKeySpec(aesKey, "AES"), new IvParameterSpec(iv));
            return cipher.doFinal(plaintext);
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to encrypt payload");
        }
    }

    public static String hex(byte[] data) {
        return HexFormat.of().formatHex(data);
    }

    public static byte[] parseHex(String value, int expectedLength, String field) {
        byte[] decoded;
        try {
            decoded = HexFormat.of().parseHex(value == null ? "" : value);
        } catch (IllegalArgumentException ex) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, field + " must be valid hex");
        }
        requireLength(decoded, expectedLength, field);
        return decoded;
    }

    private static void requireLength(byte[] value, int expected, String field) {
        if (value == null || value.length != expected) {
            throw new SidecarException(ErrorCode.BAD_REQUEST,
                field + " must be " + expected + " bytes");
        }
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }
}
