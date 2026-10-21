package com.lingqiao.server.sessions;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "sessions-write-enabled", havingValue = "true")
public class SessionsWriteConfiguration {
    @Bean
    SessionsWriteService sessionsWriteService(JdbcTemplate sidecarWriteJdbcTemplate) {
        return new SessionsWriteService(sidecarWriteJdbcTemplate);
    }
}
