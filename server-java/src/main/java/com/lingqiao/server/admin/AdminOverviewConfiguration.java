package com.lingqiao.server.admin;

import java.time.Instant;
import java.util.concurrent.atomic.AtomicLong;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "admin-overview-enabled", havingValue = "true")
public class AdminOverviewConfiguration {
    @Bean
    AdminOverviewService adminOverviewService(JdbcTemplate sidecarWriteJdbcTemplate) {
        return new AdminOverviewService(sidecarWriteJdbcTemplate, Instant.now());
    }

    @Bean
    AtomicLong adminRequestCount() {
        return new AtomicLong();
    }

    @Bean
    AdminRequestCounterFilter adminRequestCounterFilter(AtomicLong adminRequestCount) {
        return new AdminRequestCounterFilter(adminRequestCount);
    }
}
