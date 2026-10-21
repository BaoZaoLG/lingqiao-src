package com.lingqiao.server.system;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "blacklist-write-enabled", havingValue = "true")
public class BlacklistWriteConfiguration {
    @Bean
    BlacklistWriteService blacklistWriteService(JdbcTemplate sidecarWriteJdbcTemplate) {
        return new BlacklistWriteService(sidecarWriteJdbcTemplate);
    }
}
