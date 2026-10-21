package com.lingqiao.server.audit;

import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "audit-enabled", havingValue = "true")
public class AuditConfiguration {
    @Bean
    AuditRepository auditRepository(JdbcTemplate jdbcTemplate) {
        return new JdbcAuditRepository(jdbcTemplate);
    }

    @Bean
    AuditService auditService(AuditRepository repository) {
        return new AuditService(repository);
    }
}