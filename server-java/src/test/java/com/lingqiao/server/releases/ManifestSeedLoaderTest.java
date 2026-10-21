package com.lingqiao.server.releases;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.lingqiao.server.common.errors.SidecarException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HexFormat;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

class ManifestSeedLoaderTest {
    @TempDir
    Path tempDir;

    private static final String SEED_HEX =
        "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20";

    @Test
    void configuredSeedTakesPrecedence() {
        ManifestSeedLoader loader = new ManifestSeedLoader(SEED_HEX, tempDir);

        assertThat(HexFormat.of().formatHex(loader.load())).isEqualTo(SEED_HEX);
    }

    @Test
    void seedFileIsUsedWhenNoEnvSeed() throws Exception {
        Files.writeString(tempDir.resolve("update_manifest_seed.key"), SEED_HEX + "\n");
        ManifestSeedLoader loader = new ManifestSeedLoader("", tempDir);

        byte[] seed = loader.load();

        assertThat(seed).hasSize(ManifestSigner.SEED_SIZE);
        assertThat(new ManifestSigner(seed).publicKeyHex())
            .isEqualTo("79b5562e8fe654f94078b112e8a98ba7901f853ae695bed7e0e3910bad049664");
    }

    @Test
    void missingSeedIsGeneratedAndPersistedLikeGo() {
        ManifestSeedLoader loader = new ManifestSeedLoader("", tempDir);

        byte[] first = loader.load();
        byte[] second = new ManifestSeedLoader("", tempDir).load();

        assertThat(first).hasSize(ManifestSigner.SEED_SIZE);
        assertThat(second).containsExactly(first);
        assertThat(Files.exists(tempDir.resolve("update_manifest_seed.key"))).isTrue();
    }

    @Test
    void malformedSeedsAreRejected() throws Exception {
        assertThatThrownBy(() -> new ManifestSeedLoader("zz", tempDir).load())
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("invalid UPDATE_SIGNING_SEED_HEX");
        assertThatThrownBy(() -> new ManifestSeedLoader("0102", tempDir).load())
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("must decode to 32 bytes");

        Files.writeString(tempDir.resolve("update_manifest_seed.key"), "0102");
        assertThatThrownBy(() -> new ManifestSeedLoader("", tempDir).load())
            .isInstanceOf(SidecarException.class)
            .hasMessageContaining("must decode to 32 bytes");
    }
}
