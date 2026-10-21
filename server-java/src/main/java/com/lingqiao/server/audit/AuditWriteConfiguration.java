package com.lingqiao.server.audit;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "audit-write-enabled", havingValue = "true")
public class AuditWriteConfiguration {
    @Bean
    AuditWriteService auditWriteService(JdbcTemplate sidecarWriteJdbcTemplate) {
        return new AuditWriteService(sidecarWriteJdbcTemplate);
    }
}
