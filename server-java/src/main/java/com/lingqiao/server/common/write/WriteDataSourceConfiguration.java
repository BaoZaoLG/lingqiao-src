package com.lingqiao.server.common.write;

import javax.sql.DataSource;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.jdbc.DataSourceBuilder;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Conditional;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Primary;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.jdbc.datasource.DataSourceTransactionManager;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.annotation.EnableTransactionManagement;

/**
 * Shared SQLite datasource for every Java sidecar module.
 *
 * <p>Activated when any {@code lingqiao.sidecar.*} flag is on, because read modules
 * need a {@link JdbcTemplate} just as much as write modules do. Read-only use is kept
 * read-only by the services themselves, not by withholding the datasource.
 */
@Configuration
@EnableTransactionManagement
@Conditional(AnySidecarModuleEnabledCondition.class)
public class WriteDataSourceConfiguration {
    @Bean
    @Primary
    DataSource sidecarWriteDataSource(@Value("${lingqiao.write.db-url:jdbc:sqlite:file:./data/lingqiao.db}") String url) {
        return DataSourceBuilder.create()
            .driverClassName("org.sqlite.JDBC")
            .url(url)
            .build();
    }

    @Bean
    @Primary
    JdbcTemplate sidecarWriteJdbcTemplate(DataSource sidecarWriteDataSource) {
        return new JdbcTemplate(sidecarWriteDataSource);
    }

    @Bean
    @Primary
    PlatformTransactionManager sidecarWriteTransactionManager(DataSource sidecarWriteDataSource) {
        return new DataSourceTransactionManager(sidecarWriteDataSource);
    }
}
