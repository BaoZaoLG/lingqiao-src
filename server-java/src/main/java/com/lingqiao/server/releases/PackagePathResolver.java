package com.lingqiao.server.releases;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.SidecarException;
import java.nio.file.Path;
import java.util.Locale;

/**
 * Resolves a release package path inside the data directory, mirroring Go
 * releaseService.resolvePackagePath including the case-insensitive containment
 * check used on Windows.
 */
public class PackagePathResolver {
    private final Path dataDir;

    public PackagePathResolver(Path dataDir) {
        this.dataDir = dataDir.toAbsolutePath().normalize();
    }

    public Path resolve(String packagePath) {
        if (packagePath == null || packagePath.isBlank()) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "package path escapes data directory");
        }
        Path candidate = Path.of(packagePath);
        Path resolved = (candidate.isAbsolute() ? candidate : dataDir.resolve(candidate))
            .toAbsolutePath()
            .normalize();

        String base = dataDir.toString().toLowerCase(Locale.ROOT);
        String target = resolved.toString().toLowerCase(Locale.ROOT);
        boolean contained = target.equals(base)
            || target.startsWith(base + java.io.File.separator);
        if (!contained) {
            throw new SidecarException(ErrorCode.FORBIDDEN, "package path escapes data directory");
        }
        return resolved;
    }

    public static void validatePackageId(String id) {
        if (id == null || id.isEmpty() || id.contains("/") || id.contains("\\") || id.contains("..")) {
            throw new SidecarException(ErrorCode.BAD_REQUEST, "invalid package id");
        }
    }
}
