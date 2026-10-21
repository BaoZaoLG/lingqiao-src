package com.lingqiao.server.client;

import java.time.Duration;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "client-write-enabled", havingValue = "true")
public class ClientSessionConfiguration {
    @Bean
    ClientSessionService clientSessionService(
        JdbcTemplate sidecarWriteJdbcTemplate,
        @Value("${lingqiao.client.session-ttl:PT4H}") Duration sessionTtl
    ) {
        return new ClientSessionService(sidecarWriteJdbcTemplate, sessionTtl);
    }
}
