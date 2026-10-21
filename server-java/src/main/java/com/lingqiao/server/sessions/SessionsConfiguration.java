package com.lingqiao.server.sessions;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "sessions-enabled", havingValue = "true")
public class SessionsConfiguration {
    @Bean
    SessionsService sessionsService(JdbcTemplate jdbcTemplate) {
        return new SessionsService(jdbcTemplate);
    }
}
