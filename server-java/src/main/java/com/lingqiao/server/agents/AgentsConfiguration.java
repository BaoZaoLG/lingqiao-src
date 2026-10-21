package com.lingqiao.server.agents;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "agents-enabled", havingValue = "true")
public class AgentsConfiguration {
    @Bean
    AgentsService agentsService(JdbcTemplate jdbcTemplate) {
        return new AgentsService(jdbcTemplate);
    }

    @Bean
    AgentResourcesService agentResourcesService(JdbcTemplate jdbcTemplate) {
        return new AgentResourcesService(jdbcTemplate);
    }
}
