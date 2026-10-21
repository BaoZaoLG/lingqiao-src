package com.lingqiao.server.payloads;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.io.IOException;
import java.nio.file.FileAlreadyExistsException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermission;
import java.security.MessageDigest;
import java.security.SecureRandom;
import java.util.HexFormat;
import java.util.Set;

/** Shared legacy pack-script upload key, compatible with Go's UPLOAD_KEY/upload_key lookup. */
public final class PayloadUploadKey {
    private final byte[] value;

    private PayloadUploadKey(String value) {
        this.value = value.getBytes(java.nio.charset.StandardCharsets.UTF_8);
    }

    public static PayloadUploadKey load(Path dataDir, String configured) {
        String explicit = configured == null ? "" : configured;
        if (!explicit.isEmpty()) {
            return new PayloadUploadKey(explicit);
        }
        Path root = dataDir.toAbsolutePath().normalize();
        Path path = root.resolve("upload_key");
        try {
            if (Files.exists(path)) {
                String stored = Files.readString(path).trim();
                if (!stored.isEmpty()) {
                    return new PayloadUploadKey(stored);
                }
            }
            Files.createDirectories(root);
            byte[] random = new byte[24];
            new SecureRandom().nextBytes(random);
            String generated = HexFormat.of().formatHex(random);
            if (Files.exists(path)) {
                Files.writeString(
                    path, generated, java.nio.file.StandardOpenOption.TRUNCATE_EXISTING);
                setOwnerOnly(path);
                return new PayloadUploadKey(generated);
            }
            try {
                Files.writeString(path, generated, java.nio.file.StandardOpenOption.CREATE_NEW);
                setOwnerOnly(path);
                return new PayloadUploadKey(generated);
            } catch (FileAlreadyExistsException race) {
                String stored = Files.readString(path).trim();
                if (!stored.isEmpty()) {
                    return new PayloadUploadKey(stored);
                }
                throw race;
            }
        } catch (IOException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to initialize upload key");
        }
    }

    public boolean matches(String candidate) {
        byte[] supplied = (candidate == null ? "" : candidate)
            .getBytes(java.nio.charset.StandardCharsets.UTF_8);
        return MessageDigest.isEqual(value, supplied);
    }

    private static void setOwnerOnly(Path path) {
        try {
            Files.setPosixFilePermissions(path, Set.of(
                PosixFilePermission.OWNER_READ,
                PosixFilePermission.OWNER_WRITE));
        } catch (UnsupportedOperationException | IOException ignored) {
            // Windows ACLs and non-POSIX filesystems do not expose POSIX mode bits.
        }
    }
}
