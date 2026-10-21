package com.lingqiao.server.client;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.nio.charset.StandardCharsets;
import java.security.SecureRandom;
import javax.crypto.Cipher;
import javax.crypto.SecretKeyFactory;
import javax.crypto.spec.GCMParameterSpec;
import javax.crypto.spec.PBEKeySpec;
import javax.crypto.spec.SecretKeySpec;

/**
 * DLL transport cipher mirroring Go auth.GetDLLKey + encryptDLLForClient.
 * Wire format: [12-byte GCM nonce][ciphertext][16-byte GCM tag].
 */
public final class DllTransportCipher {
    public static final String DLL_SALT = "CefBridge-DLL-Salt-v1";
    public static final int PBKDF2_ITERATIONS = 100_000;
    public static final int KEY_LENGTH_BYTES = 32;
    public static final int GCM_NONCE_BYTES = 12;
    public static final int GCM_TAG_BITS = 128;

    private static final SecureRandom RANDOM = new SecureRandom();

    private DllTransportCipher() {
    }

    public static byte[] deriveKey(String clientSecret) {
        if (clientSecret == null || clientSecret.isEmpty()) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "client key unavailable");
        }
        try {
            PBEKeySpec spec = new PBEKeySpec(
                clientSecret.toCharArray(),
                DLL_SALT.getBytes(StandardCharsets.UTF_8),
                PBKDF2_ITERATIONS,
                KEY_LENGTH_BYTES * 8);
            return SecretKeyFactory.getInstance("PBKDF2WithHmacSHA256").generateSecret(spec).getEncoded();
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to derive DLL key");
        }
    }

    public static byte[] encrypt(byte[] plaintext, byte[] key) {
        if (key == null || (key.length != 16 && key.length != 24 && key.length != 32)) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR,
                "invalid AES key length: " + (key == null ? 0 : key.length));
        }
        byte[] nonce = new byte[GCM_NONCE_BYTES];
        RANDOM.nextBytes(nonce);
        return encryptWithNonce(plaintext, key, nonce);
    }

    static byte[] encryptWithNonce(byte[] plaintext, byte[] key, byte[] nonce) {
        try {
            Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding");
            cipher.init(Cipher.ENCRYPT_MODE, new SecretKeySpec(key, "AES"),
                new GCMParameterSpec(GCM_TAG_BITS, nonce));
            byte[] sealed = cipher.doFinal(plaintext);
            byte[] out = new byte[nonce.length + sealed.length];
            System.arraycopy(nonce, 0, out, 0, nonce.length);
            System.arraycopy(sealed, 0, out, nonce.length, sealed.length);
            return out;
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "DLL encryption failed");
        }
    }

    public static byte[] decrypt(byte[] wire, byte[] key) {
        if (wire == null || wire.length <= GCM_NONCE_BYTES) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "invalid DLL payload");
        }
        try {
            byte[] nonce = new byte[GCM_NONCE_BYTES];
            System.arraycopy(wire, 0, nonce, 0, GCM_NONCE_BYTES);
            Cipher cipher = Cipher.getInstance("AES/GCM/NoPadding");
            cipher.init(Cipher.DECRYPT_MODE, new SecretKeySpec(key, "AES"),
                new GCMParameterSpec(GCM_TAG_BITS, nonce));
            return cipher.doFinal(wire, GCM_NONCE_BYTES, wire.length - GCM_NONCE_BYTES);
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "DLL decryption failed");
        }
    }
}
