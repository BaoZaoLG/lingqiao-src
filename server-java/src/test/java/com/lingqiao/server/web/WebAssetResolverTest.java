package com.lingqiao.server.web;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Optional;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class WebAssetResolverTest {
    @TempDir
    Path tempDir;

    private WebAssetResolver resolver;

    @BeforeEach
    void setUp() throws Exception {
        Files.createDirectories(tempDir.resolve("admin"));
        Files.createDirectories(tempDir.resolve("agent"));
        Files.createDirectories(tempDir.resolve("assets"));
        Files.writeString(tempDir.resolve("admin/index.html"), "<html>admin</html>");
        Files.writeString(tempDir.resolve("agent/index.html"), "<html>agent</html>");
        Files.writeString(tempDir.resolve("assets/app.js"), "console.log(1)");
        Files.writeString(tempDir.resolve("assets/app.css"), "body{}");
        Files.writeString(tempDir.resolve("secret.txt"), "should not be reachable");
        resolver = new WebAssetResolver(tempDir);
    }

    @Test
    void availabilityReflectsDirectoryPresence() {
        assertThat(resolver.available()).isTrue();
        assertThat(new WebAssetResolver(tempDir.resolve("missing")).available()).isFalse();
    }

    @Test
    void panelIndexIsServedForRootAndSpaRoutes() {
        Optional<WebAssetResolver.ResolvedAsset> root = resolver.resolve("admin", "", true);
        Optional<WebAssetResolver.ResolvedAsset> deep = resolver.resolve("admin", "/cards/detail", true);

        assertThat(root).isPresent();
        assertThat(root.get().contentType()).isEqualTo("text/html;charset=UTF-8");
        assertThat(deep).isPresent();
        assertThat(deep.get().path()).isEqualTo(root.get().path());
    }

    @Test
    void assetsAreServedWithoutSpaFallback() {
        assertThat(resolver.resolve("assets", "/app.js", false)).isPresent();
        assertThat(resolver.resolve("assets", "/app.js", false).get().contentType())
            .isEqualTo("text/javascript;charset=UTF-8");
        assertThat(resolver.resolve("assets", "/app.css", false).get().contentType())
            .isEqualTo("text/css;charset=UTF-8");
        assertThat(resolver.resolve("assets", "/missing.js", false)).isEmpty();
    }

    @Test
    void traversalAttemptsAreRejected() {
        assertThatThrownBy(() -> resolver.resolve("admin", "../secret.txt", true))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid asset path");
        assertThatThrownBy(() -> resolver.resolve("assets", "/../../secret.txt", false))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid asset path");
        assertThatThrownBy(() -> resolver.resolve("admin", "..\\secret.txt", true))
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid asset path");
    }

    @Test
    void agentPanelIsIsolatedFromAdminPanel() {
        WebAssetResolver.ResolvedAsset admin = resolver.resolve("admin", "", true).orElseThrow();
        WebAssetResolver.ResolvedAsset agent = resolver.resolve("agent", "", true).orElseThrow();

        assertThat(admin.path()).isNotEqualTo(agent.path());
        assertThat(admin.path().getParent().getFileName().toString()).isEqualTo("admin");
        assertThat(agent.path().getParent().getFileName().toString()).isEqualTo("agent");
    }

    @Test
    void missingPanelWithoutIndexReturnsEmpty() throws Exception {
        Files.createDirectories(tempDir.resolve("empty"));

        assertThat(resolver.resolve("empty", "", true)).isEmpty();
    }

    @Test
    void contentTypeMappingCoversCommonAssets() throws Exception {
        Files.writeString(tempDir.resolve("assets/icon.svg"), "<svg/>");
        Files.write(tempDir.resolve("assets/logo.png"), new byte[]{1, 2, 3});
        Files.writeString(tempDir.resolve("assets/data.json"), "{}");

        assertThat(WebAssetResolver.contentTypeOf(tempDir.resolve("assets/icon.svg")))
            .isEqualTo("image/svg+xml");
        assertThat(WebAssetResolver.contentTypeOf(tempDir.resolve("assets/logo.png")))
            .isEqualTo("image/png");
        assertThat(WebAssetResolver.contentTypeOf(tempDir.resolve("assets/data.json")))
            .isEqualTo("application/json;charset=UTF-8");
    }

    @Test
    void relativePathNormalizationStripsLeadingSlashes() {
        assertThat(WebAssetResolver.normalizeRelative("///app.js")).isEqualTo("app.js");
        assertThat(WebAssetResolver.normalizeRelative("/")).isEmpty();
        assertThat(WebAssetResolver.normalizeRelative(null)).isEmpty();
    }
}
