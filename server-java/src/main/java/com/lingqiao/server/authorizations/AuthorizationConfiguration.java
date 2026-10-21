package com.lingqiao.server.authorizations;

import org.springframework.boot.autoconfigure.condition.ConditionalOnExpression;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnExpression(
    "'${lingqiao.sidecar.cards-enabled:false}' == 'true' "
        + "or '${lingqiao.sidecar.cards-write-enabled:false}' == 'true'")
public class AuthorizationConfiguration {
    @Bean
    AuthorizationControlService authorizationControlService(JdbcTemplate jdbcTemplate) {
        return new AuthorizationControlService(jdbcTemplate);
    }
}
