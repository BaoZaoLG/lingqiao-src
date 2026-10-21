package com.lingqiao.server.releases;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.util.HexFormat;
import org.bouncycastle.crypto.params.Ed25519PrivateKeyParameters;
import org.bouncycastle.crypto.params.Ed25519PublicKeyParameters;
import org.bouncycastle.crypto.signers.Ed25519Signer;

/**
 * Ed25519 manifest signer mirroring Go releases.ManifestSigner.
 * Canonical bytes must match Go's json.Marshal output exactly.
 */
public class ManifestSigner {
    public static final int SEED_SIZE = 32;

    private static final ObjectMapper MAPPER = new ObjectMapper()
        .registerModule(new JavaTimeModule())
        .disable(SerializationFeature.WRITE_DATES_AS_TIMESTAMPS)
        .setSerializationInclusion(JsonInclude.Include.ALWAYS);

    private final Ed25519PrivateKeyParameters privateKey;
    private final Ed25519PublicKeyParameters publicKey;

    public ManifestSigner(byte[] seed) {
        if (seed == null || seed.length != SEED_SIZE) {
            throw new SidecarException(ErrorCode.BAD_REQUEST,
                "manifest signer seed must be " + SEED_SIZE + " bytes");
        }
        this.privateKey = new Ed25519PrivateKeyParameters(seed, 0);
        this.publicKey = privateKey.generatePublicKey();
    }

    public String publicKeyHex() {
        return HexFormat.of().formatHex(publicKey.getEncoded());
    }

    public SignedUpdateManifest sign(UpdateManifest manifest) {
        byte[] canonical = canonical(manifest);
        Ed25519Signer signer = new Ed25519Signer();
        signer.init(true, privateKey);
        signer.update(canonical, 0, canonical.length);
        return new SignedUpdateManifest(manifest, HexFormat.of().formatHex(signer.generateSignature()));
    }

    public static boolean verify(String publicKeyHex, SignedUpdateManifest signed) {
        if (publicKeyHex == null || signed == null || signed.signature() == null) {
            return false;
        }
        byte[] key;
        byte[] signature;
        try {
            key = HexFormat.of().parseHex(publicKeyHex);
            signature = HexFormat.of().parseHex(signed.signature());
        } catch (IllegalArgumentException ex) {
            return false;
        }
        if (key.length != Ed25519PublicKeyParameters.KEY_SIZE || signature.length != 64) {
            return false;
        }
        byte[] canonical = canonical(signed.manifest());
        Ed25519Signer verifier = new Ed25519Signer();
        verifier.init(false, new Ed25519PublicKeyParameters(key, 0));
        verifier.update(canonical, 0, canonical.length);
        return verifier.verifySignature(signature);
    }

    public static byte[] canonical(UpdateManifest manifest) {
        try {
            return MAPPER.writeValueAsBytes(manifest);
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to canonicalize manifest");
        }
    }

    public static String sha256Hex(byte[] data) {
        try {
            return HexFormat.of().formatHex(MessageDigest.getInstance("SHA-256").digest(data));
        } catch (Exception ex) {
            throw new IllegalStateException("sha256 unavailable", ex);
        }
    }

    public static byte[] utf8(String value) {
        return value == null ? new byte[0] : value.getBytes(StandardCharsets.UTF_8);
    }
}
