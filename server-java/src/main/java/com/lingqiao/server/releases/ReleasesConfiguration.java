package com.lingqiao.server.releases;

import java.nio.file.Path;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "releases-enabled", havingValue = "true")
public class ReleasesConfiguration {
    @Bean
    @ConditionalOnMissingBean(ReleasesService.class)
    ReleasesService releasesService(
        @Qualifier("releasesJdbcTemplate") JdbcTemplate jdbcTemplate
    ) {
        return new ReleasesService(jdbcTemplate);
    }

    /**
     * The admin release list reports the manifest public key. When the client-update
     * module is also enabled it already contributes this bean, so only define it here
     * if nothing else did.
     */
    @Bean
    @ConditionalOnMissingBean(ManifestSeedLoader.class)
    ManifestSeedLoader releasesManifestSeedLoader(
        @Value("${lingqiao.releases.signing-seed-hex:}") String seedHex,
        @Value("${lingqiao.auth.data-dir:./data}") String dataDir
    ) {
        return new ManifestSeedLoader(seedHex, Path.of(dataDir));
    }
}
