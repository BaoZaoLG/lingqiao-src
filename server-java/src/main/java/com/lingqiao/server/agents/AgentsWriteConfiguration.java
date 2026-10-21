package com.lingqiao.server.agents;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "agents-write-enabled", havingValue = "true")
public class AgentsWriteConfiguration {
    @Bean
    AgentsWriteService agentsWriteService(JdbcTemplate sidecarWriteJdbcTemplate) {
        return new AgentsWriteService(sidecarWriteJdbcTemplate);
    }
}
