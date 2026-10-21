package com.lingqiao.server.releases;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class PackagePathResolverTest {
    @TempDir
    Path tempDir;

    @Test
    void relativePathsResolveInsideDataDir() throws Exception {
        Path packages = Files.createDirectories(tempDir.resolve("packages"));
        Files.writeString(packages.resolve("app.zip"), "payload");

        PackagePathResolver resolver = new PackagePathResolver(tempDir);
        Path resolved = resolver.resolve("packages/app.zip");

        assertThat(resolved).isEqualTo(packages.resolve("app.zip").toAbsolutePath().normalize());
        assertThat(Files.readString(resolved)).isEqualTo("payload");
    }

    @Test
    void absolutePathInsideDataDirIsAccepted() throws Exception {
        Path file = tempDir.resolve("inside.zip");
        Files.writeString(file, "x");

        PackagePathResolver resolver = new PackagePathResolver(tempDir);

        assertThat(resolver.resolve(file.toAbsolutePath().toString()))
            .isEqualTo(file.toAbsolutePath().normalize());
    }

    @Test
    void traversalAndOutsidePathsAreRejected() {
        PackagePathResolver resolver = new PackagePathResolver(tempDir);

        assertThatThrownBy(() -> resolver.resolve("../escape.zip"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("escapes data directory");
        assertThatThrownBy(() -> resolver.resolve("packages/../../escape.zip"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("escapes data directory");
        assertThatThrownBy(() -> resolver.resolve(tempDir.getParent().resolve("outside.zip").toString()))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("escapes data directory");
        assertThatThrownBy(() -> resolver.resolve(""))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("escapes data directory");
    }

    @Test
    void siblingDirectoryWithSharedPrefixIsRejected() throws Exception {
        Path sibling = tempDir.getParent().resolve(tempDir.getFileName() + "-evil");
        Files.createDirectories(sibling);
        PackagePathResolver resolver = new PackagePathResolver(tempDir);

        assertThatThrownBy(() -> resolver.resolve(sibling.resolve("pkg.zip").toString()))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("escapes data directory");
    }

    @Test
    void packageIdValidationMatchesGoRules() {
        PackagePathResolver.validatePackageId("pkg-001");

        assertThatThrownBy(() -> PackagePathResolver.validatePackageId(""))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid package id");
        assertThatThrownBy(() -> PackagePathResolver.validatePackageId("a/b"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid package id");
        assertThatThrownBy(() -> PackagePathResolver.validatePackageId(".."))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid package id");
        assertThatThrownBy(() -> PackagePathResolver.validatePackageId("a\\b"))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid package id");
    }
}
