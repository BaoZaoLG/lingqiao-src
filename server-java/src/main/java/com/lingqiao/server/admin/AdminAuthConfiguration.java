package com.lingqiao.server.admin;

import com.lingqiao.server.agentportal.InviteAdminService;
import java.time.Duration;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.jdbc.core.JdbcTemplate;

@Configuration
@ConditionalOnProperty(prefix = "lingqiao.sidecar", name = "admin-auth-enabled", havingValue = "true")
public class AdminAuthConfiguration {
    @Bean
    AdminLoginService adminLoginService(
        JdbcTemplate sidecarWriteJdbcTemplate,
        @Value("${lingqiao.admin.session-ttl:PT4H}") Duration sessionTtl
    ) {
        return new AdminLoginService(sidecarWriteJdbcTemplate, sessionTtl);
    }

    @Bean
    AdminUserService adminUserService(JdbcTemplate sidecarWriteJdbcTemplate) {
        return new AdminUserService(sidecarWriteJdbcTemplate);
    }

    @Bean
    InviteAdminService inviteAdminService(JdbcTemplate sidecarWriteJdbcTemplate) {
        return new InviteAdminService(sidecarWriteJdbcTemplate);
    }
}
