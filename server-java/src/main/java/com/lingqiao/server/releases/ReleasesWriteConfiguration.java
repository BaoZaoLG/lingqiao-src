package com.lingqiao.server.releases;

import java.nio.file.Path;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "releases-write-enabled", havingValue = "true")
public class ReleasesWriteConfiguration {
    @Bean
    ReleasesWriteService releasesWriteService(
        @Qualifier("releasesJdbcTemplate") JdbcTemplate releasesJdbcTemplate
    ) {
        return new ReleasesWriteService(releasesJdbcTemplate);
    }

    @Bean
    @ConditionalOnMissingBean(ReleasesService.class)
    ReleasesService releasesWriteReadService(
        @Qualifier("releasesJdbcTemplate") JdbcTemplate releasesJdbcTemplate
    ) {
        return new ReleasesService(releasesJdbcTemplate);
    }

    @Bean
    ReleaseAdminService releaseAdminService(
        ReleasesService reads,
        ReleasesWriteService writes,
        @Qualifier("releasesJdbcTemplate") JdbcTemplate releasesJdbcTemplate,
        @Qualifier("sidecarWriteJdbcTemplate") JdbcTemplate sidecarWriteJdbcTemplate,
        @Value("${lingqiao.auth.data-dir:./data}") String dataDir
    ) {
        return new ReleaseAdminService(
            reads, writes, releasesJdbcTemplate, sidecarWriteJdbcTemplate, Path.of(dataDir));
    }
}
