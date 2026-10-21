package com.lingqiao.server.common.security;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.nio.charset.StandardCharsets;
import java.util.HexFormat;
import javax.crypto.Mac;
import javax.crypto.SecretKeyFactory;
import javax.crypto.spec.PBEKeySpec;
import javax.crypto.spec.SecretKeySpec;

/**
 * Derives the per-client HMAC key and signs payloads, mirroring Go
 * auth.GetDerivedKey (PBKDF2, salt "CefBridge-HMAC-Salt-v2") and auth.SignHMAC.
 */
public final class DerivedKeySigner {
    public static final String HMAC_SALT = "CefBridge-HMAC-Salt-v2";
    public static final int PBKDF2_ITERATIONS = 100_000;
    public static final int KEY_LENGTH_BYTES = 32;

    private DerivedKeySigner() {
    }

    public static byte[] deriveKey(String clientSecret) {
        if (clientSecret == null || clientSecret.isEmpty()) {
            throw new SidecarException(ErrorCode.UNAUTHORIZED, "client key unavailable");
        }
        try {
            PBEKeySpec spec = new PBEKeySpec(
                clientSecret.toCharArray(),
                HMAC_SALT.getBytes(StandardCharsets.UTF_8),
                PBKDF2_ITERATIONS,
                KEY_LENGTH_BYTES * 8);
            return SecretKeyFactory.getInstance("PBKDF2WithHmacSHA256").generateSecret(spec).getEncoded();
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to derive client key");
        }
    }

    /**
     * Go signs with SignHMAC(string(derivedKey), payload). In Go, converting
     * []byte -> string -> []byte is lossless (strings are raw byte sequences),
     * so the MAC key is exactly the derived key bytes.
     */
    public static String signWithDerivedKey(byte[] derivedKey, String payload) {
        try {
            Mac mac = Mac.getInstance("HmacSHA256");
            mac.init(new SecretKeySpec(derivedKey, "HmacSHA256"));
            return HexFormat.of().formatHex(mac.doFinal(payload.getBytes(StandardCharsets.UTF_8)));
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to sign payload");
        }
    }
}
