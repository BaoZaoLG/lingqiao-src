package com.lingqiao.server.releases;

import javax.sql.DataSource;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.jdbc.DataSourceBuilder;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

/**
 * Go stores release state in data/releases.db, separately from lingqiao.db.
 * Keeping a distinct template prevents release reads and writes from silently
 * targeting the main business database.
 */
@Configuration
public class ReleaseDataSourceConfiguration {
    @Bean("releasesDataSource")
    DataSource releasesDataSource(
        @Value("${JAVA_RELEASES_DB_URL:${lingqiao.write.db-url:jdbc:sqlite:file:./data/releases.db}}")
        String url
    ) {
        return DataSourceBuilder.create()
            .driverClassName("org.sqlite.JDBC")
            .url(url)
            .build();
    }

    @Bean("releasesJdbcTemplate")
    JdbcTemplate releasesJdbcTemplate(
        @Qualifier("releasesDataSource") DataSource dataSource
    ) {
        return new JdbcTemplate(dataSource);
    }
}
