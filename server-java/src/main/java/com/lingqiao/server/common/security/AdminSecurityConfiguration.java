package com.lingqiao.server.common.security;

import com.lingqiao.server.common.write.AnySidecarModuleEnabledCondition;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Conditional;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

/**
 * Registers the shared admin authorization service.
 *
 * <p>Declared as an explicit bean rather than a {@code @Service} with
 * {@code @ConditionalOnBean(JdbcTemplate.class)}: component scanning evaluates that
 * condition before the datasource configuration has contributed its beans, so the
 * service silently disappeared whenever only read modules were enabled.
 */
@Configuration
@Conditional(AnySidecarModuleEnabledCondition.class)
public class AdminSecurityConfiguration {
    @Bean
    AdminAuthService adminAuthService(JdbcTemplate sidecarWriteJdbcTemplate) {
        return new AdminAuthService(sidecarWriteJdbcTemplate);
    }
}
