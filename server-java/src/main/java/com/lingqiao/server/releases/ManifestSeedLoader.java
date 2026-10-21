package com.lingqiao.server.releases;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.FileAlreadyExistsException;
import java.nio.file.StandardOpenOption;
import java.nio.file.attribute.PosixFilePermission;
import java.security.SecureRandom;
import java.util.Set;
import java.util.HexFormat;

/**
 * Loads the Ed25519 manifest signing seed, mirroring Go
 * releases.loadOrCreateManifestSeed, including persistent first-run creation.
 */
public class ManifestSeedLoader {
    private final String configuredSeedHex;
    private final Path dataDir;

    public ManifestSeedLoader(String configuredSeedHex, Path dataDir) {
        this.configuredSeedHex = configuredSeedHex == null ? "" : configuredSeedHex.trim();
        this.dataDir = dataDir.toAbsolutePath().normalize();
    }

    public byte[] load() {
        if (!configuredSeedHex.isEmpty()) {
            return decode(configuredSeedHex, "UPDATE_SIGNING_SEED_HEX");
        }
        Path seedPath = dataDir.resolve("update_manifest_seed.key").toAbsolutePath().normalize();
        if (!seedPath.startsWith(dataDir)) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "invalid manifest seed path");
        }
        try {
            if (!Files.exists(seedPath)) {
                createSeed(seedPath);
            }
            String content = Files.readString(seedPath, StandardCharsets.UTF_8).trim();
            return decode(content, "update_manifest_seed.key");
        } catch (SidecarException ex) {
            throw ex;
        } catch (Exception ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "failed to read manifest seed");
        }
    }

    private static void createSeed(Path seedPath) throws Exception {
        Files.createDirectories(seedPath.getParent());
        byte[] seed = new byte[ManifestSigner.SEED_SIZE];
        new SecureRandom().nextBytes(seed);
        try {
            Files.writeString(
                seedPath,
                HexFormat.of().formatHex(seed),
                StandardCharsets.UTF_8,
                StandardOpenOption.CREATE_NEW,
                StandardOpenOption.WRITE);
            try {
                Files.setPosixFilePermissions(seedPath, Set.of(
                    PosixFilePermission.OWNER_READ,
                    PosixFilePermission.OWNER_WRITE));
            } catch (UnsupportedOperationException ignored) {
                // Windows ACLs do not expose POSIX permissions.
            }
        } catch (FileAlreadyExistsException ignored) {
            // Another process won the first-run race; read its seed below.
        }
    }

    private static byte[] decode(String hex, String source) {
        byte[] seed;
        try {
            seed = HexFormat.of().parseHex(hex);
        } catch (IllegalArgumentException ex) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR, "invalid " + source);
        }
        if (seed.length != ManifestSigner.SEED_SIZE) {
            throw new SidecarException(ErrorCode.INTERNAL_ERROR,
                source + " must decode to " + ManifestSigner.SEED_SIZE + " bytes");
        }
        return seed;
    }
}
