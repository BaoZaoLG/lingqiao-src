package com.lingqiao.server.client;

import com.lingqiao.server.releases.ManifestSeedLoader;
import com.lingqiao.server.releases.PackagePathResolver;
import com.lingqiao.server.releases.ReleaseEventRecorder;
import com.lingqiao.server.releases.ReleaseSelectionStore;
import com.lingqiao.server.releases.ReleaseUpdateService;
import java.nio.file.Path;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "client-update-enabled", havingValue = "true")
public class ClientUpdateConfiguration {
    @Bean
    ReleaseSelectionStore releaseSelectionStore(
        @Qualifier("releasesJdbcTemplate") JdbcTemplate releasesJdbcTemplate
    ) {
        return new ReleaseSelectionStore(releasesJdbcTemplate);
    }

    @Bean
    ReleaseUpdateService releaseUpdateService(ReleaseSelectionStore store) {
        return new ReleaseUpdateService(store);
    }

    @Bean
    ReleaseEventRecorder releaseEventRecorder(
        @Qualifier("releasesJdbcTemplate") JdbcTemplate releasesJdbcTemplate
    ) {
        return new ReleaseEventRecorder(releasesJdbcTemplate);
    }

    @Bean
    ManifestSeedLoader manifestSeedLoader(
        @Value("${lingqiao.releases.signing-seed-hex:}") String seedHex,
        @Value("${lingqiao.auth.data-dir:./data}") String dataDir
    ) {
        return new ManifestSeedLoader(seedHex, Path.of(dataDir));
    }

    @Bean
    PackagePathResolver packagePathResolver(@Value("${lingqiao.auth.data-dir:./data}") String dataDir) {
        return new PackagePathResolver(Path.of(dataDir));
    }
}
