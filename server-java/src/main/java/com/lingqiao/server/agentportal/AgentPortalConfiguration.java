package com.lingqiao.server.agentportal;

import java.time.Duration;
import java.nio.file.Path;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "agent-portal-enabled", havingValue = "true")
public class AgentPortalConfiguration {
    @Bean
    AgentPortalAuthService agentPortalAuthService(
        JdbcTemplate sidecarWriteJdbcTemplate,
        @Value("${lingqiao.agent.session-ttl:PT4H}") Duration sessionTtl,
        AgentSessionStore agentSessionStore
    ) {
        return new AgentPortalAuthService(sidecarWriteJdbcTemplate, sessionTtl, agentSessionStore);
    }

    @Bean
    AgentSessionStore agentSessionStore(
        @Value("${lingqiao.auth.data-dir:./data}") String dataDir,
        @Value("${lingqiao.sidecar.agent-portal-write-enabled:false}") boolean writeEnabled
    ) {
        return writeEnabled
            ? new AgentSessionStore(Path.of(dataDir))
            : AgentSessionStore.readOnly(Path.of(dataDir));
    }

    @Bean
    AgentRegistrationService agentRegistrationService(JdbcTemplate sidecarWriteJdbcTemplate) {
        return new AgentRegistrationService(sidecarWriteJdbcTemplate);
    }

    @Bean
    AgentPortalCardService agentPortalCardService(JdbcTemplate sidecarWriteJdbcTemplate) {
        return new AgentPortalCardService(sidecarWriteJdbcTemplate);
    }
}
