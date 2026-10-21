package com.lingqiao.server.common.security;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.security.MessageDigest;
import java.time.Instant;
import java.util.HexFormat;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import javax.crypto.Mac;
import javax.crypto.SecretKeyFactory;
import javax.crypto.spec.PBEKeySpec;
import javax.crypto.spec.SecretKeySpec;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

@Service
public class HmacAuthService {
    private static final byte[] SALT = "CefBridge-HMAC-Salt-v2".getBytes(StandardCharsets.UTF_8);
    private static final int ITERATIONS = 100_000;
    private static final int KEY_LEN = 32;
    private static final long SKEW_SECONDS = 30;

    private final String configuredSecret;
    private final Path secretFile;
    private final Map<String, Instant> usedNonces = new ConcurrentHashMap<>();
    private volatile byte[] cachedKey;

    public HmacAuthService(
        @Value("${lingqiao.auth.hmac-secret:}") String configuredSecret,
        @Value("${lingqiao.auth.hmac-secret-file:./data/hmac_secret.key}") String secretFile
    ) {
        this.configuredSecret = configuredSecret == null ? "" : configuredSecret.trim();
        this.secretFile = Path.of(secretFile);
    }

    public void verify(String clientId, String body, String signature, String timestamp, String nonce) {
        if (isBlank(clientId) || isBlank(signature) || isBlank(timestamp) || isBlank(nonce)) {
            throw new SidecarException(ErrorCode.HMAC_MISSING, "missing HMAC fields");
        }
        long ts;
        try {
            ts = Long.parseLong(timestamp);
        } catch (NumberFormatException ex) {
            throw new SidecarException(ErrorCode.HMAC_INVALID, "invalid timestamp");
        }
        long now = Instant.now().getEpochSecond();
        long diff = now - ts;
        if (diff < -SKEW_SECONDS || diff > SKEW_SECONDS) {
            throw new SidecarException(ErrorCode.HMAC_INVALID, "request expired");
        }
        if (!checkAndStoreNonce(nonce)) {
            throw new SidecarException(ErrorCode.HMAC_INVALID, "nonce already used (replay detected)");
        }
        try {
            byte[] key = derivedKey(clientId);
            String signedData = timestamp + "|" + nonce + "|" + nullToEmpty(body);
            Mac mac = Mac.getInstance("HmacSHA256");
            mac.init(new SecretKeySpec(key, "HmacSHA256"));
            String expected = HexFormat.of().formatHex(mac.doFinal(signedData.getBytes(StandardCharsets.UTF_8)));
            if (!MessageDigest.isEqual(expected.getBytes(StandardCharsets.UTF_8), signature.getBytes(StandardCharsets.UTF_8))) {
                throw new SidecarException(ErrorCode.HMAC_INVALID, "HMAC verification failed");
            }
        } catch (SidecarException ex) {
            throw ex;
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.HMAC_INVALID, "HMAC verification failed");
        }
    }

    public String signDerived(String clientId, String payload) {
        try {
            Mac mac = Mac.getInstance("HmacSHA256");
            mac.init(new SecretKeySpec(derivedKey(clientId), "HmacSHA256"));
            return HexFormat.of().formatHex(
                mac.doFinal(nullToEmpty(payload).getBytes(StandardCharsets.UTF_8)));
        } catch (SidecarException ex) {
            throw ex;
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.HMAC_INVALID, "client key unavailable");
        }
    }

    private boolean checkAndStoreNonce(String nonce) {
        Instant now = Instant.now();
        Instant cutoff = now.minusSeconds(120);
        usedNonces.entrySet().removeIf(e -> e.getValue().isBefore(cutoff));
        Instant previous = usedNonces.putIfAbsent(nonce, now);
        return previous == null;
    }

    private byte[] derivedKey(String clientId) throws Exception {
        if (!"injector_v1".equals(clientId)) {
            throw new SidecarException(ErrorCode.HMAC_INVALID, "unknown client");
        }
        byte[] local = cachedKey;
        if (local != null) {
            return local;
        }
        synchronized (this) {
            if (cachedKey != null) {
                return cachedKey;
            }
            String secret = resolveSecret();
            PBEKeySpec spec = new PBEKeySpec(secret.toCharArray(), SALT, ITERATIONS, KEY_LEN * 8);
            SecretKeyFactory factory = SecretKeyFactory.getInstance("PBKDF2WithHmacSHA256");
            cachedKey = factory.generateSecret(spec).getEncoded();
            return cachedKey;
        }
    }

    private String resolveSecret() throws Exception {
        if (!configuredSecret.isBlank()) {
            return configuredSecret;
        }
        if (Files.exists(secretFile)) {
            return Files.readString(secretFile).trim();
        }
        throw new SidecarException(ErrorCode.HMAC_INVALID, "HMAC secret unavailable");
    }

    private static boolean isBlank(String value) {
        return value == null || value.isBlank();
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }
}
